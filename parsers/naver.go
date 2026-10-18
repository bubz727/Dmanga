package parsers

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
	"manga-downloader/themes"
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

func (n *NaverComic) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
	}

	doc, _, err := themes.FetchHTML(chapterURL, headers)
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

		imgURL = themes.CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			seen[imgURL] = true
			images = append(images, imgURL)
		}
	})

	return core.EnsureImages(images)
}

func init() {
	Register(NewNaverComic())
}
