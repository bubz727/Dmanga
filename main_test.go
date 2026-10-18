package main_test

import (
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"strings"
	"testing"
	"time"

	"manga-downloader/parsers"
)

var flagURL = flag.String("url", "", "URL chapter untuk dites")

// getTargetURL mengambil URL dari flag -url ATAU dari argumen bebas setelah -args
func getTargetURL() string {
	if *flagURL != "" {
		return *flagURL
	}
	// Periksa apakah ada URL di argumen os.Args (misal: go test . -args https://...)
	for _, arg := range os.Args {
		if strings.HasPrefix(arg, "http://") || strings.HasPrefix(arg, "https://") {
			return arg
		}
	}
	return ""
}

func TestMangaParser(t *testing.T) {
	targetURL := strings.TrimSpace(getTargetURL())
	if targetURL == "" {
		t.Skip("Lewati: Masukkan URL chapter untuk dites. Contoh: go test . -args https://01.apkomik.com/chapter-1/")
	}

	t.Logf("==================================================")
	t.Logf("DIAGNOSA PARSER UNTUK: %s", targetURL)
	t.Logf("==================================================")

	parsed, err := url.Parse(targetURL)
	if err != nil || parsed.Host == "" {
		t.Fatalf("[GAGAL] Format URL tidak valid: %v", err)
	}

	// 1. Cek Router / Registry
	p, err := parsers.FindParser(parsed.Host)
	if err != nil {
		t.Fatalf("[GAGAL DOMAIN] Domain '%s' belum terdaftar di registry!", parsed.Host)
	}
	t.Logf("[✓ 1/4] Domain Match     : Dikenali sebagai '%s'", p.Name())

	// 2. Cek Koneksi & Blokir Anti-Bot / Cloudflare
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", targetURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))

	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("[GAGAL KONEKSI] Tidak bisa menghubungi server: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		t.Fatalf("[DIBLOKIR - HTTP 403] Akses ditolak! Server memblokir request (Cloudflare/WAF).")
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		t.Fatalf("[DIBLOKIR - HTTP 503] Server 503 (Cloudflare Turnstile/Under Attack).")
	} else if resp.StatusCode != http.StatusOK {
		t.Fatalf("[GAGAL HTTP] Server merespon dengan status code %d %s", resp.StatusCode, resp.Status)
	}
	t.Logf("[✓ 2/4] Koneksi HTTP      : 200 OK (Tidak terblokir)")

	// 3. Ekstraksi Gambar
	start := time.Now()
	images, err := p.GetImages(targetURL)
	duration := time.Since(start)

	if err != nil {
		t.Fatalf("[GAGAL EKSTRAKSI] %v", err)
	}
	if len(images) == 0 {
		t.Fatalf("[GAGAL GAMBAR KOSONG] Tidak ada gambar yang ditemukan. Selector/script kemungkinan berubah.")
	}
	t.Logf("[✓ 3/4] Ekstraksi Gambar : Ditemukan %d gambar (waktu: %v)", len(images), duration)

	// 4. Validasi URL
	sample := images[0]
	t.Logf("[✓ 4/4] Sampel Gambar    : %s", sample)
	t.Logf("==================================================")
	t.Logf("[SUKSES] Parser '%s' BERFUNGSI 100%%!", p.Name())
}
