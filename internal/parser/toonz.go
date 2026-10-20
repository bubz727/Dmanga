package parser

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

var (
	toonzFilenameRegex = regexp.MustCompile(`"filename"\s*:\s*"([^"]+)"`)
	toonzChapterRegex  = regexp.MustCompile(`"chapterNumber"\s*:\s*"([0-9]+(?:\.[0-9]+)?)"`)
)

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

// GetChapter extracts both the chapter title and image URLs
func (t *Toonz) GetChapter(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}
	baseHost := parsedURL.Host

	// 1. Primary method: Next.js RSC payload stream (official method used by Tachiyomi)
	rscHeaders := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
		"rsc":     "1",
		"Accept":  "*/*",
	}

	var images []string
	seen := make(map[string]bool)
	chapterTitle := ""

	_, rscBody, err := fetcher.FetchHTML(chapterURL, rscHeaders)
	if err == nil && len(rscBody) > 0 {
		// Extract chapter number from RSC metadata
		if chMatch := toonzChapterRegex.FindStringSubmatch(rscBody); len(chMatch) > 1 {
			chNum := strings.TrimSuffix(chMatch[1], ".00")
			chapterTitle = fmt.Sprintf("Ch %s", chNum)
		}

		matches := toonzFilenameRegex.FindAllStringSubmatch(rscBody, -1)
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
	}

	if len(images) > 0 {
		validImgs, err := core.EnsureImages(images)
		if err != nil {
			return nil, err
		}
		return &core.Chapter{
			Title:  chapterTitle,
			Images: validImgs,
		}, nil
	}

	// 2. Fallback: normal HTML request
	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
	}
	doc, rawHTML, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	// Extract chapter number from rawHTML metadata if not yet extracted
	if chapterTitle == "" {
		if chMatch := toonzChapterRegex.FindStringSubmatch(rawHTML); len(chMatch) > 1 {
			chNum := strings.TrimSuffix(chMatch[1], ".00")
			chapterTitle = fmt.Sprintf("Ch %s", chNum)
		}
	}

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

	if len(images) > 0 {
		validImgs, err := core.EnsureImages(images)
		if err != nil {
			return nil, err
		}
		return &core.Chapter{
			Title:  chapterTitle,
			Images: validImgs,
		}, nil
	}

	// 3. Fallback: DOM img tags
	doc.Find("img[src*='/uploads/'], img[data-src*='/uploads/'], div.reader img, main img, div.prose img").Each(func(i int, s *goquery.Selection) {
		imgURL := ""
		for _, attr := range []string{"data-src", "src"} {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		imgURL = utils.CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "cover") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}
	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (t *Toonz) GetImages(chapterURL string) ([]string, error) {
	chap, err := t.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

func init() {
	Register(NewToonz())
}
