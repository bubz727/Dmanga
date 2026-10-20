package parser

import (
	"crypto/aes"
	"crypto/cipher"
	"encoding/base64"
	"encoding/hex"
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
	mgImgSrcsRegex     = regexp.MustCompile(`var\s+imgsrcs\s*=\s*['"]([a-zA-Z0-9+=/]+)['"]`)
	mgHexVarKeyRegex   = regexp.MustCompile(`var\s+key\s*=\s*CryptoJS\.enc\.Hex\.parse\(["']([0-9a-fA-F]+)["']\)`)
	mgHexVarIvRegex    = regexp.MustCompile(`var\s+iv\s*=\s*CryptoJS\.enc\.Hex\.parse\(["']([0-9a-fA-F]+)["']\)`)
	mgKeyLocationRegex = regexp.MustCompile(`str\.charAt\(\s*(\d+)\s*\)`)
	mgSplitLetters     = regexp.MustCompile(`[a-zA-Z]+`)
)

type Mangago struct{}

func NewMangago() *Mangago {
	return &Mangago{}
}

func (m *Mangago) Name() string {
	return "Mangago"
}

func (m *Mangago) Domains() []string {
	return []string{
		"mangago.me",
	}
}

// GetChapter extracts both the chapter title and image URLs
func (m *Mangago) GetChapter(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Cookie":  "_m_superu=1",
		"Referer": fmt.Sprintf("https://%s/", baseHost),
	}

	doc, _, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	rawTitle := strings.TrimSpace(doc.Find("a#dropdown-chapter-page, .btn-group a.chapter").First().Text())
	if rawTitle == "" {
		rawTitle = strings.TrimSpace(doc.Find("head title").Text())
	}
	chapterTitle = utils.NormalizeChapterTitle(rawTitle)

	// Find and deobfuscate chapter.js once for the entire batch
	var chapterJSURL string
	doc.Find("script[src*='chapter.js']").Each(func(i int, s *goquery.Selection) {
		if src, exists := s.Attr("src"); exists {
			chapterJSURL = utils.CleanImageURL(src, baseHost)
		}
	})

	if chapterJSURL == "" {
		return nil, core.ErrScriptNotFound
	}

	obfuscatedJS, err := fetchRawText(chapterJSURL, headers)
	if err != nil {
		return nil, fmt.Errorf("failed to download chapter.js: %w", err)
	}

	deobfJS, err := deobfuscateSoJsonV4(obfuscatedJS)
	if err != nil {
		return nil, fmt.Errorf("failed to deobfuscate chapter.js: %w", err)
	}

	keyMatch := mgHexVarKeyRegex.FindStringSubmatch(deobfJS)
	ivMatch := mgHexVarIvRegex.FindStringSubmatch(deobfJS)
	if len(keyMatch) < 2 || len(ivMatch) < 2 {
		return nil, fmt.Errorf("failed to find key or iv in chapter.js")
	}

	keyBytes, _ := hex.DecodeString(keyMatch[1])
	ivBytes, _ := hex.DecodeString(ivMatch[1])

	pageDropdown := doc.Find("div.controls ul#dropdown-menu-page li, ul#dropdown-menu-page li")
	pagesCount := pageDropdown.Length()

	var images []string
	if pagesCount > 1 {
		cleanURL := strings.TrimRight(chapterURL, "/")
		parts := strings.Split(cleanURL, "/")
		lastSegment := parts[len(parts)-1]

		isPgFormat := strings.HasPrefix(lastSegment, "pg-")
		_, isPageNumberErr := strconv.Atoi(lastSegment)
		isPageNumber := isPageNumberErr == nil

		var baseURL string
		if isPgFormat || isPageNumber {
			baseURL = strings.Join(parts[:len(parts)-1], "/")
		} else {
			baseURL = cleanURL
		}

		batchSize := 5
		var allImages []string
		batchStart := 1

		for len(allImages) < pagesCount {
			var batchDoc *goquery.Document
			if batchStart == 1 {
				batchDoc = doc
			} else {
				var batchURL string
				if isPgFormat {
					batchURL = fmt.Sprintf("%s/pg-%d/", baseURL, batchStart)
				} else {
					batchURL = fmt.Sprintf("%s/%d/", baseURL, batchStart)
				}
				bDoc, _, bErr := fetcher.FetchHTML(batchURL, headers)
				if bErr != nil {
					break
				}
				batchDoc = bDoc
			}

			batchImages, bErr := decryptDocImages(batchDoc, deobfJS, keyBytes, ivBytes)
			if bErr != nil || len(batchImages) == 0 {
				break
			}

			allImages = append(allImages, batchImages...)
			batchStart += batchSize

			if batchStart > pagesCount+batchSize {
				break
			}
		}

		if len(allImages) > pagesCount {
			allImages = allImages[:pagesCount]
		}
		validImgs, err := core.EnsureImages(allImages)
		if err != nil {
			return nil, err
		}
		images = validImgs
	} else {
		// Single page
		validImgs, err := decryptDocImages(doc, deobfJS, keyBytes, ivBytes)
		if err != nil {
			return nil, err
		}
		images = validImgs
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

// GetImages implements core.Parser
func (m *Mangago) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func decryptDocImages(doc *goquery.Document, deobfJS string, keyBytes, ivBytes []byte) ([]string, error) {
	var imgsrcsRaw string
	doc.Find("script").Each(func(i int, s *goquery.Selection) {
		text := s.Text()
		if strings.Contains(text, "imgsrcs") && imgsrcsRaw == "" {
			if matches := mgImgSrcsRegex.FindStringSubmatch(text); len(matches) > 1 {
				imgsrcsRaw = matches[1]
			}
		}
	})

	if imgsrcsRaw == "" {
		html, _ := doc.Html()
		if matches := mgImgSrcsRegex.FindStringSubmatch(html); len(matches) > 1 {
			imgsrcsRaw = matches[1]
		}
	}

	if imgsrcsRaw == "" {
		return nil, fmt.Errorf("failed to find imgsrcs data")
	}

	imgsrcBytes, err := base64.StdEncoding.DecodeString(imgsrcsRaw)
	if err != nil {
		return nil, err
	}

	decryptedBytes, err := decryptAESCBC(imgsrcBytes, keyBytes, ivBytes)
	if err != nil {
		return nil, err
	}

	decryptedStr := strings.TrimRight(string(decryptedBytes), "\x00")
	unscrambled := unscrambleImageList(decryptedStr, deobfJS)

	rawParts := strings.Split(unscrambled, ",")
	var images []string
	for _, p := range rawParts {
		u := strings.TrimSpace(p)
		if u == "" {
			continue
		}
		if (strings.HasPrefix(u, "https://") && strings.Contains(u, "/_")) || strings.Contains(u, "https://iweb_") {
			u = strings.Replace(u, "https://", "http://", 1)
		}
		images = append(images, u)
	}

	return core.EnsureImages(images)
}

func fetchRawText(targetURL string, headers map[string]string) (string, error) {
	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", targetURL, nil)
	if err != nil {
		return "", err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	return string(bodyBytes), nil
}

func deobfuscateSoJsonV4(jsf string) (string, error) {
	if !strings.HasPrefix(jsf, "['sojson.v4']") {
		return "", fmt.Errorf("not sojson.v4 format")
	}
	if len(jsf) < 300 {
		return "", fmt.Errorf("invalid script length")
	}
	middle := jsf[240 : len(jsf)-59]
	args := mgSplitLetters.Split(middle, -1)

	var sb strings.Builder
	for _, a := range args {
		if a == "" {
			continue
		}
		if val, err := strconv.Atoi(a); err == nil {
			sb.WriteRune(rune(val))
		}
	}
	return sb.String(), nil
}

func decryptAESCBC(ciphertext, key, iv []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	if len(ciphertext)%aes.BlockSize != 0 {
		padLen := aes.BlockSize - (len(ciphertext) % aes.BlockSize)
		ciphertext = append(ciphertext, make([]byte, padLen)...)
	}

	decrypted := make([]byte, len(ciphertext))
	mode := cipher.NewCBCDecrypter(block, iv)
	mode.CryptBlocks(decrypted, ciphertext)
	return decrypted, nil
}

func unscrambleImageList(imageList, js string) string {
	matches := mgKeyLocationRegex.FindAllStringSubmatch(js, -1)
	if len(matches) == 0 {
		return imageList
	}

	locMap := make(map[int]bool)
	var keyLocations []int
	for _, m := range matches {
		if len(m) > 1 {
			if loc, err := strconv.Atoi(m[1]); err == nil && !locMap[loc] {
				locMap[loc] = true
				keyLocations = append(keyLocations, loc)
			}
		}
	}
	sort.Ints(keyLocations)

	var unscrambleKey []int
	for _, loc := range keyLocations {
		if loc >= len(imageList) {
			return imageList
		}
		digit, err := strconv.Atoi(string(imageList[loc]))
		if err != nil {
			return imageList
		}
		unscrambleKey = append(unscrambleKey, digit)
	}

	cleanedList := imageList
	for idx, loc := range keyLocations {
		target := loc - idx
		if target >= 0 && target < len(cleanedList) {
			cleanedList = cleanedList[:target] + cleanedList[target+1:]
		}
	}

	s := []rune(cleanedList)
	for i := len(unscrambleKey) - 1; i >= 0; i-- {
		key := unscrambleKey[i]
		for j := len(s) - 1; j >= key; j-- {
			if j%2 != 0 {
				sourceIdx := j - key
				if sourceIdx >= 0 {
					temp := s[sourceIdx]
					s[sourceIdx] = s[j]
					s[j] = temp
				}
			}
		}
	}

	return string(s)
}

func init() {
	Register(NewMangago())
}
