package cli

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/parser"
)

// RunDiagnosticTest performs diagnostics on a URL without downloading images
func RunDiagnosticTest(rawURL string) {
	fmt.Println()
	fmt.Printf("PARSER DIAGNOSTIC: %s\n", rawURL)

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		fmt.Println("[X] Invalid URL format!")
		return
	}

	// 1. Check Router / Registry
	p, err := parser.FindParser(parsed.Host)
	if err != nil {
		fmt.Printf("[X 1/4] Domain '%s' is NOT registered in parser registry!\n", parsed.Host)
		return
	}
	fmt.Printf("[OK 1/4] Domain Match      : %s\n", p.Name())

	// 2. Check Connection & Anti-Bot Protection
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", rawURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))
	fetcher.ApplySessionHeaders(req, parsed.Host)

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[X 2/4] Failed to connect to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(resp.Body)
	isCF := fetcher.IsCloudflareChallenge(resp.StatusCode, string(bodyBytes), resp.Header)

	if isCF {
		fmt.Println("[!] 2/4 Connection Status : Cloudflare Challenge Active (Will auto-solve via browser window)")
	} else if resp.StatusCode == http.StatusForbidden {
		fmt.Println("[X 2/4] BLOCKED (HTTP 403) - Anti-Bot WAF detected!")
		return
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		fmt.Println("[X 2/4] BLOCKED (HTTP 503) - Server unavailable!")
		return
	} else if resp.StatusCode != http.StatusOK {
		fmt.Printf("[X 2/4] HTTP Error: Status %d %s\n", resp.StatusCode, resp.Status)
		return
	} else {
		fmt.Println("[OK 2/4] HTTP Connection   : 200 OK (Clean access, unblocked)")
	}

	// 3. Image & Chapter Extraction
	start := time.Now()
	chap, err := core.FetchChapter(p, rawURL)
	dur := time.Since(start)

	if err != nil {
		fmt.Printf("[X 3/4] Extraction FAILED: %v\n", err)
		return
	}
	if len(chap.Images) == 0 {
		fmt.Println("[X 3/4] Extraction FAILED: 0 images found (CSS selector or script structure may have changed)!")
		return
	}
	titleInfo := ""
	if chap.Title != "" {
		titleInfo = fmt.Sprintf(" [Title: '%s']", chap.Title)
	}
	fmt.Printf("[OK 3/4] Image Extraction  : Found %d images%s (time: %v)\n", len(chap.Images), titleInfo, dur)

	// 4. Sample Image URL Validation
	sampleURL := chap.Images[0]
	if idx := strings.Index(sampleURL, "#"); idx != -1 {
		sampleURL = sampleURL[:idx]
	}
	fmt.Printf("[OK 4/4] Sample Image URL  : %s\n", sampleURL)
	fmt.Println()
	fmt.Printf("[SUCCESS] Parser '%s'\n", p.Name())
}
