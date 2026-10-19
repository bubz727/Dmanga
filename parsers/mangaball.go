package parsers

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"manga-downloader/core"
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
	}
}

// GetChapter extracts both the chapter title and image URLs
func (m *MangaBall) GetChapter(chapterURL string) (*core.Chapter, error) {
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
		return nil, core.ChapterIDNotFoundError(chapterURL)
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
		return nil, core.HTTPStatusError(resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type MangaBallResponse struct {
		Data struct {
			Chapter struct {
				Name        string   `json:"name"`
				ChapterName string   `json:"chapter_name"`
				Title       string   `json:"title"`
				Pages       []string `json:"pages"`
			} `json:"chapter"`
		} `json:"data"`
	}

	var res MangaBallResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, core.JSONParseError("MangaBall", err)
	}

	rawTitle := strings.TrimSpace(res.Data.Chapter.ChapterName)
	if rawTitle == "" {
		rawTitle = strings.TrimSpace(res.Data.Chapter.Name)
	}
	if rawTitle == "" {
		rawTitle = strings.TrimSpace(res.Data.Chapter.Title)
	}
	chapterTitle := core.NormalizeChapterTitle(rawTitle)

	var images []string
	for _, raw := range res.Data.Chapter.Pages {
		clean := core.CleanImageURL(raw, baseHost)
		if clean != "" {
			images = append(images, clean)
		}
	}

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (m *MangaBall) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func init() {
	core.Register(NewMangaBall())
}
