package tests

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"dmanga/internal/downloader"
	"dmanga/internal/fetcher"
)

const (
	benchImages  = 60
	benchImgSize = 100 * 1024
	benchLatency = 10 * time.Millisecond
)

func benchJPEG() []byte {
	data := make([]byte, benchImgSize)
	copy(data, []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10})
	return data
}

// newImageServer starts a local server that serves synthetic JPEG payloads with a fixed latency and
// counts how many TCP connections were opened.
func newImageServer(latency time.Duration) (*httptest.Server, *atomic.Int64) {
	payload := benchJPEG()
	var conns atomic.Int64
	server := httptest.NewUnstartedServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(latency)
		_, _ = w.Write(payload)
	}))
	server.Config.ConnState = func(_ net.Conn, state http.ConnState) {
		if state == http.StateNew {
			conns.Add(1)
		}
	}
	server.Start()
	return server, &conns
}

func imageURLs(base string, n int) []string {
	urls := make([]string, n)
	for i := range urls {
		urls[i] = fmt.Sprintf("%s/img%d.jpg", base, i+1)
	}
	return urls
}

func BenchmarkDownloadAllConcurrency(b *testing.B) {
	server, _ := newImageServer(benchLatency)
	defer server.Close()
	urls := imageURLs(server.URL, benchImages)

	for _, c := range []int{1, 5, 10, 20} {
		b.Run(fmt.Sprintf("workers=%d", c), func(b *testing.B) {
			b.SetBytes(int64(benchImages * benchImgSize))
			for i := 0; i < b.N; i++ {
				dir := b.TempDir()
				done, failed := downloader.DownloadAll(urls, dir, "example.com", nil, downloader.Options{
					Concurrency: c,
					Client:      fetcher.ImageClient(),
					Out:         io.Discard,
				})
				if done != benchImages || failed != 0 {
					b.Fatalf("unexpected result: %d completed, %d failed", done, failed)
				}
			}
		})
	}
}

// BenchmarkDownloadAllClients compares the tuned shared transport against a default http.Client
// (MaxIdleConnsPerHost = 2) at the same concurrency.
func BenchmarkDownloadAllClients(b *testing.B) {
	server, _ := newImageServer(benchLatency)
	defer server.Close()
	urls := imageURLs(server.URL, benchImages)

	clients := map[string]func() *http.Client{
		"shared-tuned":   fetcher.ImageClient,
		"default-client": func() *http.Client { return &http.Client{Transport: &http.Transport{}} },
	}
	for _, name := range []string{"shared-tuned", "default-client"} {
		mk := clients[name]
		b.Run(name, func(b *testing.B) {
			client := mk()
			b.SetBytes(int64(benchImages * benchImgSize))
			for i := 0; i < b.N; i++ {
				dir := b.TempDir()
				done, failed := downloader.DownloadAll(urls, dir, "example.com", nil, downloader.Options{
					Concurrency: 10,
					Client:      client,
					Out:         io.Discard,
				})
				if done != benchImages || failed != 0 {
					b.Fatalf("unexpected result: %d completed, %d failed", done, failed)
				}
			}
		})
	}
}

// TestSharedTransportReusesConnections verifies the Phase 1 goal: with 10 workers the tuned transport keeps
// connections alive across batches, while a default client (2 idle connections per host) must re-dial.
func TestSharedTransportReusesConnections(t *testing.T) {
	const workers = 10
	runTwoBatches := func(client *http.Client) int64 {
		server, conns := newImageServer(10 * time.Millisecond)
		defer server.Close()
		urls := imageURLs(server.URL, 60)
		for batch := 0; batch < 2; batch++ {
			done, failed := downloader.DownloadAll(urls, t.TempDir(), "example.com", nil, downloader.Options{
				Concurrency: workers,
				Client:      client,
				Out:         io.Discard,
			})
			if done != 60 || failed != 0 {
				t.Fatalf("unexpected result: %d completed, %d failed", done, failed)
			}
		}
		return conns.Load()
	}

	tuned := runTwoBatches(fetcher.ImageClient())
	baseline := runTwoBatches(&http.Client{Transport: &http.Transport{}})

	if tuned > workers+2 {
		t.Errorf("Tuned transport opened %d connections for %d workers over two batches, expected about %d", tuned, workers, workers)
	}
	if baseline <= tuned {
		t.Errorf("Expected the default client to open more connections (got %d) than the tuned transport (%d)", baseline, tuned)
	}
	t.Logf("connections opened over 2 batches with %d workers: tuned=%d default=%d", workers, tuned, baseline)
}
