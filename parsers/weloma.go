package parsers

import (
	"encoding/base64"
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
)

// Weloma parser for weloma.net
type Weloma struct{}

func NewWeloma() *Weloma {
	return &Weloma{}
}

func (w *Weloma) Name() string {
	return "Weloma"
}

func (w *Weloma) Domains() []string {
	return []string{
		"weloma.net",
		"www.weloma.net",
		"weloma.art",
	}
}

func (w *Weloma) getChapterInternal(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsedURL.Host),
	}

	doc, _, err := core.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	selectors := []string{
		"#chapter-images img",
		"div.chapter-content img.chapter-img",
		"img.chapter-img",
		"#chapter_images_container img",
		".chapter-images img",
		"div.reading-content img",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			val := ""

			// 1. Check data-img for base64 encoded URL
			if dataImg, exists := s.Attr("data-img"); exists && strings.TrimSpace(dataImg) != "" {
				if b, err := base64.StdEncoding.DecodeString(strings.TrimSpace(dataImg)); err == nil {
					decoded := strings.TrimSpace(string(b))
					if strings.HasPrefix(decoded, "http://") || strings.HasPrefix(decoded, "https://") || strings.HasPrefix(decoded, "/") {
						val = decoded
					}
				}
			}

			// 2. Check standard image attributes
			if val == "" {
				for _, attr := range []string{"data-src", "data-lazy-src", "data-original", "src"} {
					if a, exists := s.Attr(attr); exists && strings.TrimSpace(a) != "" {
						a = strings.TrimSpace(a)
						if !strings.HasPrefix(a, "data:") {
							val = a
							break
						}
					}
				}
			}

			val = core.CleanImageURL(val, parsedURL.Host)
			if val != "" && !seen[val] {
				lower := strings.ToLower(val)
				if !strings.Contains(lower, "avatar") &&
					!strings.Contains(lower, "logo") &&
					!strings.Contains(lower, "cover") &&
					!strings.Contains(lower, "icon") &&
					!strings.Contains(lower, "sticker") &&
					!strings.Contains(lower, "twemoji") &&
					!strings.Contains(lower, "blogspot.com") &&
					!strings.Contains(lower, "pngtree") &&
					!strings.Contains(lower, "toppng") &&
					!strings.Contains(lower, "hiclipart") {
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
	titleText := strings.TrimSpace(doc.Find("ol.breadcrumb li:last-child span[itemprop='name'], .breadcrumb li:last-child a, .breadcrumb li:last-child").Last().Text())
	if num := core.ExtractChapterNumber(titleText); num != "" {
		chapterTitle = fmt.Sprintf("Ch %s", num)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (w *Weloma) GetImages(chapterURL string) ([]string, error) {
	chap, err := w.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

// GetChapter extracts both the chapter title and image URLs
func (w *Weloma) GetChapter(chapterURL string) (*core.Chapter, error) {
	return w.getChapterInternal(chapterURL)
}

func init() {
	core.Register(NewWeloma())
}
