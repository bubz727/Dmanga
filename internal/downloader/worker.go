package downloader

import (
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	internalimage "dmanga/internal/image"
)

// DownloadFile downloads a single image, processes it (descrambling/format conversion), and saves it to disk
func DownloadFile(client *http.Client, fileURL, targetDir, baseName, defaultExt, host string, parser core.Parser) (string, string, int, int, error) {
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
	var err error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		fetcher.CheckRateLimitCooldown()

		req, err := http.NewRequest("GET", fetchURL, nil)
		if err != nil {
			return "", origName, 0, 0, err
		}

		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
		req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))
		fetcher.ApplySessionHeaders(req, host)

		resp, err := client.Do(req)
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(2 * time.Second)
				continue
			}
			return "", origName, 0, 0, err
		}

		// Handle Rate Limiting (HTTP 429)
		if resp.StatusCode == http.StatusTooManyRequests {
			resp.Body.Close()
			retryAfter := fetcher.ParseRetryAfter(resp.Header.Get("Retry-After"), attempt)
			fmt.Printf("\n[!] [RATE LIMIT WARNING] Host '%s' returned HTTP 429 (Too Many Requests)!\n", host)
			fmt.Printf("[*] Pausing downloads for %v before retrying %s (attempt %d/%d)...\n\n", retryAfter, origName, attempt+1, maxRetries)
			fetcher.TriggerRateLimitCooldown(retryAfter)
			time.Sleep(retryAfter)
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			if attempt < maxRetries && (resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusServiceUnavailable) {
				time.Sleep(3 * time.Second)
				continue
			}
			return "", origName, 0, 0, fmt.Errorf("status code %d", resp.StatusCode)
		}

		bodyBytes, err = io.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			if attempt < maxRetries {
				time.Sleep(1 * time.Second)
				continue
			}
			return "", origName, 0, 0, err
		}

		// Successfully read body
		break
	}
	if err != nil {
		return "", origName, 0, 0, err
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

	// 3. Read image dimensions (width x height)
	width, height := 0, 0
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(bodyBytes)); err == nil {
		width, height = cfg.Width, cfg.Height
	}

	fileName, err := WriteImageFile(targetDir, baseName, defaultExt, bodyBytes)
	return fileName, origName, width, height, err
}
