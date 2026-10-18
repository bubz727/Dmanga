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

type MangaDex struct{}

func NewMangaDex() *MangaDex {
	return &MangaDex{}
}

func (m *MangaDex) Name() string {
	return "MangaDex"
}

func (m *MangaDex) Domains() []string {
	return []string{
		"mangadex.org",
	}
}

type mangaDexAtHomeResponse struct {
	Result  string `json:"result"`
	BaseURL string `json:"baseUrl"`
	Chapter struct {
		Hash      string   `json:"hash"`
		Data      []string `json:"data"`
		DataSaver []string `json:"dataSaver"`
	} `json:"chapter"`
}

func (m *MangaDex) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	// URL format: https://mangadex.org/chapter/{chapterId} or /chapter/{chapterId}/1
	cleanPath := strings.Trim(parsedURL.Path, "/")
	parts := strings.Split(cleanPath, "/")

	chapterID := ""
	for i, p := range parts {
		if p == "chapter" && i+1 < len(parts) {
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

	apiURL := fmt.Sprintf("https://api.mangadex.org/at-home/server/%s?forcePort443=false", chapterID)

	client := &http.Client{Timeout: 30 * time.Second}
	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")
	req.Header.Set("Accept", "application/json")

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

	var res mangaDexAtHomeResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, core.JSONParseError("MangaDex", err)
	}

	if res.Result != "ok" || len(res.Chapter.Data) == 0 {
		return nil, core.ErrNoImagesFound
	}

	var images []string
	prefix := fmt.Sprintf("%s/data/%s/", res.BaseURL, res.Chapter.Hash)
	for _, filename := range res.Chapter.Data {
		images = append(images, prefix+filename)
	}

	return core.EnsureImages(images)
}

func init() {
	Register(NewMangaDex())
}
