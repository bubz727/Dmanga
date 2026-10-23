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

type NaverComic struct{}

func NewNaverComic() *NaverComic {
	return &NaverComic{}
}

func (n *NaverComic) Name() string {
	return "Naver Comic"
}

func (n *NaverComic) Domains() []string {
	return []string{
		"comic.naver.com",
		"m.comic.naver.com",
	}
}

func (n *NaverComic) getChapterInternal(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
	}

	doc, _, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	doc.Find(".wt_viewer img, .toon_view_lst img.toon_image, div#comic_view_area img").Each(func(i int, s *goquery.Selection) {
		imgURL := ""
		for _, attr := range []string{"data-src", "src"} {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		imgURL = utils.CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			seen[imgURL] = true
			images = append(images, imgURL)
		}
	})

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	text := strings.TrimSpace(doc.Find("#subTitle_toolbar, .episode").First().Text())
	if num := utils.ExtractChapterNumber(text); num != "" {
		chapterTitle = fmt.Sprintf("Ch %s", num)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (n *NaverComic) GetImages(chapterURL string) ([]string, error) {
	chap, err := n.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

// GetChapter extracts both the chapter title and image URLs
func (n *NaverComic) GetChapter(chapterURL string) (*core.Chapter, error) {
	return n.getChapterInternal(chapterURL)
}

func init() {
	Register(NewNaverComic())
}
