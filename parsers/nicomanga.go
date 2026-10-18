package parsers

import (
	"encoding/json"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
)

var (
	nextFRegex = regexp.MustCompile(`self\.__next_f\.push\(\[1,\s*"([\s\S]*?)"\]\)`)
)

type NicoManga struct{}

func NewNicoManga() *NicoManga {
	return &NicoManga{}
}

func (n *NicoManga) Name() string {
	return "NicoManga"
}

func (n *NicoManga) Domains() []string {
	return []string{
		"nicomanga.com",
		"www.nicomanga.com",
		"nicomanga.net",
		"www.nicomanga.net",
	}
}

func (n *NicoManga) GetImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Referer": fmtReferer(parsedURL),
		"Origin":  fmtOrigin(parsedURL),
	}

	doc, rawHTML, err := core.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	// 1. PRIMARY STRATEGY: Decode Next.js chaotic_payload stream
	if matches := nextFRegex.FindAllStringSubmatch(rawHTML, -1); len(matches) > 0 {
		for _, m := range matches {
			raw := m[1]
			runes := []rune(raw)
			if len(runes) < 20 {
				continue
			}

			// Check for CJK character stream (runes between 0x4E00 and 0x9FFF)
			cjkCount := 0
			sampleLen := 100
			if len(runes) < sampleLen {
				sampleLen = len(runes)
			}
			for i := 0; i < sampleLen; i++ {
				if runes[i] >= 19968 && runes[i] <= 40959 {
					cjkCount++
				}
			}

			if cjkCount > 10 {
				clean := strings.ReplaceAll(raw, `\"`, `"`)
				clean = strings.ReplaceAll(clean, `\\`, `\`)

				buf := decodeChaoticPayload(clean, "NicoMangaX2")
				var payload struct {
					Images []string `json:"images"`
				}

				if err := json.Unmarshal(buf, &payload); err == nil && len(payload.Images) > 0 {
					var cleaned []string
					for _, img := range payload.Images {
						if cleanURL := core.CleanImageURL(img, parsedURL.Host); cleanURL != "" {
							cleaned = append(cleaned, cleanURL)
						}
					}
					if len(cleaned) > 0 {
						return core.EnsureImages(cleaned)
					}
				}
			}
		}
	}

	// 2. FALLBACK STRATEGY: Extract from DOM elements if server renders static markup
	var images []string
	seen := make(map[string]bool)

	selectors := []string{
		"#chapter_images_container img",
		"img.chapter-img",
		"div.chapter-image-wrapper img",
		"div.chapter-images img",
	}

	for _, sel := range selectors {
		doc.Find(sel).Each(func(i int, s *goquery.Selection) {
			imgURL := ""
			for _, attr := range []string{"data-src", "data-original", "data-lazy-src", "src"} {
				if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
					imgURL = val
					break
				}
			}

			imgURL = core.CleanImageURL(imgURL, parsedURL.Host)
			if imgURL != "" && !seen[imgURL] {
				lower := strings.ToLower(imgURL)
				if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "thumb") {
					seen[imgURL] = true
					images = append(images, imgURL)
				}
			}
		})

		if len(images) > 0 {
			break
		}
	}

	return core.EnsureImages(images)
}

// decodeChaoticPayload decodes NicoManga's Unicode codePoint XOR cipher
func decodeChaoticPayload(encoded, key string) []byte {
	if key == "" {
		key = "NicoMangaX2"
	}
	keyRunes := []rune(key)
	keyLen := len(keyRunes)

	runes := []rune(encoded)
	buf := make([]byte, len(runes))

	for n, r := range runes {
		codePoint := int(r) - 19968
		xorKey := int(keyRunes[n%keyLen])
		buf[n] = byte(codePoint ^ xorKey)
	}

	return buf
}

func fmtReferer(u *url.URL) string {
	if u.Host != "" {
		return "https://" + u.Host + "/"
	}
	return "https://nicomanga.com/"
}

func fmtOrigin(u *url.URL) string {
	if u.Host != "" {
		return "https://" + u.Host
	}
	return "https://nicomanga.com"
}

func init() {
	core.Register(NewNicoManga())
}
