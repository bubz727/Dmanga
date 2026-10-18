package parsers

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"sort"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
	"manga-downloader/themes"
)

var (
	// Regex astro props : "url": [0, "https://..."]
	asuraAstroRegex = regexp.MustCompile(`"url":\s*\[0,\s*"([^"]+)"\]`)

	// Regex Next.js streaming chunks: self.__next_f.push([1,"..."])
	asuraNextChunkRegex = regexp.MustCompile(`self\.__next_f\.push\(\s*(\[[^)]*\])\s*\)`)
)

type AsuraScans struct{}

func NewAsuraScans() *AsuraScans {
	return &AsuraScans{}
}

func (a *AsuraScans) Name() string {
	return "Asura Scans"
}

func (a *AsuraScans) Domains() []string {
	return []string{
		"asurascans.com",
	}
}

func (a *AsuraScans) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	doc, rawHTML, err := themes.FetchHTML(chapterURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
	})
	if err != nil {
		return nil, err
	}

	// 1. APPROACH 1: Astro Island Component Props
	var astroUrls []string
	seen := make(map[string]bool)

	doc.Find("astro-island[component-url*='ChapterReader']").Each(func(i int, s *goquery.Selection) {
		props, exists := s.Attr("props")
		if exists && props != "" {
			cleanProps := strings.ReplaceAll(props, "&quot;", "\"")
			matches := asuraAstroRegex.FindAllStringSubmatch(cleanProps, -1)
			for _, m := range matches {
				if len(m) > 1 {
					cleanURL := themes.CleanImageURL(m[1], baseHost)
					if cleanURL != "" && !seen[cleanURL] {
						seen[cleanURL] = true
						astroUrls = append(astroUrls, cleanURL)
					}
				}
			}
		}
	})

	if len(astroUrls) > 0 {
		return astroUrls, nil
	}

	// 2. APPROACH 2: Next.js Streaming Chunks
	nextMatches := asuraNextChunkRegex.FindAllStringSubmatch(rawHTML, -1)
	if len(nextMatches) > 0 {
		var combinedSB strings.Builder
		for _, m := range nextMatches {
			if len(m) > 1 {
				var arr []interface{}
				if err := json.Unmarshal([]byte(m[1]), &arr); err == nil {
					for _, item := range arr {
						if str, ok := item.(string); ok {
							combinedSB.WriteString(str)
						}
					}
				}
			}
		}

		fullStr := combinedSB.String()
		lines := strings.Split(fullStr, "\n")

		type PageItem struct {
			Order int
			URL   string
		}
		var pages []PageItem

		for _, line := range lines {
			idx := strings.Index(line, ":")
			if idx == -1 {
				continue
			}
			jsonPart := strings.TrimSpace(line[idx+1:])
			var obj map[string]interface{}
			if err := json.Unmarshal([]byte(jsonPart), &obj); err == nil {
				orderVal, hasOrder := obj["order"]
				urlVal, hasURL := obj["url"]
				if hasOrder && hasURL {
					if uStr, ok := urlVal.(string); ok && uStr != "" {
						orderInt := 0
						if oFloat, ok := orderVal.(float64); ok {
							orderInt = int(oFloat)
						}
						pages = append(pages, PageItem{Order: orderInt, URL: themes.CleanImageURL(uStr, baseHost)})
					}
				}
			}
		}

		if len(pages) > 0 {
			sort.Slice(pages, func(i, j int) bool {
				return pages[i].Order < pages[j].Order
			})
			var sortedUrls []string
			for _, p := range pages {
				if !seen[p.URL] {
					seen[p.URL] = true
					sortedUrls = append(sortedUrls, p.URL)
				}
			}
			if len(sortedUrls) > 0 {
				return sortedUrls, nil
			}
		}
	}

	// 3. APPROACH 3: Fallback directly to DOM <img>
	var fallbackImages []string
	doc.Find("div.w-full.mx-auto.center img, div#readerarea img, div.reading-content img, article img").Each(func(i int, s *goquery.Selection) {
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
			if !strings.Contains(lower, "logo") && !strings.Contains(lower, "banner") {
				seen[imgURL] = true
				fallbackImages = append(fallbackImages, imgURL)
			}
		}
	})

	return core.EnsureImages(fallbackImages)
}

func init() {
	Register(NewAsuraScans())
}
