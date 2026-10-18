package parsers

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
	"manga-downloader/themes"
)

var toonzFilenameRegex = regexp.MustCompile(`"filename"\s*:\s*"([^"]+)"`)

// Toonz parser for toonz.to and toonx.to (ported from toonz.kt)
type Toonz struct{}

func NewToonz() *Toonz {
	return &Toonz{}
}

func (t *Toonz) Name() string {
	return "Toonz"
}

func (t *Toonz) Domains() []string {
	return []string{
		"toonz.to",
		"toonx.to",
	}
}

func (t *Toonz) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
		"Cookie":  "reader_prefs=%7B%22rating%22%3A%22pornographic%22%7D",
	}

	doc, rawHTML, err := themes.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	// 1. Next.js JSON extraction (filename in streaming chunks / RSC payload)
	matches := toonzFilenameRegex.FindAllStringSubmatch(rawHTML, -1)
	for _, m := range matches {
		if len(m) > 1 {
			fn := strings.TrimPrefix(m[1], "/")
			fn = strings.ReplaceAll(fn, `\/`, "/")
			lower := strings.ToLower(fn)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "favicon") && !strings.Contains(lower, "cover") {
				var imgURL string
				if strings.HasPrefix(fn, "http://") || strings.HasPrefix(fn, "https://") {
					imgURL = fn
				} else if strings.HasPrefix(fn, "uploads/") {
					imgURL = fmt.Sprintf("https://%s/%s", baseHost, fn)
				} else {
					imgURL = fmt.Sprintf("https://%s/uploads/%s", baseHost, fn)
				}
				if !seen[imgURL] {
					seen[imgURL] = true
					images = append(images, imgURL)
				}
			}
		}
	}

	// 2. If no filename matches found in initial HTML, try requesting with rsc: 1
	if len(images) == 0 {
		rscHeaders := map[string]string{
			"Referer": fmt.Sprintf("https://%s/", baseHost),
			"Cookie":  "reader_prefs=%7B%22rating%22%3A%22pornographic%22%7D",
			"rsc":     "1",
		}
		_, rscBody, rscErr := themes.FetchHTML(chapterURL, rscHeaders)
		if rscErr == nil {
			rscMatches := toonzFilenameRegex.FindAllStringSubmatch(rscBody, -1)
			for _, m := range rscMatches {
				if len(m) > 1 {
					fn := strings.TrimPrefix(m[1], "/")
					fn = strings.ReplaceAll(fn, `\/`, "/")
					lower := strings.ToLower(fn)
					if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "favicon") && !strings.Contains(lower, "cover") {
						var imgURL string
						if strings.HasPrefix(fn, "http://") || strings.HasPrefix(fn, "https://") {
							imgURL = fn
						} else if strings.HasPrefix(fn, "uploads/") {
							imgURL = fmt.Sprintf("https://%s/%s", baseHost, fn)
						} else {
							imgURL = fmt.Sprintf("https://%s/uploads/%s", baseHost, fn)
						}
						if !seen[imgURL] {
							seen[imgURL] = true
							images = append(images, imgURL)
						}
					}
				}
			}
		}
	}

	if len(images) > 0 {
		return core.EnsureImages(images)
	}

	// 3. DOM fallback: inspect img tags inside reader/prose/main containers
	doc.Find("img[src*='/uploads/'], img[data-src*='/uploads/'], div.reader img, main img, div.prose img").Each(func(i int, s *goquery.Selection) {
		imgURL := ""
		for _, attr := range []string{"data-src", "src"} {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		imgURL = themes.CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "cover") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	return core.EnsureImages(images)
}

func init() {
	Register(NewToonz())
}
