package parser

import (
	"fmt"
	"net/url"
	"strings"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
	"github.com/PuerkitoBio/goquery"
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

	doc, _, err := fetcher.FetchHTML(imagesURL, map[string]string{
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

		imgURL = utils.CleanImageURL(imgURL, parsedURL.Host)
		if imgURL != "" && !seen[imgURL] {
			seen[imgURL] = true
			images = append(images, imgURL)
		}
	})

	return core.EnsureImages(images)
}

// GetChapter extracts both the chapter title and image URLs
func (w *WeebCentral) GetChapter(chapterURL string) (*core.Chapter, error) {
	images, err := w.GetImages(chapterURL)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	// Fetch reader page (without /images) to get the chapter button text
	readerURL := chapterURL
	if idx := strings.Index(readerURL, "/images"); idx != -1 {
		readerURL = readerURL[:idx]
	}

	parsedURL, _ := url.Parse(readerURL)
	host := ""
	if parsedURL != nil {
		host = parsedURL.Host
	}

	doc, _, err := fetcher.FetchHTML(readerURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", host),
	})
	if err == nil && doc != nil {
		text := strings.TrimSpace(doc.Find("button[onclick*='chapter_select_modal'] span, button.btn-secondary span, h1").First().Text())
		if num := utils.ExtractChapterNumber(text); num != "" {
			chapterTitle = fmt.Sprintf("Ch %s", num)
		}
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

func init() {
	Register(NewWeebCentral())
}
