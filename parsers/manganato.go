package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
	"manga-downloader/themes"
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
		"manganato.com",
		"www.manganato.com",
		"chapmanganato.to",
		"chapmanganato.com",
		"natomanga.com",
		"www.natomanga.com",
		"readmanganato.com",
		"www.readmanganato.com",
	}
}

func (m *Manganato) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	}

	doc, _, err := themes.FetchHTML(chapterURL, headers)
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

			val = themes.CleanImageURL(val, parsedURL.Host)
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

	return core.EnsureImages(images)
}

func init() {
	Register(NewManganato())
}
