package parser

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
)

var (
	bzcdnRegex      = regexp.MustCompile(`^https?://(?:[\w-]+)\.bzcdn\.net/(.+)$`)
	baozimhTitleCut = regexp.MustCompile(`\s*[\(（]\d+/\d+[\)）]\s*`)
)

type Baozimh struct{}

func NewBaozimh() *Baozimh {
	return &Baozimh{}
}

func (b *Baozimh) Name() string {
	return "Baozimh"
}

func (b *Baozimh) Domains() []string {
	return []string{
		"baozimh.com",
		"www.baozimh.com",
		"cn.baozimh.com",
		"tw.baozimh.com",
		"baozimh.org",
		"www.baozimh.org",
		"baozimhw.com",
		"www.baozimhw.com",
		"twmanga.com",
		"www.twmanga.com",
		"webmota.com",
		"www.webmota.com",
	}
}

func (b *Baozimh) GetImages(chapterURL string) ([]string, error) {
	chap, err := b.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func (b *Baozimh) GetChapter(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	doc, _, err := fetcher.FetchHTML(chapterURL, nil)
	if err != nil {
		return nil, err
	}

	// 1. Extract chapter title
	rawTitle := ""
	titleSel := doc.Find(".l-content .text .title, .text span.title, .comic-contain .title, span.title, h1.title, .comic-chapter .title").First()
	if titleSel.Length() > 0 {
		rawTitle = strings.TrimSpace(titleSel.Text())
	}
	if rawTitle == "" {
		rawTitle = strings.TrimSpace(doc.Find("head title").Text())
		if idx := strings.Index(rawTitle, "-"); idx != -1 {
			rawTitle = strings.TrimSpace(rawTitle[:idx])
		}
	}
	// Clean pagination suffix like (1/4)
	cleanTitle := baozimhTitleCut.ReplaceAllString(rawTitle, "")
	cleanTitle = strings.TrimSpace(cleanTitle)

	// 2. Extract images
	var images []string
	seen := make(map[string]bool)

	extractImages := func(d *goquery.Document) {
		d.Find(".comic-contain amp-img, .comic-contain img").Each(func(_ int, s *goquery.Selection) {
			src, exists := s.Attr("data-src")
			if !exists || strings.TrimSpace(src) == "" {
				src, exists = s.Attr("data-original")
			}
			if !exists || strings.TrimSpace(src) == "" {
				src, exists = s.Attr("src")
			}
			src = strings.TrimSpace(src)
			if src == "" {
				return
			}

			// Replace CDN if needed
			if bzcdnRegex.MatchString(src) {
				src = bzcdnRegex.ReplaceAllString(src, "https://static-tw.baozimh.com/$1")
			}

			if !seen[src] {
				seen[src] = true
				images = append(images, src)
			}
		})
	}

	extractImages(doc)

	// 3. Follow chapter multi-subpage pagination if chapter is split (e.g. 0_542.html -> 0_542_2.html -> 0_542_3.html)
	currentDoc := doc
	maxSubPages := 15

	for pageIdx := 2; pageIdx <= maxSubPages; pageIdx++ {
		nextHref := ""
		currentDoc.Find("div.comic-chapter div.next_chapter a, a.next_chapter, div.next_chapter a, a#next-chapter").Each(func(_ int, s *goquery.Selection) {
			href, exists := s.Attr("href")
			if exists && strings.TrimSpace(href) != "" {
				nextHref = strings.TrimSpace(href)
			}
		})

		if nextHref == "" {
			break
		}

		nextURL, err := parsedURL.Parse(nextHref)
		if err != nil {
			break
		}

		expectedSuffix := fmt.Sprintf("_%d.html", pageIdx)
		if !strings.HasSuffix(strings.ToLower(nextURL.Path), expectedSuffix) && !strings.Contains(strings.ToLower(nextURL.Path), fmt.Sprintf("_%d", pageIdx)) {
			break
		}

		nextDoc, _, err := fetcher.FetchHTML(nextURL.String(), nil)
		if err != nil {
			break
		}

		extractImages(nextDoc)
		currentDoc = nextDoc
	}

	if len(images) == 0 {
		return nil, core.ErrNoImagesFound
	}

	return &core.Chapter{
		Title:  cleanTitle,
		Images: images,
	}, nil
}

func init() {
	Register(NewBaozimh())
}
