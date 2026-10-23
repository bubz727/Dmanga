package parser

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

var (
	mangaUpJapanImageRegex          = regexp.MustCompile(`https://ja-img\.manga-up\.com/[^"'\s\\]+/manga/(?:high|middle|low)/[^"'\s\\]+`)
	mangaUpJapanCurrentChapterRegex = regexp.MustCompile(`"currentChapter"\s*:\s*\{[^{}]*?"name"\s*:\s*"([^"]*)"[^{}]*?"subName"\s*:\s*"([^"]*)"`)
)

type MangaUpJapan struct{}

func NewMangaUpJapan() *MangaUpJapan {
	return &MangaUpJapan{}
}

func (m *MangaUpJapan) Name() string {
	return "MangaUpJapan"
}

func (m *MangaUpJapan) Domains() []string {
	return []string{
		"manga-up.com",
		"www.manga-up.com",
	}
}

func (m *MangaUpJapan) GetImages(chapterURL string) ([]string, error) {
	chap, err := m.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (m *MangaUpJapan) GetChapter(chapterURL string) (*core.Chapter, error) {
	_, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	doc, rawHTML, err := fetcher.FetchHTML(chapterURL, map[string]string{
		"Referer": "https://www.manga-up.com/",
	})
	if err != nil {
		return nil, err
	}

	// Unescape Next.js RSC escaped strings so URLs (&expires=...) and JSON fields are intact
	unescapedHTML := strings.ReplaceAll(rawHTML, `\u0026`, "&")
	unescapedHTML = strings.ReplaceAll(unescapedHTML, `&amp;`, "&")
	unescapedHTML = strings.ReplaceAll(unescapedHTML, `\"`, `"`)

	// 1. Extract chapter title
	chapterTitle := ""

	// A. Extract from RSC "currentChapter":{"id":...,"name":"黒朗と布由 - ①","subName":"第一話"}
	if match := mangaUpJapanCurrentChapterRegex.FindStringSubmatch(unescapedHTML); len(match) >= 3 {
		name := strings.TrimSpace(match[1])
		subName := strings.TrimSpace(match[2])
		chapterTitle = strings.TrimSpace(subName + " " + name)
	}

	// B. Fallback: DOM h1 + next sibling div (e.g. h1="京兼家の花嫁 第一話", div="黒朗と布由 - ①")
	if chapterTitle == "" && doc != nil {
		h1 := doc.Find("h1").First()
		if h1.Length() > 0 {
			h1Text := strings.TrimSpace(h1.Text())
			seriesName := strings.TrimSpace(doc.Find("h2").First().Text())
			if seriesName != "" && strings.HasPrefix(h1Text, seriesName) {
				h1Text = strings.TrimSpace(strings.TrimPrefix(h1Text, seriesName))
			}
			subDiv := strings.TrimSpace(h1.Next().Text())
			chapterTitle = strings.TrimSpace(h1Text + " " + subDiv)
		}
	}

	// C. Fallback: BreadcrumbList or meta tags
	if chapterTitle == "" && doc != nil {
		doc.Find("script[type='application/ld+json']").Each(func(_ int, s *goquery.Selection) {
			var data map[string]interface{}
			if err := json.Unmarshal([]byte(s.Text()), &data); err == nil {
				if data["@type"] == "BreadcrumbList" {
					if items, ok := data["itemListElement"].([]interface{}); ok && len(items) > 0 {
						if last, ok := items[len(items)-1].(map[string]interface{}); ok {
							if name, ok := last["name"].(string); ok && strings.TrimSpace(name) != "" {
								chapterTitle = strings.TrimSpace(name)
							}
						}
					}
				}
			}
		})
	}
	chapterTitle = utils.SanitizeFolderName(chapterTitle)

	// 2. Extract image URLs from unescaped HTML (preserves ?hash=...&expires=...)
	var images []string
	seen := make(map[string]bool)

	matches := mangaUpJapanImageRegex.FindAllString(unescapedHTML, -1)
	for _, cleanURL := range matches {
		if !seen[cleanURL] {
			seen[cleanURL] = true
			images = append(images, cleanURL)
		}
	}

	// Fallback: inspect DOM <img> tags
	if len(images) == 0 && doc != nil {
		doc.Find("img").Each(func(_ int, s *goquery.Selection) {
			src := s.AttrOr("src", s.AttrOr("data-src", ""))
			if strings.Contains(src, "ja-img.manga-up.com") && strings.Contains(src, "/manga/") {
				clean := strings.ReplaceAll(src, "&amp;", "&")
				if !seen[clean] {
					seen[clean] = true
					images = append(images, clean)
				}
			}
		})
	}

	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: images,
	}, nil
}

func init() {
	Register(NewMangaUpJapan())
}
