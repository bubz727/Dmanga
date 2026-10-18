package parsers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"manga-downloader/themes"
)

type XComic struct{}

func NewXComic() *XComic {
	return &XComic{}
}

func (x *XComic) Name() string {
	return "XComic"
}

func (x *XComic) Domains() []string {
	return []string{
		"xcomic.me",
		"xcomic.net",
		"comik.to",
		"yona.to",
	}
}

const xcomicGraphQLQuery = `query get_chapterNode($id: ID!) {
	get_chapterNode(id: $id) {
		id
		data {
			imageUrls
		}
	}
}`

func (x *XComic) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

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
		return nil, fmt.Errorf("tidak dapat menemukan chapter ID dari URL XComic: %s", chapterURL)
	}

	payload := map[string]interface{}{
		"query": xcomicGraphQLQuery,
		"variables": map[string]string{
			"id": chapterID,
		},
	}
	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return nil, err
	}

	apiURL := fmt.Sprintf("https://%s/query/", baseHost)
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("POST", apiURL, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", baseHost))
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36")

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("GraphQL error: HTTP %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	type XComicResponse struct {
		Data struct {
			GetChapterNode struct {
				Data struct {
					ImageUrls []string `json:"imageUrls"`
				} `json:"data"`
			} `json:"get_chapterNode"`
		} `json:"data"`
	}

	var res XComicResponse
	if err := json.Unmarshal(bodyBytes, &res); err != nil {
		return nil, fmt.Errorf("gagal parse GraphQL JSON: %v", err)
	}

	rawUrls := res.Data.GetChapterNode.Data.ImageUrls
	if len(rawUrls) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan untuk chapter XComic ini")
	}

	var images []string
	for _, raw := range rawUrls {
		clean := themes.CleanImageURL(raw, baseHost)
		if clean != "" {
			images = append(images, clean)
		}
	}

	return images, nil
}

func init() {
	Register(NewXComic())
}
