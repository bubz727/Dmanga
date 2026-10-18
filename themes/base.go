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
	"manga-downloader/core"
)

// Parser is the primary interface implemented by every website parser
type Parser interface {
	Name() string
	Domains() []string
	GetImages(chapterURL string) ([]string, error)
}

// ImageProcessor allows a parser to process or descramble raw image bytes before saving them to disk
type ImageProcessor interface {
	ProcessImage(rawBytes []byte, imageURL string) ([]byte, error)
}

// FetchHTML downloads a webpage and returns both the goquery Document and raw HTML string
func FetchHTML(pageURL string, customHeaders map[string]string) (*goquery.Document, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	req, err := http.NewRequest("GET", pageURL, nil)
	if err != nil {
		return nil, "", err
	}

	parsed, err := url.Parse(pageURL)
	if err != nil {
		return nil, "", err
	}

	// Modern browser headers with Chrome client hints
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,image/avif,image/webp,*/*;q=0.8")
	req.Header.Set("Accept-Language", "en-US,en;q=0.9")
	req.Header.Set("Sec-Ch-Ua", `"Chromium";v="124", "Google Chrome";v="124", "Not-A.Brand";v="99"`)
	req.Header.Set("Sec-Ch-Ua-Mobile", "?0")
	req.Header.Set("Sec-Ch-Ua-Platform", `"Windows"`)
	req.Header.Set("Sec-Fetch-Dest", "document")
	req.Header.Set("Sec-Fetch-Mode", "navigate")
	req.Header.Set("Sec-Fetch-Site", "same-origin")
	req.Header.Set("Sec-Fetch-User", "?1")
	req.Header.Set("Upgrade-Insecure-Requests", "1")

	if parsed.Host != "" {
		req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))
		core.ApplySessionHeaders(req, parsed.Host)
	}

	for k, v := range customHeaders {
		if strings.EqualFold(k, "cookie") {
			existing := req.Header.Get("Cookie")
			if existing != "" {
				req.Header.Set("Cookie", existing+"; "+v)
			} else {
				req.Header.Set("Cookie", v)
			}
		} else {
			req.Header.Set(k, v)
		}
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, "", err
	}

	rawHTML := string(bodyBytes)

	// Check if blocked by Cloudflare challenge (403/503 Turnstile / Under Attack)
	if core.IsCloudflareChallenge(resp.StatusCode, rawHTML, resp.Header) {
		if core.HasAttemptedSolve(parsed.Host) {
			return nil, "", fmt.Errorf("access denied by Cloudflare anti-bot for '%s' (verification already attempted, skipping loop)", parsed.Host)
		}
		solvedHTML, err := core.SolveCloudflare(pageURL)
		if err != nil {
			return nil, "", fmt.Errorf("Cloudflare challenge bypass failed: %w", err)
		}
		rawHTML = solvedHTML
	} else if resp.StatusCode != http.StatusOK {
		return nil, "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, "", err
	}

	return doc, rawHTML, nil
}

// CleanImageURL resolves relative URLs, '//' protocol schemes, and removes WordPress resize query params (?w=...)
func CleanImageURL(rawURL, baseHost string) string {
	val := strings.TrimSpace(rawURL)
	if val == "" {
		return ""
	}

	// Strip WordPress resize query parameters (e.g. ?w=640&quality=80)
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

// GetFileExtension extracts the image extension (.jpg, .png, .webp)
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
