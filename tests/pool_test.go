package tests

import (
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"dmanga/internal/cli"
	"dmanga/internal/downloader"
	"dmanga/internal/utils"
)

func TestParseConcurrencyFlag(t *testing.T) {
	tests := []struct {
		name      string
		items     []string
		wantN     int
		wantFound bool
	}{
		{"short flag", []string{"dmanga", "-c", "12", "https://a.com/1"}, 12, true},
		{"long flag", []string{"--concurrency", "8"}, 8, true},
		{"short equals", []string{"-c=6"}, 6, true},
		{"long equals", []string{"--concurrency=20"}, 20, true},
		{"prompt mode single string", []string{"-c 15 https://a.com/1 https://a.com/2"}, 15, true},
		{"missing value", []string{"-c"}, 0, true},
		{"non numeric", []string{"-c", "abc"}, 0, true},
		{"zero", []string{"-c", "0"}, 0, true},
		{"negative", []string{"-c", "-3"}, 0, true},
		{"absent", []string{"dmanga", "https://a.com/1"}, 0, false},
		{"folder flag unaffected", []string{"-f", "https://a.com/1"}, 0, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, found := cli.ParseConcurrencyFlag(tt.items...)
			if n != tt.wantN || found != tt.wantFound {
				t.Errorf("ParseConcurrencyFlag(%v) = (%d, %v), expected (%d, %v)", tt.items, n, found, tt.wantN, tt.wantFound)
			}
		})
	}
}

func TestConcurrencyFlagDoesNotPollutURLInput(t *testing.T) {
	urls := cli.ParseInputURLs("-c 12 https://example.com/ch-1 https://example.com/ch-2")
	if len(urls) != 2 {
		t.Fatalf("Expected 2 URLs, got %d: %v", len(urls), urls)
	}
}

func TestResolveConcurrency(t *testing.T) {
	t.Setenv("DMANGA_CONCURRENCY", "")

	if n, warn := cli.ResolveConcurrency(0, false); n != downloader.DefaultConcurrency || warn != "" {
		t.Errorf("Default: got (%d, %q)", n, warn)
	}
	if n, _ := cli.ResolveConcurrency(7, true); n != 7 {
		t.Errorf("Flag value: got %d, expected 7", n)
	}
	if n, warn := cli.ResolveConcurrency(100, true); n != downloader.MaxConcurrency || warn == "" {
		t.Errorf("Clamp: got (%d, %q)", n, warn)
	}
	if n, warn := cli.ResolveConcurrency(0, true); n != downloader.DefaultConcurrency || warn == "" {
		t.Errorf("Invalid flag: got (%d, %q)", n, warn)
	}

	t.Setenv("DMANGA_CONCURRENCY", "14")
	if n, _ := cli.ResolveConcurrency(0, false); n != 14 {
		t.Errorf("Env value: got %d, expected 14", n)
	}
	if n, _ := cli.ResolveConcurrency(4, true); n != 4 {
		t.Errorf("Flag must override env: got %d, expected 4", n)
	}

	t.Setenv("DMANGA_CONCURRENCY", "zzz")
	if n, warn := cli.ResolveConcurrency(0, false); n != downloader.DefaultConcurrency || warn == "" {
		t.Errorf("Invalid env: got (%d, %q)", n, warn)
	}
}

func TestDownloadAllPoolRespectsConcurrencyAndNaming(t *testing.T) {
	const total = 30
	const concurrency = 4
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, make([]byte, 1024)...)

	var inFlight, maxInFlight atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		cur := inFlight.Add(1)
		for {
			old := maxInFlight.Load()
			if cur <= old || maxInFlight.CompareAndSwap(old, cur) {
				break
			}
		}
		time.Sleep(15 * time.Millisecond)
		inFlight.Add(-1)
		_, _ = w.Write(jpegBytes)
	}))
	defer server.Close()

	var urls []string
	for i := 1; i <= total; i++ {
		urls = append(urls, fmt.Sprintf("%s/img%d.jpg", server.URL, i))
	}

	dir := t.TempDir()
	completed, failed := downloader.DownloadAll(urls, dir, "example.com", nil, downloader.Options{
		Concurrency: concurrency,
		Client:      server.Client(),
		Out:         io.Discard,
	})

	if completed != total || failed != 0 {
		t.Fatalf("Expected %d completed and 0 failed, got %d/%d", total, completed, failed)
	}
	if got := maxInFlight.Load(); got > concurrency {
		t.Errorf("Max in-flight requests %d exceeded concurrency %d", got, concurrency)
	}
	if got := maxInFlight.Load(); got < 2 {
		t.Errorf("Expected parallel downloads, max in-flight was %d", got)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != total {
		t.Errorf("Expected %d files, found %d", total, len(entries))
	}
	for _, e := range entries {
		if filepath.Ext(e.Name()) == ".part" {
			t.Errorf("Leftover temporary file: %s", e.Name())
		}
	}
	for i := 1; i <= total; i++ {
		name := utils.FormatImageIndex(i, total) + ".jpg"
		if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
			t.Errorf("Expected image %d saved as %s: %v", i, name, err)
		}
	}
}

func TestDownloadAllCountsFailures(t *testing.T) {
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, make([]byte, 256)...)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/bad.jpg" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write(jpegBytes)
	}))
	defer server.Close()

	urls := []string{server.URL + "/a.jpg", server.URL + "/bad.jpg", server.URL + "/c.jpg"}
	completed, failed := downloader.DownloadAll(urls, t.TempDir(), "example.com", nil, downloader.Options{
		Concurrency: 3,
		Client:      server.Client(),
		Out:         io.Discard,
	})
	if completed != 2 || failed != 1 {
		t.Errorf("Expected 2 completed and 1 failed, got %d/%d", completed, failed)
	}
}

func TestExistingImageDetection(t *testing.T) {
	dir := t.TempDir()

	if _, ok := downloader.ExistingImage(dir, "001", ".jpg"); ok {
		t.Errorf("Nothing exists yet")
	}

	// Empty file and .part file must not count
	_ = os.WriteFile(filepath.Join(dir, "001.jpg"), nil, 0644)
	_ = os.WriteFile(filepath.Join(dir, "002.jpg.part"), []byte("partial"), 0644)
	if _, ok := downloader.ExistingImage(dir, "001", ".jpg"); ok {
		t.Errorf("Empty file should not count as existing")
	}
	if _, ok := downloader.ExistingImage(dir, "002", ".jpg"); ok {
		t.Errorf(".part file should not count as existing")
	}

	_ = os.WriteFile(filepath.Join(dir, "003.png"), []byte("data"), 0644)
	if name, ok := downloader.ExistingImage(dir, "003", ".webp"); !ok || name != "003.png" {
		t.Errorf("Expected 003.png to be detected even when URL extension differs, got (%q, %v)", name, ok)
	}
}

func TestDownloadAllSkipsExistingUnlessForced(t *testing.T) {
	jpegBytes := append([]byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}, make([]byte, 256)...)
	var hits atomic.Int64
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits.Add(1)
		_, _ = w.Write(jpegBytes)
	}))
	defer server.Close()

	var urls []string
	for i := 1; i <= 5; i++ {
		urls = append(urls, fmt.Sprintf("%s/img%d.jpg", server.URL, i))
	}
	dir := t.TempDir()
	opts := downloader.Options{Concurrency: 3, Client: server.Client(), Out: io.Discard}

	// First run downloads everything
	if done, failed := downloader.DownloadAll(urls, dir, "example.com", nil, opts); done != 5 || failed != 0 {
		t.Fatalf("first run: %d/%d", done, failed)
	}
	if hits.Load() != 5 {
		t.Fatalf("expected 5 requests, got %d", hits.Load())
	}

	// Second run skips all of them without touching the network
	var out strings.Builder
	opts.Out = &out
	if done, failed := downloader.DownloadAll(urls, dir, "example.com", nil, opts); done != 5 || failed != 0 {
		t.Fatalf("second run: %d/%d", done, failed)
	}
	if hits.Load() != 5 {
		t.Errorf("skip run should not hit the server, total requests %d", hits.Load())
	}
	if !strings.Contains(out.String(), "Skipped 5") {
		t.Errorf("expected skip summary, got %q", out.String())
	}

	// Partial resume: delete two files, only those are fetched
	_ = os.Remove(filepath.Join(dir, utils.FormatImageIndex(2, 5)+".jpg"))
	_ = os.Remove(filepath.Join(dir, utils.FormatImageIndex(4, 5)+".jpg"))
	opts.Out = io.Discard
	if done, failed := downloader.DownloadAll(urls, dir, "example.com", nil, opts); done != 5 || failed != 0 {
		t.Fatalf("resume run: %d/%d", done, failed)
	}
	if hits.Load() != 7 {
		t.Errorf("expected only 2 new requests on resume, total %d", hits.Load())
	}

	// Force re-downloads everything
	opts.Force = true
	if done, failed := downloader.DownloadAll(urls, dir, "example.com", nil, opts); done != 5 || failed != 0 {
		t.Fatalf("forced run: %d/%d", done, failed)
	}
	if hits.Load() != 12 {
		t.Errorf("expected 5 more requests with --force, total %d", hits.Load())
	}
}

func TestHasForceFlag(t *testing.T) {
	if !cli.HasForceFlag("dmanga", "--force", "https://a.com/1") {
		t.Errorf("--force should be detected")
	}
	if !cli.HasForceFlag("--force https://a.com/1") {
		t.Errorf("--force in prompt string should be detected")
	}
	if cli.HasForceFlag("-f", "https://a.com/1") {
		t.Errorf("-f (folder) must not be treated as force")
	}
}
