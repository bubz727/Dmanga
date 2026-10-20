package themes

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

type Natsu struct {
	PageSelector string
}

// NewNatsu initializes the default configuration for Natsu theme
func NewNatsu() Natsu {
	return Natsu{
		PageSelector: "main section section > img, main section img, div.reader-area img",
	}
}

// ExtractImages extracts all chapter images based on natsu.kt logic
func (n *Natsu) ExtractImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
		"Origin":  fmt.Sprintf("https://%s", baseHost),
	}

	doc, _, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	doc.Find(n.PageSelector).Each(func(i int, s *goquery.Selection) {
		imgURL := ""
		for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		imgURL = utils.CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "banner") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}
	return validImgs, nil
}

// ExtractChapter extracts both chapter title and images based on Natsu theme
func (n *Natsu) ExtractChapter(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
		"Origin":  fmt.Sprintf("https://%s", baseHost),
	}

	doc, _, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	doc.Find(n.PageSelector).Each(func(i int, s *goquery.Selection) {
		imgURL := ""
		for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		imgURL = utils.CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "banner") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	text := strings.TrimSpace(doc.Find("div.font-semibold.text-gray-50, h1.entry-title, .headpost h1").First().Text())
	if num := utils.ExtractChapterNumber(text); num != "" {
		chapterTitle = fmt.Sprintf("Ch %s", num)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}
