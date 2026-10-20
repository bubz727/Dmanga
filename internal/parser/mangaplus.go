package parser

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

var mpImageURLRegex = regexp.MustCompile(`https://[a-zA-Z0-9.\-_/]+\.(?:jpg|jpeg|png|webp|bin)`)

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

func (m *MangaPlus) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	// URL format: https://mangaplus.shueisha.co.jp/viewer/{chapterId}
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

	// Generate random session UUID
	sessionToken := generateUUID()

	apiURL := fmt.Sprintf("https://jumpg-webapi.tokyo-cdn.com/api/manga_viewer_v3?chapter_id=%s&split=yes&img_quality=super_high&clang=eng", chapterID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("SESSION-TOKEN", sessionToken)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

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

	// Extract image URLs from protobuf stream
	rawMatches := mpImageURLRegex.FindAll(bodyBytes, -1)
	var images []string
	seen := make(map[string]bool)

	for _, b := range rawMatches {
		u := string(b)
		if !seen[u] {
			seen[u] = true
			images = append(images, u)
		}
	}

	if len(images) == 0 {
		// Fallback: look for URLs containing tokyo-cdn or mangaplus
		fallbackRegex := regexp.MustCompile(`https://[a-zA-Z0-9.\-_/]+(?:tokyo-cdn|mangaplus)[a-zA-Z0-9.\-_/]+`)
		for _, b := range fallbackRegex.FindAll(bodyBytes, -1) {
			u := string(b)
			if !seen[u] {
				seen[u] = true
				images = append(images, u)
			}
		}
	}

	return core.EnsureImages(images)
}

// GetChapter extracts both the chapter title and image URLs
func (m *MangaPlus) GetChapter(chapterURL string) (*core.Chapter, error) {
	images, err := m.GetImages(chapterURL)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	doc, _, err := fetcher.FetchHTML(chapterURL, map[string]string{
		"User-Agent": "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36",
	})
	if err == nil {
		rawTitle := strings.TrimSpace(doc.Find("p[class*='chapterTitle'], [class*='chapterTitle'], .chapter-title").First().Text())
		if rawTitle == "" {
			rawTitle = strings.TrimSpace(doc.Find("head title").Text())
		}
		chapterTitle = utils.NormalizeChapterTitle(rawTitle)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func init() {
	Register(NewMangaPlus())
}
