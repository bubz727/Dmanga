package themes

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/PuerkitoBio/goquery"
)

// Natsu adalah base theme parser untuk tema WordPress NatsuId (diporting dari Kotatsu natsu.kt)
type Natsu struct {
	PageSelector string
}

// NewNatsu menginisialisasi konfigurasi default tema Natsu
func NewNatsu() Natsu {
	return Natsu{
		PageSelector: "main section section > img, main section img, div.reader-area img",
	}
}

// ExtractImages mengekstrak semua gambar chapter berdasarkan logika natsu.kt
func (n *Natsu) ExtractImages(chapterURL string) ([]string, error) {
	parsedURL, err := url.Parse(chapterURL)
	if err != nil {
		return nil, err
	}
	baseHost := parsedURL.Host

	headers := map[string]string{
		"Referer": fmt.Sprintf("https://%s/", baseHost),
		"Origin":  fmt.Sprintf("https://%s", baseHost),
	}

	doc, _, err := FetchHTML(chapterURL, headers)
	if err != nil {
		return nil, err
	}

	var images []string
	seen := make(map[string]bool)

	// Kotatsu selector: main section section > img
	doc.Find(n.PageSelector).Each(func(i int, s *goquery.Selection) {
		imgURL := ""
		for _, attr := range []string{"data-src", "data-lazy-src", "src"} {
			if val, exists := s.Attr(attr); exists && strings.TrimSpace(val) != "" {
				imgURL = val
				break
			}
		}

		imgURL = CleanImageURL(imgURL, baseHost)
		if imgURL != "" && !seen[imgURL] {
			lower := strings.ToLower(imgURL)
			if !strings.Contains(lower, "avatar") && !strings.Contains(lower, "logo") && !strings.Contains(lower, "banner") {
				seen[imgURL] = true
				images = append(images, imgURL)
			}
		}
	})

	if len(images) == 0 {
		return nil, fmt.Errorf("tidak ada gambar yang ditemukan untuk tema Natsu")
	}

	return images, nil
}
