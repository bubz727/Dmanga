package parser

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

// Manganato parser for manganato.gg and associated domains
type Manganato struct{}

func NewManganato() *Manganato {
	return &Manganato{}
}

func (m *Manganato) Name() string {
	return "Manganato"
}

func (m *Manganato) Domains() []string {
	return []string{
		"manganato.gg",
		"www.manganato.gg",
	}
}

func (m *Manganato) getChapterInternal(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	}

	doc, _, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	selectors := []string{
		"div.container-chapter-reader img",
		"div.panel-read-story img",
		"#chapter-reader img",
		"div.reading-content img",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			val := ""
			for _, attr := range []string{"data-src", "data-lazy-src", "data-original", "src"} {
				if a, exists := s.Attr(attr); exists && strings.TrimSpace(a) != "" {
					a = strings.TrimSpace(a)
					if !strings.HasPrefix(a, "data:") {
						val = a
						break
					}
				}
			}

			val = utils.CleanImageURL(val, parsedURL.Host)
			if val != "" && !seen[val] {
				lower := strings.ToLower(val)
				if !strings.Contains(lower, "avatar") &&
					!strings.Contains(lower, "logo") &&
					!strings.Contains(lower, "cover") &&
					!strings.Contains(lower, "banner") &&
					!strings.Contains(lower, "icon") {
					seen[val] = true
					images = append(images, val)
				}
			}
		})

		if len(images) > 0 {
			break
		}
	}

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	text := strings.TrimSpace(doc.Find(".info-top-chapter h2, div.panel-breadcrumb a:last-child, h1").First().Text())
	if num := utils.ExtractChapterNumber(text); num != "" {
		chapterTitle = fmt.Sprintf("Ch %s", num)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (m *Manganato) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

// GetChapter extracts both the chapter title and image URLs
func (m *Manganato) GetChapter(chapterURL string) (*core.Chapter, error) {
	return m.getChapterInternal(chapterURL)
}

func init() {
	Register(NewManganato())
}
