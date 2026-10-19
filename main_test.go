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

	"github.com/PuerkitoBio/goquery"
	"manga-downloader/core"
	"manga-downloader/parsers"
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
	p, err := core.FindParser(parsed.Host)
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
	resBytes, ext, err := core.EnsureJPGorPNG(jpegBytes, ".jpg")
	if err != nil || ext != ".jpg" || len(resBytes) != len(jpegBytes) {
		t.Errorf("JPEG check failed: ext=%s, err=%v", ext, err)
	}

	// 2. PNG magic bytes
	pngBytes := []byte{0x89, 'P', 'N', 'G', 0x0D, 0x0A, 0x1A, 0x0A, 0x00}
	resBytes, ext, err = core.EnsureJPGorPNG(pngBytes, ".png")
	if err != nil || ext != ".png" || len(resBytes) != len(pngBytes) {
		t.Errorf("PNG check failed: ext=%s, err=%v", ext, err)
	}
}

func TestParseInputURLs(t *testing.T) {
	// 1. Space separated
	input1 := "https://example.com/ch-1 https://example.com/ch-2 https://example.com/ch-3"
	urls1 := core.ParseInputURLs(input1)
	if len(urls1) != 3 {
		t.Fatalf("Expected 3 URLs, got %d", len(urls1))
	}
	if urls1[0] != "https://example.com/ch-1" || urls1[2] != "https://example.com/ch-3" {
		t.Errorf("Unexpected URLs parsed: %v", urls1)
	}

	// 2. Pipe separated
	input2 := "https://example.com/ch-1 | https://example.com/ch-2"
	urls2 := core.ParseInputURLs(input2)
	if len(urls2) != 2 {
		t.Fatalf("Expected 2 URLs for pipe delimiter, got %d", len(urls2))
	}

	// 3. Comma and newline separated
	input3 := "https://example.com/ch-1, https://example.com/ch-2\nhttps://example.com/ch-3\tinvalid-url"
	urls3 := core.ParseInputURLs(input3)
	if len(urls3) != 3 {
		t.Fatalf("Expected 3 valid URLs, got %d", len(urls3))
	}
}

func TestNormalizeChapterTitle(t *testing.T) {
	tests := []struct {
		rawTitle string
		expected string
	}{
		{"Chapter 344 - Saharan vs. Valhalla", "Ch 344"},
		{"Solo Leveling Chapter 370", "Ch 370"},
		{"Ch. 105.5: The End", "Ch 105.5"},
		{"Special Episode", "Special Episode"},
		{"", ""},
	}

	for _, tt := range tests {
		got := core.NormalizeChapterTitle(tt.rawTitle)
		if got != tt.expected {
			t.Errorf("NormalizeChapterTitle(%q) = %q, expected %q", tt.rawTitle, got, tt.expected)
		}
	}
}

func TestSanitizeFolderName(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"Ch 344: Special?", "Ch 344- Special-"},
		{`Volume 1 / Ch 10 *Special* <Final>`, "Volume 1 - Ch 10 -Special- -Final-"},
		{"  Ch 105...  ", "Ch 105"},
		{"Ch | 12", "Ch - 12"},
	}

	for _, tt := range tests {
		got := core.SanitizeFolderName(tt.input)
		if got != tt.expected {
			t.Errorf("SanitizeFolderName(%q) = %q, expected %q", tt.input, got, tt.expected)
		}
	}
}

func TestAsuraScansHTMLTitleExtraction(t *testing.T) {
	// 1. Testing with user's exact snippet from Asura Scans
	htmlSnippet := `
	<!DOCTYPE html>
	<html>
	<head><title>Overgeared - Chapter 344</title></head>
	<body>
		<div class="header">
			<div class="font-semibold truncate text-zinc-400">
				<span class="text-zinc-100">Chapter <!-- -->344</span>
				<span class="font-normal text-sm"> - <!-- -->Saharan vs. Valhalla</span>
			</div>
		</div>
	</body>
	</html>
	`

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(htmlSnippet))
	if err != nil {
		t.Fatalf("Failed to parse HTML document: %v", err)
	}

	title := parsers.ExtractAsuraChapterTitle(doc)
	expected := "Ch 344"

	if title != expected {
		t.Fatalf("Expected chapter title %q, got %q", expected, title)
	}
	t.Logf("[OK] Extracted Asura chapter title successfully as: %s", title)

	// 2. Testing when the selector is absent (should return empty string, no guessing)
	emptyDoc, _ := goquery.NewDocumentFromReader(strings.NewReader("<div>No header here</div>"))
	emptyTitle := parsers.ExtractAsuraChapterTitle(emptyDoc)
	if emptyTitle != "" {
		t.Fatalf("Expected empty title when selector is missing, got %q", emptyTitle)
	}
}

func TestMultiChapterNamingCollision(t *testing.T) {
	// Simulating multi-link folder collision prevention logic
	usedFolders := make(map[string]int)
	resolveFolderName := func(title string, idx int) string {
		folderName := title
		if folderName == "" {
			folderName = fmt.Sprintf("Ch %d", idx+1)
		}
		key := strings.ToLower(folderName)
		if count, exists := usedFolders[key]; exists {
			usedFolders[key] = count + 1
			folderName = fmt.Sprintf("%s (%d)", folderName, count+1)
		} else {
			usedFolders[key] = 1
		}
		return folderName
	}

	// Scenario: multiple links where two share the same chapter name
	r1 := resolveFolderName("Ch 344", 0)
	r2 := resolveFolderName("Ch 344", 1)
	r3 := resolveFolderName("Ch 345", 2)

	if r1 != "Ch 344" {
		t.Errorf("Unexpected r1: %s", r1)
	}
	if r2 != "Ch 344 (2)" {
		t.Errorf("Expected deduplicated r2: 'Ch 344 (2)', got '%s'", r2)
	}
	if r3 != "Ch 345" {
		t.Errorf("Unexpected r3: %s", r3)
	}
}
