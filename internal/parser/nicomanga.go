package parser

import (
	"encoding/json"
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

func (n *NicoManga) getChapterInternal(chapterURL string) (*core.Chapter, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}

	headers := map[string]string{
		"Referer": fmtReferer(parsedURL),
		"Origin":  fmtOrigin(parsedURL),
	}

	doc, rawHTML, err := fetcher.FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string

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
						if cleanURL := utils.CleanImageURL(img, parsedURL.Host); cleanURL != "" {
							cleaned = append(cleaned, cleanURL)
						}
					}
					if len(cleaned) > 0 {
						images = cleaned
						break
					}
				}
			}
		}
	}

	// 2. FALLBACK STRATEGY: Extract from DOM elements if server renders static markup
	if len(images) == 0 {
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

				imgURL = utils.CleanImageURL(imgURL, parsedURL.Host)
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
	}

	validImgs, err := core.EnsureImages(images)
	if err != nil {
		return nil, err
	}

	chapterTitle := ""
	text := strings.TrimSpace(doc.Find(".breadcrumb-navigation [class*='breadcrumb-item'], [class*='breadcrumb-item']").Last().Text())
	if num := utils.ExtractChapterNumber(text); num != "" {
		chapterTitle = fmt.Sprintf("Ch %s", num)
	}

	return &core.Chapter{
		Title:  chapterTitle,
		Images: validImgs,
	}, nil
}

// GetImages implements core.Parser
func (n *NicoManga) GetImages(chapterURL string) ([]string, error) {
	chap, err := n.GetChapter(chapterURL)
	if err != nil {
		return nil, err
	}
	return chap.Images, nil
}

// GetChapter extracts both the chapter title and image URLs
func (n *NicoManga) GetChapter(chapterURL string) (*core.Chapter, error) {
	return n.getChapterInternal(chapterURL)
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
	Register(NewNicoManga())
}
