package fetcher

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

		ApplyDefaultBrowserHeaders(req, parsed.Host)

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

		// If caller requests RSC stream or API data, adjust fetch metadata
		if _, isRSC := customHeaders["rsc"]; isRSC {
			req.Header.Set("Sec-Fetch-Dest", "empty")
			req.Header.Set("Sec-Fetch-Mode", "cors")
			req.Header.Del("Upgrade-Insecure-Requests")
			req.Header.Set("Accept", "*/*")
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

			// Verification succeeded! Retry with the new session cookies
			if attempt < maxRetries {
				time.Sleep(1 * time.Second)
				continue
			}
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
