package parser

import (
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/image/descramble"
	"dmanga/internal/utils"
)

var (
	reMangaUpGlobalChapterID = regexp.MustCompile(`/(?:manga/\d+/|viewer/|chapter/)(\d+)`)
	reMangaUpGlobalTitleID   = regexp.MustCompile(`/manga/(\d+)`)
	reMangaUpGlobalChNum     = regexp.MustCompile(`(?i)(?:chapter|chap|ch|episode|ep)[.\s-_:#]*([0-9]+)(?:[\s\-._/()]+([0-9]+)\)?)?`)
)

// FormatMangaUpGlobalChapterTitle formats MangaUp Global chapter names (e.g. "Chapter 1 -1") into "Ch 1.1"
func FormatMangaUpGlobalChapterTitle(titleName, subName string, chapterID uint32) string {
	for _, s := range []string{titleName, subName} {
		s = strings.TrimSpace(s)
		if s == "" {
			continue
		}
		if m := reMangaUpGlobalChNum.FindStringSubmatch(s); len(m) > 1 {
			if len(m) > 2 && m[2] != "" {
				return fmt.Sprintf("Ch %s.%s", m[1], m[2])
			}
			return fmt.Sprintf("Ch %s", m[1])
		}
		if num := utils.ExtractChapterNumber(s); num != "" {
			return fmt.Sprintf("Ch %s", num)
		}
	}
	if strings.TrimSpace(titleName) != "" {
		return utils.SanitizeFolderName(titleName)
	}
	return fmt.Sprintf("Ch %d", chapterID)
}

type MangaUpGlobal struct {
	client *http.Client
}

func NewMangaUpGlobal() *MangaUpGlobal {
	return &MangaUpGlobal{
		client: &http.Client{Timeout: 30 * time.Second},
	}
}

func (m *MangaUpGlobal) Name() string {
	return "MangaUpGlobal"
}

func (m *MangaUpGlobal) Domains() []string {
	return []string{
		"global.manga-up.com",
	}
}

func (m *MangaUpGlobal) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (m *MangaUpGlobal) GetChapter(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	titleID, chapterID := m.extractIDs(chapterURL, parsedURL)

	// If only titleID is given (user passed manga series URL), resolve first readable chapter
	if chapterID == 0 && titleID > 0 {
		resolvedChapterID, seriesTitle, chapTitle, err := m.resolveChapterFromManga(titleID, chapterURL)
		if err != nil {
			return nil, err
		}
		chapterID = resolvedChapterID
		return m.fetchChapter(titleID, chapterID, seriesTitle, chapTitle)
	}

	if chapterID == 0 {
		return nil, fmt.Errorf("failed to extract chapter ID from URL: %s", chapterURL)
	}

	return m.fetchChapter(titleID, chapterID, "", "")
}

func (m *MangaUpGlobal) extractIDs(rawURL string, parsedURL *url.URL) (uint32, uint32) {
	var titleID, chapterID uint32

	// Check query params first: ?chapter_id=... or ?id=...
	if cidStr := parsedURL.Query().Get("chapter_id"); cidStr != "" {
		if id, err := strconv.ParseUint(cidStr, 10, 32); err == nil {
			chapterID = uint32(id)
		}
	} else if cidStr := parsedURL.Query().Get("id"); cidStr != "" {
		if id, err := strconv.ParseUint(cidStr, 10, 32); err == nil {
			chapterID = uint32(id)
		}
	}

	if tidStr := parsedURL.Query().Get("title_id"); tidStr != "" {
		if id, err := strconv.ParseUint(tidStr, 10, 32); err == nil {
			titleID = uint32(id)
		}
	}

	// Check URL path: /manga/<titleId>/<chapterId>
	pathParts := strings.Split(strings.Trim(parsedURL.Path, "/"), "/")
	if len(pathParts) >= 3 && pathParts[0] == "manga" {
		if tid, err := strconv.ParseUint(pathParts[1], 10, 32); err == nil {
			titleID = uint32(tid)
		}
		if cid, err := strconv.ParseUint(pathParts[2], 10, 32); err == nil {
			chapterID = uint32(cid)
		}
	} else if len(pathParts) == 2 && pathParts[0] == "manga" {
		if tid, err := strconv.ParseUint(pathParts[1], 10, 32); err == nil {
			titleID = uint32(tid)
		}
	} else if len(pathParts) >= 2 && (pathParts[0] == "viewer" || pathParts[0] == "chapter") {
		if cid, err := strconv.ParseUint(pathParts[1], 10, 32); err == nil {
			chapterID = uint32(cid)
		}
	}

	if chapterID == 0 {
		if match := reMangaUpGlobalChapterID.FindStringSubmatch(rawURL); len(match) > 1 {
			if id, err := strconv.ParseUint(match[1], 10, 32); err == nil {
				chapterID = uint32(id)
			}
		}
	}

	if titleID == 0 {
		if match := reMangaUpGlobalTitleID.FindStringSubmatch(rawURL); len(match) > 1 {
			if id, err := strconv.ParseUint(match[1], 10, 32); err == nil {
				titleID = uint32(id)
			}
		}
	}

	return titleID, chapterID
}

func (m *MangaUpGlobal) resolveChapterFromManga(titleID uint32, originalURL string) (uint32, string, string, error) {
	detailURL := fmt.Sprintf("https://global-api.manga-up.com/api/manga/detail_v2?title_id=%d", titleID)
	req, err := http.NewRequest("GET", detailURL, nil)
	if err != nil {
		return 0, "", "", err
	}
	m.setAPIHeaders(req)

	resp, err := m.client.Do(req)
	if err != nil {
		return 0, "", "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, "", "", fmt.Errorf("detail_v2 failed with status: %d", resp.StatusCode)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, "", "", err
	}

	seriesTitle, chapters := parseMangaDetailProto(buf)
	if len(chapters) == 0 {
		return 0, "", "", fmt.Errorf("no chapters found for title ID: %d", titleID)
	}

	// Pick first available chapter
	first := chapters[0]
	chapName := FormatMangaUpGlobalChapterTitle(first.titleName, first.subName, first.id)
	return first.id, seriesTitle, chapName, nil
}

func (m *MangaUpGlobal) fetchChapter(titleID, chapterID uint32, preSeriesTitle, preChapTitle string) (*core.Chapter, error) {
	apiURL := fmt.Sprintf("https://global-api.manga-up.com/api/manga/viewer_v2?chapter_id=%d&quality=high", chapterID)
	req, err := http.NewRequest("POST", apiURL, nil)
	if err != nil {
		return nil, err
	}
	m.setAPIHeaders(req)

	resp, err := m.client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("viewer_v2 failed with status: %d", resp.StatusCode)
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	pages := parseViewerPagesProto(buf)
	if len(pages) == 0 {
		return nil, core.ErrNoImagesFound
	}

	images := make([]string, 0, len(pages))
	for _, p := range pages {
		fullURL := p.imageURL
		if !strings.HasPrefix(fullURL, "http://") && !strings.HasPrefix(fullURL, "https://") {
			fullURL = "https://global-img.manga-up.com/" + strings.TrimPrefix(fullURL, "/")
		}
		if p.encryptionKey != "" && p.iv != "" {
			fullURL += fmt.Sprintf("#key=%s&iv=%s", p.encryptionKey, p.iv)
		}
		images = append(images, fullURL)
	}

	chapterTitle := preChapTitle

	if chapterTitle == "" && titleID > 0 {
		detailURL := fmt.Sprintf("https://global-api.manga-up.com/api/manga/detail_v2?title_id=%d", titleID)
		if dReq, dErr := http.NewRequest("GET", detailURL, nil); dErr == nil {
			m.setAPIHeaders(dReq)
			if dResp, dDoErr := m.client.Do(dReq); dDoErr == nil && dResp.StatusCode == http.StatusOK {
				defer dResp.Body.Close()
				if dBuf, rErr := io.ReadAll(dResp.Body); rErr == nil {
					_, chaps := parseMangaDetailProto(dBuf)
					for _, c := range chaps {
						if c.id == chapterID {
							chapterTitle = FormatMangaUpGlobalChapterTitle(c.titleName, c.subName, chapterID)
							break
						}
					}
				}
			}
		}
	}

	if chapterTitle == "" {
		chapterTitle = fmt.Sprintf("Ch %d", chapterID)
	}

	return &core.Chapter{
		Title:  utils.SanitizeFolderName(chapterTitle),
		Images: images,
	}, nil
}

func (m *MangaUpGlobal) setAPIHeaders(req *http.Request) {
	fetcher.ApplyDefaultBrowserHeaders(req, "global.manga-up.com")
	req.Header.Set("Referer", "https://global.manga-up.com/")
	req.Header.Set("Origin", "https://global.manga-up.com")
	req.Header.Set("Accept", "*/*")
}

// ProcessImage decrypts AES-CBC encrypted image data
func (m *MangaUpGlobal) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	return descramble.DecryptMangaUpGlobal(rawBytes, imageURL)
}

// --- Protobuf wire-format decoding based on mangaupglobal.proto ---

type mangaPageProto struct {
	imageURL      string
	encryptionKey string
	iv            string
}

type mangaDetailChapterProto struct {
	id        uint32
	titleName string
	subName   string
}

type protoField struct {
	num  int
	wire int
	data []byte
	u64  uint64
}

func readProtoFields(data []byte) []protoField {
	var fields []protoField
	for len(data) > 0 {
		tag, n := binary.Uvarint(data)
		if n <= 0 {
			break
		}
		data = data[n:]
		wire := int(tag & 0x7)
		num := int(tag >> 3)

		switch wire {
		case 0: // varint
			val, vn := binary.Uvarint(data)
			if vn <= 0 {
				return fields
			}
			data = data[vn:]
			fields = append(fields, protoField{num: num, wire: wire, u64: val})
		case 1: // 64-bit
			if len(data) < 8 {
				return fields
			}
			fields = append(fields, protoField{num: num, wire: wire, data: data[:8]})
			data = data[8:]
		case 2: // length-delimited
			l, ln := binary.Uvarint(data)
			if ln <= 0 {
				return fields
			}
			data = data[ln:]
			if uint64(len(data)) < l {
				return fields
			}
			fields = append(fields, protoField{num: num, wire: wire, data: data[:l]})
			data = data[l:]
		case 5: // 32-bit
			if len(data) < 4 {
				return fields
			}
			fields = append(fields, protoField{num: num, wire: wire, data: data[:4]})
			data = data[4:]
		default:
			return fields
		}
	}
	return fields
}

func parseViewerPagesProto(buf []byte) []mangaPageProto {
	var pages []mangaPageProto
	for _, f := range readProtoFields(buf) {
		// MangaViewerV2View: repeated PageBlock pageblocks = 3
		if f.num == 3 && f.wire == 2 {
			for _, bf := range readProtoFields(f.data) {
				// PageBlock: repeated MangaPage pages = 3
				if bf.num == 3 && bf.wire == 2 {
					var p mangaPageProto
					for _, pf := range readProtoFields(bf.data) {
						switch pf.num {
						case 1: // string imageUrl
							p.imageURL = string(pf.data)
						case 5: // string encryptionKey
							p.encryptionKey = string(pf.data)
						case 6: // string iv
							p.iv = string(pf.data)
						}
					}
					if p.imageURL != "" {
						pages = append(pages, p)
					}
				}
			}
		}
	}
	return pages
}

func parseMangaDetailProto(buf []byte) (string, []mangaDetailChapterProto) {
	var titleName string
	var chapters []mangaDetailChapterProto

	for _, f := range readProtoFields(buf) {
		// MangaDetailView: optional string titleName = 3
		if f.num == 3 && f.wire == 2 {
			titleName = string(f.data)
		} else if f.num == 13 && f.wire == 2 { // repeated Chapter chapters = 13
			var c mangaDetailChapterProto
			for _, cf := range readProtoFields(f.data) {
				switch cf.num {
				case 1: // optional uint32 id = 1
					c.id = uint32(cf.u64)
				case 2: // optional string titleName = 2
					c.titleName = string(cf.data)
				case 3: // optional string subName = 3
					c.subName = string(cf.data)
				}
			}
			chapters = append(chapters, c)
		}
	}
	return titleName, chapters
}

func init() {
	Register(NewMangaUpGlobal())
}
