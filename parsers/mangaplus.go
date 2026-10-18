package parsers

import (
	"crypto/rand"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
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

	// Format URL: https://mangaplus.shueisha.co.jp/viewer/{chapterId}
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
		return nil, fmt.Errorf("tidak dapat menemukan chapter ID dari URL MangaPlus: %s", chapterURL)
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
		return nil, fmt.Errorf("MangaPlus API error: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	// Ekstrak URL gambar dari stream protobuf
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
		// Fallback: cari URL yang mengandung tokyo-cdn atau mangaplus
		fallbackRegex := regexp.MustCompile(`https://[a-zA-Z0-9.\-_/]+(?:tokyo-cdn|mangaplus)[a-zA-Z0-9.\-_/]+`)
		for _, b := range fallbackRegex.FindAll(bodyBytes, -1) {
			u := string(b)
			if !seen[u] {
				seen[u] = true
				images = append(images, u)
			}
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan untuk chapter MangaPlus ini")
	}

	return images, nil
}

func generateUUID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16])
}

func init() {
	Register(NewMangaPlus())
}
