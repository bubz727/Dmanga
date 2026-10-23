package downloader

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	internalimage "dmanga/internal/image"
)

// maxPreallocBytes caps the buffer pre-sized from Content-Length to avoid huge allocations
// when a server reports a bogus length.
const maxPreallocBytes = 64 << 20

// maxRetries is the number of retries after the first attempt for a single image.
const maxRetries = 3

// StallTimeout is how long an image body may go without receiving any bytes before the attempt is aborted
// and retried. It is a variable so tests can shorten it.
var StallTimeout = 15 * time.Second

// stallReader aborts a download (via cancel) when no data arrives for StallTimeout.
type stallReader struct {
	r       io.Reader
	timer   *time.Timer
	timeout time.Duration
	stalled atomic.Bool
}

func (s *stallReader) Read(p []byte) (int, error) {
	n, err := s.r.Read(p)
	if n > 0 {
		s.timer.Reset(s.timeout)
	}
	return n, err
}

// readBody reads the response body into memory, pre-sizing the buffer from Content-Length when known.
// If no bytes arrive for StallTimeout the request is cancelled and a stall error is returned.
func readBody(resp *http.Response, cancel context.CancelFunc) ([]byte, error) {
	sr := &stallReader{r: resp.Body, timeout: StallTimeout}
	sr.timer = time.AfterFunc(sr.timeout, func() {
		sr.stalled.Store(true)
		cancel()
	})
	defer sr.timer.Stop()

	var buf bytes.Buffer
	if n := resp.ContentLength; n > 0 && n <= maxPreallocBytes {
		buf.Grow(int(n))
	}
	if _, err := io.Copy(&buf, sr); err != nil {
		if sr.stalled.Load() {
			return nil, fmt.Errorf("download stalled: no data received for %v", sr.timeout)
		}
		return nil, err
	}
	return buf.Bytes(), nil
}

// attemptResult describes the outcome of one HTTP attempt.
type attemptResult struct {
	body      []byte
	err       error
	retryable bool
	delay     time.Duration // wait before the next attempt when retryable
}

// fetchAttempt performs a single GET for an image and classifies the outcome.
func fetchAttempt(client *http.Client, fetchURL, host, origName string, attempt int) attemptResult {
	parent := fetcher.Context()
	ctx, cancel := context.WithCancel(parent)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", fetchURL, nil)
	if err != nil {
		return attemptResult{err: err}
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))
	fetcher.ApplySessionHeaders(req, host)

	resp, err := client.Do(req)
	if err != nil {
		if parent.Err() != nil {
			return attemptResult{err: parent.Err()}
		}
		return attemptResult{err: err, retryable: true, delay: fetcher.Backoff(attempt)}
	}
	defer resp.Body.Close()

	// Handle Rate Limiting (HTTP 429)
	if resp.StatusCode == http.StatusTooManyRequests {
		retryAfter := fetcher.ParseRetryAfter(resp.Header.Get("Retry-After"), attempt)
		fmt.Printf("\n[!] [RATE LIMIT WARNING] Host '%s' returned HTTP 429 (Too Many Requests)!\n", host)
		fmt.Printf("[*] Pausing downloads for %v before retrying %s (attempt %d/%d)...\n\n", retryAfter, origName, attempt+1, maxRetries)
		fetcher.TriggerRateLimitCooldown(retryAfter)
		return attemptResult{err: fmt.Errorf("status code %d (rate limited)", resp.StatusCode), retryable: true, delay: retryAfter}
	}

	if resp.StatusCode != http.StatusOK {
		retryable := resp.StatusCode == http.StatusBadGateway || resp.StatusCode == http.StatusGatewayTimeout || resp.StatusCode == http.StatusServiceUnavailable
		return attemptResult{err: fmt.Errorf("status code %d", resp.StatusCode), retryable: retryable, delay: fetcher.Backoff(attempt)}
	}

	body, err := readBody(resp, cancel)
	if err != nil {
		if parent.Err() != nil {
			return attemptResult{err: parent.Err()}
		}
		return attemptResult{err: err, retryable: true, delay: fetcher.Backoff(attempt)}
	}
	return attemptResult{body: body}
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

	var bodyBytes []byte
	var lastErr error

	for attempt := 0; attempt <= maxRetries; attempt++ {
		fetcher.CheckRateLimitCooldown()
		if err := fetcher.Context().Err(); err != nil {
			return "", 0, err
		}

		res := fetchAttempt(client, fetchURL, host, origName, attempt)
		if res.err == nil {
			bodyBytes = res.body
			lastErr = nil
			break
		}
		lastErr = res.err

		if !res.retryable {
			return "", 0, res.err
		}
		if attempt == maxRetries {
			break
		}
		if !fetcher.Sleep(res.delay) {
			return "", 0, fetcher.Context().Err()
		}
	}

	// All attempts exhausted without obtaining a body: never write an empty file
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
