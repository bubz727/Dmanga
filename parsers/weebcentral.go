package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
)

type WeebCentral struct{}

func NewWeebCentral() *WeebCentral {
	return &WeebCentral{}
}

func (w *WeebCentral) Name() string {
	return "Weeb Central"
}

func (w *WeebCentral) Domains() []string {
	return []string{
		"weebcentral.com",
	}
}

func (w *WeebCentral) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	// https://weebcentral.com/chapters/{chapterId}/images?is_prev=False&reading_style=long_strip
	var imagesURL string
	if strings.Contains(chapterURL, "/images") {
		imagesURL = chapterURL
	} else {
		// Clean path to extract chapter ID
		cleanPath := strings.Trim(parsedURL.Path, "/")
		parts := strings.Split(cleanPath, "/")

		// URL format: /chapters/{id} or /chapters/{id}/...
		chapterID := ""
		for i, part := range parts {
			if part == "chapters" && i+1 < len(parts) {
				chapterID = parts[i+1]
				break
			}
		}

		if chapterID != "" {
			imagesURL = fmt.Sprintf("https://%s/chapters/%s/images?is_prev=False&reading_style=long_strip", parsedURL.Host, chapterID)
		} else {
			imagesURL = chapterURL
		}
	}

	doc, _, err := core.FetchHTML(imagesURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	})
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	doc.Find("section[x-data~=scroll] > img, section > img").Each(func(i int, s *goquery.Selection) {
		imgURL, exists := s.Attr("src")
		if !exists || strings.TrimSpace(imgURL) == "" {
			imgURL, _ = s.Attr("data-src")
		}

		imgURL = core.CleanImageURL(imgURL, parsedURL.Host)
		if imgURL != "" && !seen[imgURL] {
			seen[imgURL] = true
			images = append(images, imgURL)
		}
	})

	return core.EnsureImages(images)
}

func init() {
	core.Register(NewWeebCentral())
}
