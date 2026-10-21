package parser

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/image/descramble"
	"dmanga/internal/utils"
)

type MangaPlus struct{}

func NewMangaPlus() *MangaPlus {
	return &MangaPlus{}
}

func (m *MangaPlus) Name() string {
	return "MangaPlus"
}

func (m *MangaPlus) Domains() []string {
	return []string{
		"mangaplus.shueisha.co.jp",
	}
}

// ProcessImage decrypts MangaPlus XOR-encrypted images using the encryptionKey fragment
func (m *MangaPlus) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	return descramble.DecryptMangaPlus(rawBytes, imageURL)
}

func (m *MangaPlus) fetchViewerData(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	cleanPath := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(cleanPath, "/")

	chapterID := ""
	for i, p := range parts {
		if p == "viewer" && i+1 < len(parts) {
			chapterID = parts[i+1]
			break
		}
	}
	if chapterID == "" && len(parts) > 0 {
		chapterID = parts[len(parts)-1]
	}

	if chapterID == "" {
		return nil, core.ChapterIDNotFoundError(chapterURL)
	}

	sessionToken := generateUUID()
	apiURL := fmt.Sprintf("https://jumpg-webapi.tokyo-cdn.com/api/manga_viewer?chapter_id=%s&split=yes&img_quality=super_high", chapterID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("SESSION-TOKEN", sessionToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, core.HTTPStatusError(resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	reURL := regexp.MustCompile(`https://jumpg-assets\.tokyo-cdn\.com/[a-zA-Z0-9.\-_/?=&]+`)
	matches := reURL.FindAllIndex(bodyBytes, -1)
	keyRegex := regexp.MustCompile(`[0-9a-fA-F]{32,128}`)

	var images []string
	seen := make(map[string]bool)

	for _, loc := range matches {
		u := string(bodyBytes[loc[0]:loc[1]])
		if !bytes.Contains([]byte(u), []byte("/manga_page/")) {
			continue
		}

		// Look for encryptionKey in protobuf following imageUrl
		end := loc[1] + 300
		if end > len(bodyBytes) {
			end = len(bodyBytes)
		}
		after := bodyBytes[loc[1]:end]

		keyStr := ""
		starPos := bytes.IndexByte(after, 0x2A) // Protobuf tag for Field 5 (wire type 2)
		if starPos != -1 && starPos < 30 {
			if m := keyRegex.Find(after[starPos:]); m != nil {
				keyStr = string(m)
			}
		}

		fullURL := u
		if keyStr != "" {
			fullURL = fmt.Sprintf("%s#encryptionKey=%s", u, keyStr)
		}

		if !seen[u] {
			seen[u] = true
			images = append(images, fullURL)
		}
	}

	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	// Extract chapter title
	chapterTitle := ""
	_, chapName := parseMangaPlusSnsShare(bodyBytes, chapterID)
	if chapName != "" {
		chapterTitle = formatMangaPlusChapter(chapName)
	}
	if chapterTitle == "" {
		chapterTitle = extractMangaPlusChapterFallback(bodyBytes, chapterID)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

func (m *MangaPlus) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.fetchViewerData(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (m *MangaPlus) GetChapter(chapterURL string) (*core.Chapter, error) {
	return m.fetchViewerData(chapterURL)
}

func parseMangaPlusSnsShare(data []byte, chapterID string) (string, string) {
	needle := fmt.Sprintf("chapter_id=%s", chapterID)
	pos := bytes.Index(data, []byte(needle))
	if pos == -1 {
		return "", ""
	}
	chunk := data[pos+len(needle):]
	title := ""
	chapter := ""
	idx := 0
	for idx < len(chunk) && idx < 200 {
		tag := chunk[idx]
		idx++
		if tag == 0x2a { // Field 5: Title name
			if idx < len(chunk) {
				length := int(chunk[idx])
				idx++
				if idx+length <= len(chunk) {
					title = string(chunk[idx : idx+length])
					idx += length
				}
			}
		} else if tag == 0x32 { // Field 6: Chapter name
			if idx < len(chunk) {
				length := int(chunk[idx])
				idx++
				if idx+length <= len(chunk) {
					chapter = string(chunk[idx : idx+length])
					idx += length
				}
			}
			break
		}
	}
	return title, chapter
}

func extractMangaPlusChapterFallback(bodyBytes []byte, chapterID string) string {
	thumbNeedle := fmt.Sprintf("chapter/%s/chapter_thumbnail", chapterID)
	thumbPos := bytes.Index(bodyBytes, []byte(thumbNeedle))
	if thumbPos != -1 {
		start := thumbPos - 200
		if start < 0 {
			start = 0
		}
		chunk := bodyBytes[start:thumbPos]
		reHash := regexp.MustCompile(`#[0-9]{3}(?:,#[0-9]{3})?`)
		if m := reHash.Find(chunk); m != nil {
			return formatMangaPlusChapter(string(m))
		}
		reChap := regexp.MustCompile(`Chapter\s+[0-9]+(?:\.[0-9]+)?`)
		if m := reChap.Find(chunk); m != nil {
			return utils.NormalizeChapterTitle(string(m))
		}
	}
	return ""
}

func formatMangaPlusChapter(hashStr string) string {
	parts := strings.Split(hashStr, ",")
	var nums []string
	for _, p := range parts {
		if n := utils.ExtractChapterNumber(p); n != "" {
			nums = append(nums, n)
		}
	}
	if len(nums) > 1 {
		return fmt.Sprintf("Ch %s-%s", nums[0], nums[len(nums)-1])
	}
	if len(nums) == 1 {
		return fmt.Sprintf("Ch %s", nums[0])
	}
	return utils.NormalizeChapterTitle(hashStr)
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func init() {
	Register(NewMangaPlus())
}
