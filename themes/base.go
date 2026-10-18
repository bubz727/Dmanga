package themes

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// Parser adalah interface utama yang diimplementasikan oleh setiap parser website
type Parser interface {
	Name() string
	Domains() []string
	GetImages(chapterURL string) ([]string, error)
}

// ImageProcessor mengizinkan parser untuk memproses atau mendescramble bytes gambar sebelum disimpan ke file
type ImageProcessor interface {
	ProcessImage(rawBytes []byte, imageURL string) ([]byte, error)
}

// FetchHTML mendownload halaman dan mengembalikan goquery Document serta string HTML mentahnya
func FetchHTML(pageURL string, customHeaders map[string]string) (*goquery.Document, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, "", err
	}

	// Default modern browser headers
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/webp,*/*;q=0.8")

	parsed, err := url.Parse(pageURL)
	if err == nil && parsed.Host != "" {
		req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))
	}

	for k, v := range customHeaders {
		req.Header.Set(k, v)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	rawHTML := string(bodyBytes)
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, "", err
	}

	return doc, rawHTML, nil
}

// CleanImageURL membersihkan URL relatif, protokol '//', dan query resize WordPress (?w=...)
func CleanImageURL(rawURL, baseHost string) string {
	val := strings.TrimSpace(rawURL)
	if val == "" {
		return ""
	}

	// Hapus query parameter resize WordPress (e.g. ?w=640&quality=80)
	if idx := strings.Index(val, "?"); idx != -1 {
		val = val[:idx]
	}

	// Handle protocol-relative URL
	if strings.HasPrefix(val, "//") {
		return "https:" + val
	}

	// Handle relative path (/wp-content/...)
	if strings.HasPrefix(val, "/") && baseHost != "" {
		return fmt.Sprintf("https://%s%s", baseHost, val)
	}

	return val
}

// GetFileExtension mendeteksi ekstensi gambar (.jpg, .png, .webp)
func GetFileExtension(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		ext := filepath.Ext(u.Path)
		if ext != "" && len(ext) <= 5 {
			return strings.ToLower(ext)
		}
	}
	return ".jpg"
}
