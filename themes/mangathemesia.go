package themes

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

var (
	// Regex untuk mendeteksi array JSON gambar di dalam tag <script>
	// Porting langsung dari Kotatsu: Regex(""""images"\s*:\s*(\[.*?])""", RegexOption.DOT_MATCHES_ALL)
	jsonImagesRegex = regexp.MustCompile(`"images"\s*:\s*(\[[^\]]*\])`)

	// Daftar prioritas atribut lazy load dari Kotatsu:
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

// MangaThemesia adalah base struct untuk semua website berbasis tema MangaThemesia
type MangaThemesia struct {
	PageSelector  string
	CustomHeaders map[string]string
}

// NewMangaThemesia menginisialisasi base theme MangaThemesia dengan konfigurasi default Kotatsu
func NewMangaThemesia() MangaThemesia {
	return MangaThemesia{
		PageSelector:  "div#readerarea img",
		CustomHeaders: make(map[string]string),
	}
}

// ExtractImages mengimplementasikan logika ekstraksi gambar dari Kotatsu mangathemesia.kt
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

	// 1. CARA PERTAMA: Cek apakah ada data JSON gambar di dalam tag <script>
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

	// 2. CARA KEDUA: Fallback ke elemen DOM <img> langsung
	var images []string
	seen := make(map[string]bool)

	doc.Find(m.PageSelector).Each(func(i int, s *goquery.Selection) {
		imgURL := ""

		// Cek atribut secara berurutan sesuai prioritas Kotatsu
		for _, attr := range lazyAttributes {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		// Fallback ke srcset jika atribut standar kosong
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

		// Filter out gambar kosong, avatar, atau logo kecil
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	if len(images) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan dengan selector '%s'", m.PageSelector)
	}

	return images, nil
}
