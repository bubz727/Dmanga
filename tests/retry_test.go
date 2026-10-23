package tests

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dmanga/internal/downloader"
	"dmanga/internal/fetcher"
)

func TestBackoffBoundsAndGrowth(t *testing.T) {
	for i := 0; i < 200; i++ {
		d0 := fetcher.Backoff(0)
		if d0 < 225*time.Millisecond || d0 > 375*time.Millisecond {
			t.Fatalf("Backoff(0) = %v, expected within 300ms +/- 25%%", d0)
		}
		d2 := fetcher.Backoff(2)
		if d2 < 900*time.Millisecond || d2 > 1500*time.Millisecond {
			t.Fatalf("Backoff(2) = %v, expected within 1.2s +/- 25%%", d2)
		}
		dBig := fetcher.Backoff(50)
		if dBig > 6250*time.Millisecond {
			t.Fatalf("Backoff(50) = %v, expected capped near 5s", dBig)
		}
	}
}

func TestSleepInterruptedByContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	fetcher.SetContext(ctx)
	defer fetcher.SetContext(context.Background())

	go func() {
		time.Sleep(50 * time.Millisecond)
		cancel()
	}()

	start := time.Now()
	if fetcher.Sleep(5 * time.Second) {
		t.Errorf("Sleep should report interruption")
	}
	if time.Since(start) > time.Second {
		t.Errorf("Sleep did not return promptly after cancel: %v", time.Since(start))
	}
}

func TestDownloadFileRetriesTransientServerError(t *testing.T) {
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, make([]byte, 512)...)
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			http.Error(w, "unavailable", http.StatusServiceUnavailable)
			return
		}
		_, _ = w.Write(jpegBytes)
	}))
	defer server.Close()

	start := time.Now()
	_, size, err := downloader.DownloadFile(server.Client(), server.URL+"/a.jpg", t.TempDir(), "001", ".jpg", "example.com", nil)
	if err != nil {
		t.Fatalf("Expected success after retry, got %v", err)
	}
	if size != int64(len(jpegBytes)) || hits.Load() != 2 {
		t.Errorf("Unexpected result: size=%d hits=%d", size, hits.Load())
	}
	// Old behaviour slept a flat 3s for a 503; jittered backoff should be well under that.
	if time.Since(start) > 2*time.Second {
		t.Errorf("Retry took too long (%v); backoff should start around 300ms", time.Since(start))
	}
}

func TestDownloadFileStalledBodyIsAborted(t *testing.T) {
	old := downloader.StallTimeout
	downloader.StallTimeout = 200 * time.Millisecond
	defer func() { downloader.StallTimeout = old }()

	release := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Length", "100000")
		_, _ = w.Write([]byte{0xFF, 0xD8, 0xFF, 0xE0})
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
	}))
	defer server.Close()
	defer close(release)

	start := time.Now()
	_, _, err := downloader.DownloadFile(server.Client(), server.URL+"/stall.jpg", t.TempDir(), "001", ".jpg", "example.com", nil)
	if err == nil {
		t.Fatalf("Expected error for stalled download")
	}
	if !strings.Contains(err.Error(), "stalled") {
		t.Errorf("Expected stall error, got %v", err)
	}
	if time.Since(start) > 15*time.Second {
		t.Errorf("Stalled download took too long to fail: %v", time.Since(start))
	}
}

func TestDownloadAllCircuitBreakerSkipsRemaining(t *testing.T) {
	const total = 40
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		http.NotFound(w, r)
	}))
	defer server.Close()

	var urls []string
	for i := 1; i <= total; i++ {
		urls = append(urls, fmt.Sprintf("%s/img%d.jpg", server.URL, i))
	}

	var out strings.Builder
	completed, failed := downloader.DownloadAll(urls, t.TempDir(), "example.com", nil, downloader.Options{
		Concurrency: 2,
		Client:      server.Client(),
		Out:         &out,
	})

	if completed != 0 || failed != total {
		t.Errorf("Expected 0 completed and %d failed, got %d/%d", total, completed, failed)
	}
	if hits.Load() > 16 {
		t.Errorf("Circuit breaker should stop early, but server received %d requests", hits.Load())
	}
	if !strings.Contains(out.String(), "skipping the remaining images") {
		t.Errorf("Expected circuit breaker message in output, got: %q", out.String())
	}
}

func TestDownloadAllStopsOnCancel(t *testing.T) {
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = io.WriteString(w, "x")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	fetcher.SetContext(ctx)
	defer fetcher.SetContext(context.Background())

	urls := []string{server.URL + "/1.jpg", server.URL + "/2.jpg", server.URL + "/3.jpg"}
	var out strings.Builder
	completed, failed := downloader.DownloadAll(urls, t.TempDir(), "example.com", nil, downloader.Options{
		Concurrency: 2,
		Client:      server.Client(),
		Out:         &out,
	})

	if completed != 0 || failed != len(urls) {
		t.Errorf("Expected all images skipped after cancel, got %d/%d", completed, failed)
	}
	if hits.Load() != 0 {
		t.Errorf("No requests should be sent after cancel, got %d", hits.Load())
	}
	if !strings.Contains(out.String(), "Interrupted") {
		t.Errorf("Expected interrupt message, got %q", out.String())
	}
}
