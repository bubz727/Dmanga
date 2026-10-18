package themes

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
)

var (
	// Regex to detect JSON image arrays inside <script> tags
	jsonImagesRegex = regexp.MustCompile(`"images"\s*:\s*(\[[^\]]*\])`)

	// listOf("data-src", "data-lazy-src", "data-original", "data-cfsrc", "data-image", "src")
	lazyAttributes = []string{
		"data-src",
		"data-lazy-src",
		"data-original",
		"data-cfsrc",
		"data-image",
		"src",
	}
)

// MangaThemesia is the base struct for websites running MangaThemesia theme
type MangaThemesia struct {
	PageSelector  string
	CustomHeaders map[string]string
}

func NewMangaThemesia() MangaThemesia {
	return MangaThemesia{
		PageSelector:  "div#readerarea img",
		CustomHeaders: make(map[string]string),
	}
}

func (m *MangaThemesia) ExtractImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	doc, rawHTML, err := FetchHTML(chapterURL, m.CustomHeaders)
	if err != nil {
		return nil, err
	}

	// 1. FIRST APPROACH: Check for JSON image array in <script> tags
	if matches := jsonImagesRegex.FindStringSubmatch(rawHTML); len(matches) > 1 {
		var jsonImages []string
		if err := json.Unmarshal([]byte(matches[1]), &jsonImages); err == nil && len(jsonImages) > 0 {
			var cleaned []string
			for _, img := range jsonImages {
				if finalURL := CleanImageURL(img, baseHost); finalURL != "" {
					cleaned = append(cleaned, finalURL)
				}
			}
			if len(cleaned) > 0 {
				return cleaned, nil
			}
		}
	}

	// 2. SECOND APPROACH: Fallback directly to DOM <img> elements
	var images []string
	seen := make(map[string]bool)

	doc.Find(m.PageSelector).Each(func(i int, s *goquery.Selection) {
		imgURL := ""

		for _, attr := range lazyAttributes {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		// Fallback to srcset if standard attributes are empty
		if imgURL == "" {
			if srcset, exists := s.Attr("srcset"); exists && strings.TrimSpace(srcset) != "" {
				parts := strings.Split(srcset, ",")
				lastPart := strings.TrimSpace(parts[len(parts)-1])
				tokens := strings.Fields(lastPart)
				if len(tokens) > 0 {
					imgURL = tokens[0]
				}
			}
		}

		imgURL = CleanImageURL(imgURL, baseHost)

		// Filter out empty images, avatars, or logos
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	return core.EnsureImages(images)
}
