package fetcher

import (
	"fmt"
	"net/http"
	"strings"
)

// ApplyDefaultBrowserHeaders sets modern Chrome browser headers and Client Hints on the request
func ApplyDefaultBrowserHeaders(req *http.Request, host string) {
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

	if host != "" {
		req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))
		ApplySessionHeaders(req, host)
	}

	// If session uses a custom User-Agent (e.g. Edge), remove Chrome client hints to avoid fingerprint mismatch
	if sess := GetSession(host); sess != nil && sess.UserAgent != "" && !strings.Contains(sess.UserAgent, "Chrome/124") {
		req.Header.Del("Sec-Ch-Ua")
	}
}
