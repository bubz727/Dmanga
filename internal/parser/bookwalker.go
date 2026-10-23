package parser

import (
	"bytes"
	"compress/zlib"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

var (
	reBookWalkerUUID   = regexp.MustCompile(`(?i)[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`)
	reBookWalkerCID    = regexp.MustCompile(`(?i)(?:[?&]cid=|data-cid=["']|/c\?cid=)([0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12})`)
	reBookWalkerReadID = regexp.MustCompile(`/read/([0-9A-Za-z]+)(?:/([^/?#]+))?`)
	reBookWalkerVol    = regexp.MustCompile(`(?i)(?:^|[-_\s])vol(?:ume)?[-_\s.]*(\d+(?:\.\d+)?)$`)
	reVariantHeight    = regexp.MustCompile(`^h(\d+)/`)
)

type BookWalker struct {
	client *http.Client
}

func NewBookWalker() *BookWalker {
	return &BookWalker{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (b *BookWalker) Name() string {
	return "BookWalker"
}

func (b *BookWalker) Domains() []string {
	return []string{
		"bookwalker.jp",
		"www.bookwalker.jp",
		"global.bookwalker.jp",
		"bookwalker.com",
		"www.bookwalker.com",
		"viewer-trial.bookwalker.jp",
		"viewer.bookwalker.jp",
		"viewer-df.bookwalker.jp",
	}
}

func (b *BookWalker) GetImages(chapterURL string) ([]string, error) {
	chap, err := b.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (b *BookWalker) GetChapter(chapterURL string) (*core.Chapter, error) {
	// 1. Check if this is a BookWalker Global /read/<readId> URL
	if m := reBookWalkerReadID.FindStringSubmatch(chapterURL); len(m) > 1 {
		readID := m[1]
		slug := ""
		if len(m) > 2 {
			slug = m[2]
		}
		return b.getChapterFromReadService(chapterURL, readID, slug)
	}

	// 2. Otherwise use the trial viewer (viewer-trial.bookwalker.jp) CID flow
	cid, pageTitle, err := b.resolveCID(chapterURL)
	if err != nil {
		return nil, err
	}

	auth, err := b.fetchTrialAuth(cid)
	if err != nil {
		return nil, fmt.Errorf("BookWalker auth error: %w", err)
	}

	images, err := b.fetchPages(auth)
	if err != nil {
		return nil, err
	}

	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	title := auth.CTI
	if strings.TrimSpace(title) == "" {
		title = pageTitle
	}
	if strings.TrimSpace(title) == "" {
		title = fmt.Sprintf("BookWalker_%s", cid)
	}

	return &core.Chapter{
		Title:  utils.SanitizeFolderName(title),
		Images: images,
	}, nil
}

func (b *BookWalker) getChapterFromReadService(chapterURL, readID, slug string) (*core.Chapter, error) {
	prdID := readID
	if !strings.HasPrefix(prdID, "PRD_") {
		prdID = "PRD_" + prdID
	}

	// Encode ViewerRequestBody { string id = 1; }
	idBytes := []byte(prdID)
	reqBody := make([]byte, 0, 2+len(idBytes))
	reqBody = append(reqBody, 0x0a)
	reqBody = appendVarint(reqBody, uint64(len(idBytes)))
	reqBody = append(reqBody, idBytes...)

	openURL := "https://bookwalker.com/api/kyon/kyon.v1.ReadService/Open"
	req, err := http.NewRequest("POST", openURL, bytes.NewReader(reqBody))
	if err != nil {
		return nil, err
	}
	fetcher.ApplyDefaultBrowserHeaders(req, "bookwalker.com")
	req.Header.Set("Content-Type", "application/proto")
	req.Header.Set("Accept", "application/proto, */*")
	req.Header.Set("Origin", "https://bookwalker.com")
	req.Header.Set("Referer", chapterURL)

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call BookWalker ReadService/Open: %w", err)
	}
	bodyBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BookWalker ReadService/Open returned HTTP %d (chapter may require login/purchase)", resp.StatusCode)
	}

	manifestURLStr, mimeType := parseViewerResponse(bodyBytes)
	if manifestURLStr == "" {
		return nil, fmt.Errorf("failed to extract manifest URL from BookWalker ReadService/Open response")
	}

	images, err := b.fetchE4PManifestPages(manifestURLStr, mimeType)
	if err != nil {
		return nil, err
	}
	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	title := formatBookWalkerSlugTitle(slug, readID)
	return &core.Chapter{
		Title:  title,
		Images: images,
	}, nil
}

func formatBookWalkerSlugTitle(slug, readID string) string {
	slug = strings.TrimSpace(slug)
	if slug == "" {
		return "BookWalker_" + readID
	}
	spaced := strings.ReplaceAll(slug, "-", " ")
	if num := utils.ExtractChapterNumber(spaced); num != "" {
		return "Ch " + num
	}
	if m := reBookWalkerVol.FindStringSubmatch(slug); len(m) > 1 {
		return "Vol " + m[1]
	}
	return utils.SanitizeFolderName(spaced)
}

func appendVarint(buf []byte, v uint64) []byte {
	for v >= 0x80 {
		buf = append(buf, byte(v)|0x80)
		v >>= 7
	}
	return append(buf, byte(v))
}

func readVarint(buf []byte) (uint64, int) {
	return binary.Uvarint(buf)
}

func skipWireValue(data []byte, offset int, wireType int) int {
	switch wireType {
	case 0:
		_, n := binary.Uvarint(data[offset:])
		if n <= 0 {
			return -1
		}
		return offset + n
	case 1:
		if offset+8 > len(data) {
			return -1
		}
		return offset + 8
	case 2:
		l, n := binary.Uvarint(data[offset:])
		if n <= 0 || offset+n+int(l) > len(data) {
			return -1
		}
		return offset + n + int(l)
	case 5:
		if offset+4 > len(data) {
			return -1
		}
		return offset + 4
	default:
		return -1
	}
}

func parseViewerResponse(data []byte) (manifestURL string, mimeType string) {
	offset := 0
	for offset < len(data) {
		tag, n := readVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7

		if wireType == 2 {
			length, n := readVarint(data[offset:])
			if n <= 0 || offset+n+int(length) > len(data) {
				break
			}
			offset += n
			sub := data[offset : offset+int(length)]
			offset += int(length)

			if fieldNum == 2 { // ViewerDetails
				mURL, mType := parseViewerDetails(sub)
				if mURL != "" {
					manifestURL = mURL
				}
				if mType != "" {
					mimeType = mType
				}
			}
		} else if wireType == 0 {
			_, n := readVarint(data[offset:])
			if n <= 0 {
				break
			}
			offset += n
		} else {
			offset = skipWireValue(data, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}
	return manifestURL, mimeType
}

func parseViewerDetails(data []byte) (manifestURL string, mimeType string) {
	offset := 0
	for offset < len(data) {
		tag, n := readVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7

		if wireType == 2 {
			length, n := readVarint(data[offset:])
			if n <= 0 || offset+n+int(length) > len(data) {
				break
			}
			offset += n
			strVal := string(data[offset : offset+int(length)])
			offset += int(length)

			switch fieldNum {
			case 1:
				manifestURL = strVal
			case 6:
				mimeType = strVal
			}
		} else if wireType == 0 {
			_, n := readVarint(data[offset:])
			if n <= 0 {
				break
			}
			offset += n
		} else {
			offset = skipWireValue(data, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}
	return manifestURL, mimeType
}

func (b *BookWalker) fetchE4PManifestPages(manifestURLStr, mimeType string) ([]string, error) {
	manifestURL, err := url.Parse(manifestURLStr)
	if err != nil {
		return nil, err
	}

	req, err := http.NewRequest("GET", manifestURLStr, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Origin", "https://bookwalker.com")
	req.Header.Set("Referer", "https://bookwalker.com/")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch BookWalker manifest: %w", err)
	}
	manifestBytes, err := io.ReadAll(resp.Body)
	resp.Body.Close()
	if err != nil {
		return nil, err
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("BookWalker manifest returned HTTP %d", resp.StatusCode)
	}

	pubBytes := manifestBytes
	if strings.Contains(mimeType, "vnd.e4p.qst") {
		unwrapped, err := unwrapE4PQSTicket(manifestBytes)
		if err != nil {
			return nil, err
		}
		pubBytes = unwrapped
	}

	links := parseProtoPubLinks(pubBytes)
	if len(links) == 0 {
		return nil, core.ErrNoImagesFound
	}

	manifestQuery := manifestURL.Query()
	var images []string

	for _, variants := range links {
		bestLink := pickBestVariantLink(variants)
		if bestLink == "" {
			continue
		}
		relURL, err := url.Parse(bestLink)
		if err != nil {
			continue
		}
		resolved := manifestURL.ResolveReference(relURL)
		q := resolved.Query()
		for k, vals := range manifestQuery {
			if len(vals) > 0 {
				q.Set(k, vals[0])
			}
		}
		resolved.RawQuery = q.Encode()
		images = append(images, resolved.String())
	}

	return images, nil
}

type e4pVariant struct {
	link     string
	hasImage bool
}

func parseProtoPubLinks(data []byte) [][]e4pVariant {
	var spine [][]e4pVariant
	offset := 0
	for offset < len(data) {
		tag, n := readVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7

		if wireType == 2 {
			length, n := readVarint(data[offset:])
			if n <= 0 || offset+n+int(length) > len(data) {
				break
			}
			offset += n
			sub := data[offset : offset+int(length)]
			offset += int(length)

			if fieldNum == 2 { // spine Link
				variants := parseProtoPubLink(sub)
				if len(variants) > 0 {
					spine = append(spine, variants)
				}
			}
		} else if wireType == 0 {
			_, n := readVarint(data[offset:])
			if n <= 0 {
				break
			}
			offset += n
		} else {
			offset = skipWireValue(data, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}
	return spine
}

func parseProtoPubLink(data []byte) []e4pVariant {
	var variants []e4pVariant
	offset := 0
	for offset < len(data) {
		tag, n := readVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7

		if wireType == 2 {
			length, n := readVarint(data[offset:])
			if n <= 0 || offset+n+int(length) > len(data) {
				break
			}
			offset += n
			sub := data[offset : offset+int(length)]
			offset += int(length)

			if fieldNum == 1 { // Variant
				v := parseProtoPubVariant(sub)
				if v.link != "" {
					variants = append(variants, v)
				}
			}
		} else if wireType == 0 {
			_, n := readVarint(data[offset:])
			if n <= 0 {
				break
			}
			offset += n
		} else {
			offset = skipWireValue(data, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}
	return variants
}

func parseProtoPubVariant(data []byte) e4pVariant {
	var v e4pVariant
	offset := 0
	for offset < len(data) {
		tag, n := readVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7

		if wireType == 2 {
			length, n := readVarint(data[offset:])
			if n <= 0 || offset+n+int(length) > len(data) {
				break
			}
			offset += n
			sub := data[offset : offset+int(length)]
			offset += int(length)

			switch fieldNum {
			case 1:
				v.link = string(sub)
			case 2:
				v.hasImage = true
			}
		} else if wireType == 0 {
			_, n := readVarint(data[offset:])
			if n <= 0 {
				break
			}
			offset += n
		} else {
			offset = skipWireValue(data, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}
	return v
}

func pickBestVariantLink(variants []e4pVariant) string {
	bestLink := ""
	bestHeight := -1
	for _, v := range variants {
		if !v.hasImage {
			continue
		}
		h := 0
		if m := reVariantHeight.FindStringSubmatch(v.link); len(m) > 1 {
			h, _ = strconv.Atoi(m[1])
		}
		if h > bestHeight || bestLink == "" {
			bestHeight = h
			bestLink = v.link
		}
	}
	if bestLink == "" && len(variants) > 0 {
		bestLink = variants[len(variants)-1].link
	}
	return bestLink
}

func unwrapE4PQSTicket(data []byte) ([]byte, error) {
	// Parse E4PQSTicket field 1 (type) and field 5 (child E4PQSWrapper)
	var ticketType uint64
	var childBytes []byte
	offset := 0
	for offset < len(data) {
		tag, n := readVarint(data[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7
		if wireType == 0 {
			val, n := readVarint(data[offset:])
			if n <= 0 {
				break
			}
			offset += n
			if fieldNum == 1 {
				ticketType = val
			}
		} else if wireType == 2 {
			length, n := readVarint(data[offset:])
			if n <= 0 || offset+n+int(length) > len(data) {
				break
			}
			offset += n
			sub := data[offset : offset+int(length)]
			offset += int(length)
			if fieldNum == 5 {
				childBytes = sub
			}
		} else {
			offset = skipWireValue(data, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}

	if ticketType != 0 {
		return nil, fmt.Errorf("BookWalker chapter uses DRM-encrypted ticket (type %d)", ticketType)
	}

	// Parse E4PQSWrapper field 1 (type), field 4 (data), field 5 (dataType)
	var wrapperType, dataType uint64
	var payload []byte
	offset = 0
	for offset < len(childBytes) {
		tag, n := readVarint(childBytes[offset:])
		if n <= 0 {
			break
		}
		offset += n
		fieldNum := tag >> 3
		wireType := tag & 7
		if wireType == 0 {
			val, n := readVarint(childBytes[offset:])
			if n <= 0 {
				break
			}
			offset += n
			switch fieldNum {
			case 1:
				wrapperType = val
			case 5:
				dataType = val
			}
		} else if wireType == 2 {
			length, n := readVarint(childBytes[offset:])
			if n <= 0 || offset+n+int(length) > len(childBytes) {
				break
			}
			offset += n
			sub := childBytes[offset : offset+int(length)]
			offset += int(length)
			if fieldNum == 4 {
				payload = sub
			}
		} else {
			offset = skipWireValue(childBytes, offset, int(wireType))
			if offset < 0 {
				break
			}
		}
	}

	if wrapperType != 0 {
		return nil, fmt.Errorf("BookWalker chapter uses DRM-encrypted wrapper (type %d)", wrapperType)
	}

	if len(payload) >= 52 && string(payload[:4]) == "PBEX" {
		payload = payload[52:]
	}

	if dataType == 5 { // PROTOPUB_ZLIB
		r, err := zlib.NewReader(bytes.NewReader(payload))
		if err != nil {
			return nil, err
		}
		defer r.Close()
		return io.ReadAll(r)
	}
	return payload, nil
}

// ProcessImage handles QSC archive extraction and XEBP-to-WEBP header stripping if needed
func (b *BookWalker) ProcessImage(data []byte, fileURL string) ([]byte, error) {
	// Check if data is a QSC archive ("E4PQSC\x01\x00")
	if len(data) >= 4096 && string(data[:6]) == "E4PQSC" && data[6] == 1 && data[7] == 0 {
		qscKey := ""
		if idx := strings.Index(fileURL, "#"); idx != -1 {
			frag := fileURL[idx+1:]
			parts := strings.Split(frag, "\n")
			qscKey = parts[0]
		}
		if qscKey != "" {
			runningOffset := 0
			for i := 0; i < 127; i++ {
				base := 32 + i*32
				size := int(binary.LittleEndian.Uint32(data[base+4 : base+8]))
				if size == 0 {
					break
				}
				entryName := strings.TrimRight(string(data[base+8:base+32]), "\x00")
				if entryName == qscKey {
					start := 4096 + runningOffset
					end := start + size
					if end <= len(data) {
						data = data[start:end]
					}
					break
				}
				runningOffset += size
			}
		}
	}

	// If data is RIFF....XEBP, strip to RIFF....WEBP
	if len(data) >= 20 && string(data[:4]) == "RIFF" && string(data[8:12]) == "XEBP" {
		vp8Size := int(binary.LittleEndian.Uint32(data[16:20]))
		webpEnd := 20 + vp8Size + (vp8Size & 1)
		if webpEnd <= len(data) {
			out := make([]byte, 0, webpEnd)
			out = append(out, "RIFF"...)
			var sizeBuf [4]byte
			binary.LittleEndian.PutUint32(sizeBuf[:], uint32(webpEnd-8))
			out = append(out, sizeBuf[:]...)
			out = append(out, "WEBP"...)
			out = append(out, data[12:webpEnd]...)
			return out, nil
		}
	}

	return data, nil
}

func (b *BookWalker) resolveCID(rawURL string) (string, string, error) {
	parsed, err := url.Parse(rawURL)
	if err == nil {
		// 1. Direct query parameter: ?cid=...
		if cid := parsed.Query().Get("cid"); cid != "" {
			return cid, "", nil
		}
	}

	// 2. Direct UUID in the URL string
	if match := reBookWalkerUUID.FindString(rawURL); match != "" {
		return match, "", nil
	}

	// 3. Webpage - scrape HTML to find cid
	doc, rawHTML, err := fetcher.FetchHTML(rawURL, map[string]string{
		"Referer": "https://bookwalker.jp/",
	})
	if err != nil {
		return "", "", fmt.Errorf("failed to load BookWalker webpage: %w", err)
	}

	pageTitle := ""
	if doc != nil {
		pageTitle = strings.TrimSpace(doc.Find("title").Text())
		if idx := strings.Index(pageTitle, "｜"); idx != -1 {
			pageTitle = strings.TrimSpace(pageTitle[:idx])
		} else if idx := strings.Index(pageTitle, "|"); idx != -1 {
			pageTitle = strings.TrimSpace(pageTitle[:idx])
		} else if idx := strings.Index(pageTitle, " - "); idx != -1 {
			pageTitle = strings.TrimSpace(pageTitle[:idx])
		}

		// Look for viewer links in <a> or buttons
		doc.Find("a[href*='viewer'], a[data-cid], button[data-cid]").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			if cid, exists := s.Attr("data-cid"); exists && cid != "" {
				rawHTML += "\n?cid=" + cid
				return false
			}
			if href, exists := s.Attr("href"); exists {
				if match := reBookWalkerUUID.FindString(href); match != "" {
					rawHTML += "\n?cid=" + match
					return false
				}
			}
			return true
		})
	}

	if match := reBookWalkerCID.FindStringSubmatch(rawHTML); len(match) > 1 {
		return match[1], pageTitle, nil
	}

	return "", "", fmt.Errorf("failed to extract BookWalker CID from URL: %s", rawURL)
}

type bwTrialAuth struct {
	Status   string `json:"status"`
	URL      string `json:"url"`
	CTI      string `json:"cti"`
	CTY      int    `json:"cty"`
	AuthInfo *struct {
		PfCd      string `json:"pfCd"`
		Policy    string `json:"Policy"`
		Signature string `json:"Signature"`
		KeyPairID string `json:"Key-Pair-Id"`
	} `json:"auth_info"`
}

func (b *BookWalker) fetchTrialAuth(cid string) (*bwTrialAuth, error) {
	endpoints := []string{
		fmt.Sprintf("https://viewer-trial.bookwalker.jp/trial-page/c?cid=%s&BID=0", cid),
		fmt.Sprintf("https://viewer.bookwalker.jp/trial-page/c?cid=%s&BID=0", cid),
		fmt.Sprintf("https://viewer-df.bookwalker.jp/trial-page/c?cid=%s&BID=0", cid),
	}

	var lastErr error
	for _, ep := range endpoints {
		req, err := http.NewRequest("GET", ep, nil)
		if err != nil {
			lastErr = err
			continue
		}
		fetcher.ApplyDefaultBrowserHeaders(req, "viewer-trial.bookwalker.jp")
		req.Header.Set("Referer", "https://viewer-trial.bookwalker.jp/")

		resp, err := b.client.Do(req)
		if err != nil {
			lastErr = err
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}

		var auth bwTrialAuth
		if err := json.Unmarshal(bodyBytes, &auth); err != nil {
			lastErr = err
			continue
		}

		if auth.Status == "200" && auth.URL != "" && auth.AuthInfo != nil && auth.AuthInfo.PfCd != "" {
			return &auth, nil
		}
		lastErr = fmt.Errorf("status code %s", auth.Status)
	}

	return nil, fmt.Errorf("failed to authenticate BookWalker CID '%s': %v", cid, lastErr)
}

type bwConfigPack struct {
	Configuration struct {
		Contents []struct {
			File  string `json:"file"`
			Index int    `json:"index"`
			Type  string `json:"type"`
		} `json:"contents"`
	} `json:"configuration"`
}

func (b *BookWalker) fetchPages(auth *bwTrialAuth) ([]string, error) {
	baseURL := auth.URL
	if auth.CTY == 0 {
		baseURL += "normal_default/"
	}
	if !strings.HasSuffix(baseURL, "/") {
		baseURL += "/"
	}

	authString := fmt.Sprintf("?pfCd=%s&Policy=%s&Signature=%s&Key-Pair-Id=%s",
		auth.AuthInfo.PfCd,
		auth.AuthInfo.Policy,
		auth.AuthInfo.Signature,
		auth.AuthInfo.KeyPairID,
	)

	configURL := baseURL + "configuration_pack.json" + authString

	req, err := http.NewRequest("GET", configURL, nil)
	if err != nil {
		return nil, err
	}
	fetcher.ApplyDefaultBrowserHeaders(req, "viewer-epubs-trial.bookwalker.jp")

	resp, err := b.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch configuration_pack.json: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("configuration_pack.json returned status: %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var pack bwConfigPack
	if err := json.Unmarshal(bodyBytes, &pack); err != nil {
		return nil, fmt.Errorf("failed to parse configuration_pack.json: %w", err)
	}

	contents := pack.Configuration.Contents
	if len(contents) == 0 {
		// Fallback: search keys in raw JSON for .xhtml
		var rawMap map[string]interface{}
		if err := json.Unmarshal(bodyBytes, &rawMap); err == nil {
			var files []string
			for k := range rawMap {
				if strings.Contains(k, ".xhtml") {
					files = append(files, k)
				}
			}
			sort.Strings(files)
			for idx, f := range files {
				contents = append(contents, struct {
					File  string `json:"file"`
					Index int    `json:"index"`
					Type  string `json:"type"`
				}{File: f, Index: idx, Type: "jpeg"})
			}
		}
	} else {
		sort.Slice(contents, func(i, j int) bool {
			return contents[i].Index < contents[j].Index
		})
	}

	var images []string
	for _, item := range contents {
		if strings.TrimSpace(item.File) == "" {
			continue
		}
		pageURL := fmt.Sprintf("%s%s/0.jpeg%s", baseURL, item.File, authString)
		images = append(images, pageURL)
	}

	return images, nil
}

func init() {
	Register(NewBookWalker())
}
