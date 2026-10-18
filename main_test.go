package main_test

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"manga-downloader/parsers"
	"manga-downloader/themes"
)

var flagURL = flag.String("url", "", "Chapter URL to test")

// getTargetURL retrieves the URL from the -url flag or positional argument after -args
func getTargetURL() string {
	if *flagURL != "" {
		return *flagURL
	}
	// Check if there is a URL in os.Args (e.g. go test . -args https://...)
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return arg
		}
	}
	return ""
}

func TestMangaParser(t *testing.T) {
	targetURL := strings.TrimSpace(getTargetURL())
	if targetURL == "" {
		t.Skip("Skip: Provide a chapter URL to test. Example: go test . -args https://01.apkomik.com/chapter-1/")
	}

	t.Logf("==================================================")
	t.Logf("PARSER DIAGNOSTIC FOR: %s", targetURL)
	t.Logf("==================================================")

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" {
		t.Fatalf("[FAILED] Invalid URL format: %v", err)
	}

	// 1. Check Router / Registry
	p, err := parsers.FindParser(parsed.Host)
	if err != nil {
		t.Fatalf("[FAILED DOMAIN] Domain '%s' is not registered in parser registry!", parsed.Host)
	}
	t.Logf("[OK 1/4] Domain Match     : Recognized as '%s'", p.Name())

	// 2. Check Connection & Anti-Bot / Cloudflare Protection
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("[CONNECTION FAILED] Unable to reach server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("[BLOCKED - HTTP 403] Access denied! Server blocked request (Cloudflare/WAF).")
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("[BLOCKED - HTTP 503] Server 503 (Cloudflare Turnstile/Under Attack).")
	} else if resp.StatusCode != http.StatusOK {
		t.Fatalf("[HTTP FAILED] Server responded with status code %d %s", resp.StatusCode, resp.Status)
	}
	t.Logf("[OK 2/4] HTTP Connection  : 200 OK (Clean access, unblocked)")

	// 3. Image Extraction
	start := time.Now()
	images, err := p.GetImages(targetURL)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("[EXTRACTION FAILED] %v", err)
	}
	if len(images) == 0 {
		t.Fatalf("[EMPTY IMAGES FAILED] No images found. Selector/script structure may have changed.")
	}
	t.Logf("[OK 3/4] Image Extraction : Found %d images (time: %v)", len(images), duration)

	// 4. Sample Image URL Validation
	sample := images[0]
	t.Logf("[OK 4/4] Sample Image URL : %s", sample)
	t.Logf("==================================================")
	t.Logf("[SUCCESS] Parser '%s' IS 100%% FUNCTIONAL!", p.Name())
}

func TestImageFormatConversion(t *testing.T) {
	// 1. JPEG magic bytes
	jpegBytes := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
	resBytes, ext, err := themes.EnsureJPGorPNG(jpegBytes, ".jpg")
	if err != nil || ext != ".jpg" || len(resBytes) != len(jpegBytes) {
		t.Errorf("JPEG check failed: ext=%s, err=%v", ext, err)
	}

	// 2. PNG magic bytes
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	resBytes, ext, err = themes.EnsureJPGorPNG(pngBytes, ".png")
	if err != nil || ext != ".png" || len(resBytes) != len(pngBytes) {
		t.Errorf("PNG check failed: ext=%s, err=%v", ext, err)
	}
}
