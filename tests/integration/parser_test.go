package integration_test

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/parser"
)

var flagURL = flag.String("url", "", "Chapter URL to test")

func getTargetURL() string {
	if *flagURL != "" {
		return *flagURL
	}
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
		t.Skip("Skip: Provide a chapter URL to test. Example: go test ./tests/integration/... -args https://comic-days.com/episode/3270296674374344033")
	}

	t.Log("")
	t.Logf("PARSER DIAGNOSTIC FOR: %s", targetURL)
	t.Log("")

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" {
		t.Fatalf("[FAILED] Invalid URL format: %v", err)
	}

	p, err := parser.FindParser(parsed.Host)
	if err != nil {
		t.Fatalf("[FAILED DOMAIN] Domain '%s' is not registered in parser registry!", parsed.Host)
	}
	t.Logf("[OK 1/4] Domain Match     : Recognized as '%s'", p.Name())

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

	start := time.Now()
	chap, err := core.FetchChapter(p, targetURL)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("[EXTRACTION FAILED] %v", err)
	}
	if len(chap.Images) == 0 {
		t.Fatalf("[EMPTY IMAGES FAILED] No images found. Selector/script structure may have changed.")
	}
	t.Logf("[OK 3/4] Image Extraction : Found %d images (time: %v)", len(chap.Images), duration)

	sample := chap.Images[0]
	t.Logf("[OK 4/4] Sample Image URL : %s", sample)
	t.Log("")
	t.Logf("[SUCCESS] Parser '%s' IS 100%% FUNCTIONAL!", p.Name())
}
