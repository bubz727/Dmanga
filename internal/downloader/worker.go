package downloader

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	internalimage "dmanga/internal/image"
)

// maxPreallocBytes caps the buffer pre-sized from Content-Length to avoid huge allocations
// when a server reports a bogus length.
const maxPreallocBytes = 64 << 20

// readBody reads the response body into memory, pre-sizing the buffer from Content-Length
// when it is known to avoid repeated slice growth.
func readBody(resp *http.Response) ([]byte, error) {
	var buf bytes.Buffer
	if n := resp.ContentLength; n > 0 && n <= maxPreallocBytes {
		buf.Grow(int(n))
	}
	if _, err := io.Copy(&buf, resp.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// DownloadFile downloads a single image, processes it (descrambling/format conversion), and saves it to disk.
// It returns the saved file name and the number of bytes written.
func DownloadFile(client *http.Client, fileURL, targetDir, baseName, defaultExt, host string, parser core.Parser) (string, int64, error) {
	fetchURL := fileURL
	if idx := strings.Index(fetchURL, "#"); idx != -1 {
		fetchURL = fetchURL[:idx]
	}

	// Extract original file name from URL for display
	origName := "image"
	if parsed, err := url.Parse(fetchURL); err == nil {
		b := filepath.Base(parsed.Path)
		if b != "" && b != "/" && b != "." {
			origName = b
		}
	}

	maxRetries := 3
	var bodyBytes []byte
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		fetcher.CheckRateLimitCooldown()

		req, reqErr := http.NewRequest("GET", fetchURL, nil)
		if reqErr != nil {
			return "", 0, reqErr
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))
		fetcher.ApplySessionHeaders(req, host)

		resp, doErr := client.Do(req)
		if doErr != nil {
			lastErr = doErr
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return "", 0, lastErr
		}

		// Handle Rate Limiting (HTTP 429)
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			lastErr = fmt.Errorf("status code %d (rate limited)", resp.StatusCode)
			retryAfter := fetcher.ParseRetryAfter(resp.Header.Get("Retry-After"), attempt)
			fmt.Printf("\n[!] [RATE LIMIT WARNING] Host '%s' returned HTTP 429 (Too Many Requests)!\n", host)
			fmt.Printf("[*] Pausing downloads for %v before retrying %s (attempt %d/%d)...\n\n", retryAfter, origName, attempt+1, maxRetries)
			fetcher.TriggerRateLimitCooldown(retryAfter)
			time.Sleep(retryAfter)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("status code %d", resp.StatusCode)
			if attempt < maxRetries && (resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusServiceUnavailable) {
				time.Sleep(3 * time.Second)
				continue
			}
			return "", 0, lastErr
		}

		body, readErr := readBody(resp)
		resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt < maxRetries {
				time.Sleep(1 * time.Second)
				continue
			}
			return "", 0, lastErr
		}

		// Successfully read body
		bodyBytes = body
		lastErr = nil
		break
	}

	// All attempts exhausted without obtaining a body (e.g. persistent HTTP 429): never write an empty file
	if bodyBytes == nil {
		if lastErr == nil {
			lastErr = errors.New("no data received")
		}
		return "", 0, fmt.Errorf("giving up after %d attempts: %w", maxRetries+1, lastErr)
	}

	// 1. Process image with ImageProcessor (e.g. descrambler) if supported
	if processor, ok := parser.(core.ImageProcessor); ok {
		processed, err := processor.ProcessImage(bodyBytes, fileURL)
		if err == nil {
			bodyBytes = processed
		}
	}

	// 2. Ensure final format is JPG or PNG (convert WebP, GIF, etc. to JPG)
	finalBytes, finalExt, err := internalimage.EnsureJPGorPNG(bodyBytes, defaultExt)
	if err == nil {
		bodyBytes = finalBytes
		defaultExt = finalExt
	}

	fileName, err := WriteImageFile(targetDir, baseName, defaultExt, bodyBytes)
	return fileName, int64(len(bodyBytes)), err
}
