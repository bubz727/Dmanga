package parsers

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"manga-downloader/core"
)

type GigaViewerParser struct{}

func NewGigaViewerParser() core.Parser {
	return &GigaViewerParser{}
}

func init() {
	core.Register(NewGigaViewerParser())
}

func (p *GigaViewerParser) Name() string {
	return "GigaViewer"
}

func (p *GigaViewerParser) Domains() []string {
	return []string{
		"comic-days.com",
		"ichicomi.com",
		"shonenjumpplus.com",
		"tonarinoyj.jp",
		"andsofa.com",
		"morningtwo.com",
		"getsumagakichi.com",
		"bibliosirius.com",
		"kuragebunch.com",
		"comicbunch-kai.com",
		"viewer.heros-web.com",
		"comicborder.com",
		"comic-gardo.com",
		"comic-zenon.com",
		"magcomi.com",
		"comic-action.com",
		"comic-trail.com",
		"feelweb.jp",
		"sunday-webry.com",
		"www.sunday-webry.com",
		"comic-ogyaaa.com",
		"comic-earthstar.com",
		"ourfeel.jp",
		"comic-seasons.com",
		"comic-y-ours.com",
	}
}

type gigaManifest struct {
	ReadableProduct struct {
		Title         string `json:"title"`
		PageStructure struct {
			Pages []struct {
				Type string `json:"type"`
				Src  string `json:"src"`
			} `json:"pages"`
		} `json:"pageStructure"`
	} `json:"readableProduct"`
}

// GetChapter extracts both the authentic Japanese chapter title and image URLs
func (p *GigaViewerParser) GetChapter(chapterURL string) (*core.Chapter, error) {
	parsed, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", core.ErrInvalidURL, err)
	}

	doc, rawHTML, err := core.FetchHTML(chapterURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsed.Host),
	})
	if err != nil {
		return nil, fmt.Errorf("failed to fetch HTML: %w", err)
	}

	var jsonStr string

	// 1. Check data-value attribute of #episode-json tag
	if val, exists := doc.Find("#episode-json").Attr("data-value"); exists && strings.TrimSpace(val) != "" {
		jsonStr = val
	}

	// 2. Check inner text of #episode-json tag
	if jsonStr == "" {
		textVal := strings.TrimSpace(doc.Find("#episode-json").Text())
		if textVal != "" && strings.HasPrefix(textVal, "{") {
			jsonStr = textVal
		}
	}

	// 3. Fallback regex
	if jsonStr == "" {
		reDataVal := regexp.MustCompile(`id=['"]episode-json['"][^>]*data-value=['"]([^'"]+)['"]`)
		if m := reDataVal.FindStringSubmatch(rawHTML); len(m) > 1 {
			jsonStr = html.UnescapeString(m[1])
		} else {
			reInner := regexp.MustCompile(`<script id=['"]episode-json['"][^>]*>([\s\S]*?)</script>`)
			if m2 := reInner.FindStringSubmatch(rawHTML); len(m2) > 1 {
				jsonStr = strings.TrimSpace(m2[1])
			}
		}
	}

	if jsonStr == "" {
		return nil, core.ErrManifestNotFound
	}

	var manifest gigaManifest
	if err := json.Unmarshal([]byte(jsonStr), &manifest); err != nil {
		return nil, core.JSONParseError("episode", err)
	}

	isSJ := strings.Contains(parsed.Host, "shonenjumpplus.com")
	fragment := "#giga=true"
	if isSJ {
		fragment = "#sj=true"
	}

	var imageURLs []string
	for _, page := range manifest.ReadableProduct.PageStructure.Pages {
		if page.Type == "main" && page.Src != "" {
			imageURLs = append(imageURLs, page.Src+fragment)
		}
	}

	validImgs, err := core.EnsureImages(imageURLs)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	if manifest.ReadableProduct.Title != "" {
		chapterTitle = core.SanitizeFolderName(manifest.ReadableProduct.Title)
	} else {
		chapterTitle = core.SanitizeFolderName(doc.Find("head title").Text())
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (p *GigaViewerParser) GetImages(chapterURL string) ([]string, error) {
	chap, err := p.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

// ProcessImage descrambles GigaViewer puzzle image tiles before saving to file
func (p *GigaViewerParser) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	return core.ProcessImageDescramble(rawBytes, imageURL)
}
