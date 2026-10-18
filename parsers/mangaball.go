package parsers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"manga-downloader/themes"
)

type MangaBall struct{}

func NewMangaBall() *MangaBall {
	return &MangaBall{}
}

func (m *MangaBall) Name() string {
	return "MangaBall"
}

func (m *MangaBall) Domains() []string {
	return []string{
		"mangaball.com",
		"mangaball.net",
	}
}

func (m *MangaBall) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	cleanPath := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(cleanPath, "/")

	chapterID := ""
	for i, p := range parts {
		if (p == "chapter" || p == "chapter-detail") && i+1 < len(parts) {
			chapterID = parts[i+1]
			break
		}
	}
	if chapterID == "" && len(parts) > 0 {
		chapterID = parts[len(parts)-1]
	}

	if chapterID == "" {
		return nil, fmt.Errorf("tidak dapat menemukan chapter ID dari URL MangaBall: %s", chapterURL)
	}

	apiURL := fmt.Sprintf("https://%s/api/v1/chapter-detail?chapter_id=%s", baseHost, chapterID)
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", baseHost))
	req.Header.Set("Origin", fmt.Sprintf("https://%s", baseHost))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API error: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type MangaBallResponse struct {
		Data struct {
			Chapter struct {
				Pages []string `json:"pages"`
			} `json:"chapter"`
		} `json:"data"`
	}

	var res MangaBallResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("gagal parse JSON MangaBall: %v", err)
	}

	var images []string
	for _, raw := range res.Data.Chapter.Pages {
		clean := themes.CleanImageURL(raw, baseHost)
		if clean != "" {
			images = append(images, clean)
		}
	}

	if len(images) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan untuk chapter MangaBall ini")
	}

	return images, nil
}

func init() {
	Register(NewMangaBall())
}
