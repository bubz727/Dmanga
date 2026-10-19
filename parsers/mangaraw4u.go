package parsers

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
)

var mangaraw4uImageRegex = regexp.MustCompile(`https?://[^"'\s<>]+\.(?:jpg|jpeg|png|webp)`)

// MangaRaw4U parser for mangaraw4u.com
type MangaRaw4U struct{}

func NewMangaRaw4U() *MangaRaw4U {
	return &MangaRaw4U{}
}

func (m *MangaRaw4U) Name() string {
	return "MangaRaw4U"
}

func (m *MangaRaw4U) Domains() []string {
	return []string{
		"mangaraw4u.com",
		"www.mangaraw4u.com",
	}
}

func (m *MangaRaw4U) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	}

	doc, rawHTML, err := core.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	// 1. Primary extraction: inspect images inside reader container
	selectors := []string{
		"#readerPages img",
		".reader-pages img",
		"div.img-wrap img",
		"div.reading-content img",
		"div.entry-content img",
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

			val = core.CleanImageURL(val, parsedURL.Host)
			if val != "" && !seen[val] {
				lower := strings.ToLower(val)
				if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "icon") && !strings.Contains(lower, "banner") {
					seen[val] = true
					images = append(images, val)
				}
			}
		})

		if len(images) > 0 {
			break
		}
	}

	// 2. Fallback: regex search across raw HTML if DOM container was missing
	if len(images) == 0 && rawHTML != "" {
		matches := mangaraw4uImageRegex.FindAllString(rawHTML, -1)
		for _, m := range matches {
			lower := strings.ToLower(m)
			if !strings.Contains(lower, "cover") && !strings.Contains(lower, "thumb") && !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "icon") {
				if !seen[m] {
					seen[m] = true
					images = append(images, m)
				}
			}
		}
	}

	return core.EnsureImages(images)
}

// GetChapter extracts both the chapter title and image URLs
func (m *MangaRaw4U) GetChapter(chapterURL string) (*core.Chapter, error) {
	images, err := m.GetImages(chapterURL)
	if err != nil {
		return nil, err
	}

	chapterTitle := core.ExtractMangaRaw4UChapterTitle(chapterURL)

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

func init() {
	core.Register(NewMangaRaw4U())
}
