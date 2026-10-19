package core

import (
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/PuerkitoBio/goquery"
)

// FetchHTML downloads a webpage and returns both the goquery Document and raw HTML string
func FetchHTML(pageURL string, customHeaders map[string]string) (*goquery.Document, string, error) {
	client := &http.Client{Timeout: 30 * time.Second}

	parsed, err := url.Parse(pageURL)
	if err != nil {
		return nil, "", err
	}

	maxRetries := 3
	var rawHTML string

	for attempt := 0; attempt <= maxRetries; attempt++ {
		CheckRateLimitCooldown()

		req, err := http.NewRequest("GET", pageURL, nil)
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
			ApplySessionHeaders(req, parsed.Host)
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
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return nil, "", err
		}

		// Handle Rate Limiting (HTTP 429)
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := ParseRetryAfter(resp.Header.Get("Retry-After"), attempt)
			fmt.Printf("\n[!] [RATE LIMIT WARNING] '%s' returned HTTP 429 (Too Many Requests)!\n", parsed.Host)
			fmt.Printf("[*] Pausing requests for %v before retrying (attempt %d/%d)...\n\n", retryAfter, attempt+1, maxRetries)
			TriggerRateLimitCooldown(retryAfter)
			time.Sleep(retryAfter)
			continue
		}

		bodyBytes, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(1 * time.Second)
				continue
			}
			return nil, "", err
		}

		rawHTML = string(bodyBytes)

		// Check if blocked by Cloudflare challenge (403/503 Turnstile / Under Attack)
		if IsCloudflareChallenge(resp.StatusCode, rawHTML, resp.Header) {
			if HasAttemptedSolve(parsed.Host) {
				return nil, "", fmt.Errorf("access denied by Cloudflare anti-bot for '%s' (verification already attempted, skipping loop)", parsed.Host)
			}
			solvedHTML, err := SolveCloudflare(pageURL)
			if err != nil {
				return nil, "", fmt.Errorf("Cloudflare challenge bypass failed: %w", err)
			}
			rawHTML = solvedHTML
		} else if resp.StatusCode != http.StatusOK {
			return nil, "", fmt.Errorf("HTTP error: %d %s", resp.StatusCode, resp.Status)
		}

		// Successfully read and verified page
		break
	}

	doc, err := goquery.NewDocumentFromReader(strings.NewReader(rawHTML))
	if err != nil {
		return nil, "", err
	}

	return doc, rawHTML, nil
}
