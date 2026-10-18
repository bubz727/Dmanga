package parsers

import (
	"encoding/json"
	"fmt"
	"html"
	"net/url"
	"regexp"
	"strings"

	"manga-downloader/themes"
)

type GigaViewerParser struct{}

func NewGigaViewerParser() themes.Parser {
	return &GigaViewerParser{}
}

func init() {
	Register(NewGigaViewerParser())
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

func (p *GigaViewerParser) GetImages(chapterURL string) ([]string, error) {
	parsed, err := url.Parse(chapterURL)
	if err != nil {
		return nil, fmt.Errorf("URL tidak valid: %w", err)
	}

	doc, rawHTML, err := themes.FetchHTML(chapterURL, map[string]string{
		"Referer": fmt.Sprintf("https://%s/", parsed.Host),
	})
	if err != nil {
		return nil, fmt.Errorf("gagal mengambil HTML: %w", err)
	}

	var jsonStr string

	// 1. Cek dari atribut data-value tag #episode-json
	if val, exists := doc.Find("#episode-json").Attr("data-value"); exists && strings.TrimSpace(val) != "" {
		jsonStr = val
	}

	// 2. Cek dari inner text tag #episode-json
	if jsonStr == "" {
		textVal := strings.TrimSpace(doc.Find("#episode-json").Text())
		if textVal != "" && strings.HasPrefix(textVal, "{") {
			jsonStr = textVal
		}
	}

	// 3. Regex fallback
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
		return nil, fmt.Errorf("tidak dapat menemukan data manifest episode-json pada halaman")
	}

	var manifest gigaManifest
	if err := json.Unmarshal([]byte(jsonStr), &manifest); err != nil {
		return nil, fmt.Errorf("gagal parse JSON episode: %w", err)
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

	if len(imageURLs) == 0 {
		return nil, fmt.Errorf("tidak ditemukan halaman gambar utama (type: main)")
	}

	return imageURLs, nil
}

// ProcessImage mendescramble potongan puzzle GigaViewer sebelum disimpan ke file
func (p *GigaViewerParser) ProcessImage(rawBytes []byte, imageURL string) ([]byte, error) {
	return themes.ProcessImageDescramble(rawBytes, imageURL)
}
