package main

import (
	"bufio"
	"bytes"
	"fmt"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"manga-downloader/parsers"
	"manga-downloader/themes"
)

func main() {
	// Mode 1: DIAGNOSTIC TEST MODE (-t atau --test)
	// Contoh: downloadManga -t https://01.apkomik.com/chapter-1/
	// atau:   go run . -t https://01.apkomik.com/chapter-1/
	if len(os.Args) >= 2 && (os.Args[1] == "-t" || os.Args[1] == "--test") {
		testURL := ""
		if len(os.Args) >= 3 {
			testURL = os.Args[2]
		} else {
			fmt.Print("Masukkan URL untuk di-test: ")
			scanner := bufio.NewScanner(os.Stdin)
			if scanner.Scan() {
				testURL = strings.TrimSpace(scanner.Text())
			}
		}
		runDiagnosticTest(testURL)
		return
	}

	// Mode 2: NORMAL DOWNLOAD MODE
	fmt.Println("========================================")
	fmt.Println("       CLI MANGA/COMIC DOWNLOADER       ")
	fmt.Println("========================================")

	targetDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[!] Gagal membaca folder saat ini: %v\n", err)
		return
	}
	fmt.Printf("[*] Folder simpan: %s\n\n", targetDir)

	rawInput := ""
	if len(os.Args) > 1 {
		rawInput = strings.Join(os.Args[1:], " ")
	} else {
		fmt.Print("Masukkan URL Chapter (bisa multi-link dipisah spasi atau |): ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			rawInput = strings.TrimSpace(scanner.Text())
		}
	}

	urls := parseInputURLs(rawInput)
	if len(urls) == 0 {
		fmt.Println("[!] Tidak ada URL chapter yang valid.")
		return
	}

	isMulti := len(urls) > 1
	if isMulti {
		fmt.Printf("[+] Mode Multi-Link Terdeteksi (%d Chapter)\n\n", len(urls))
	}

	for idx, chapterURL := range urls {
		parsedURL, err := url.Parse(chapterURL)
		if err != nil || parsedURL.Host == "" {
			fmt.Printf("[!] [%d/%d] Format URL tidak valid: %s\n\n", idx+1, len(urls), chapterURL)
			continue
		}

		parser, err := parsers.FindParser(parsedURL.Host)
		if err != nil {
			fmt.Printf("[!] [%d/%d] %v\n\n", idx+1, len(urls), err)
			continue
		}

		if isMulti {
			fmt.Printf("==================================================\n")
			fmt.Printf("[%d/%d] Memproses Chapter: %s\n", idx+1, len(urls), chapterURL)
			fmt.Printf("[+] Parser: %s\n", parser.Name())
		} else {
			fmt.Printf("[+] Menggunakan Parser: %s\n", parser.Name())
		}

		fmt.Println("[*] Mengambil daftar halaman gambar...")
		imageURLs, err := parser.GetImages(chapterURL)
		if err != nil {
			fmt.Printf("[!] Gagal mengekstrak gambar: %v\n\n", err)
			continue
		}

		total := len(imageURLs)
		if total == 0 {
			fmt.Println("[!] Tidak ada gambar yang ditemukan.")
			continue
		}

		// Tentukan folder simpan
		saveDir := targetDir
		if isMulti {
			folderName := fmt.Sprintf("Chapter %d", idx+1)
			saveDir = filepath.Join(targetDir, folderName)
			if err := os.MkdirAll(saveDir, 0755); err != nil {
				fmt.Printf("[!] Gagal membuat folder '%s': %v\n\n", saveDir, err)
				continue
			}
			fmt.Printf("[+] Target Folder: %s\n", folderName)
		}

		fmt.Printf("[+] Ditemukan %d gambar. Memulai proses download...\n\n", total)
		downloadAll(imageURLs, saveDir, parsedURL.Host, parser)

		if isMulti {
			fmt.Printf("\n[✓] [%d/%d] Selesai diunduh!\n\n", idx+1, len(urls))
		} else {
			fmt.Println("\n[✓] Selesai! Semua gambar berhasil disimpan.")
		}
	}

	if isMulti {
		fmt.Println("==================================================")
		fmt.Println("[✓] SEMUA CHAPTER TELAH SELESAI DIUNDUH!")
		fmt.Println("==================================================")
	}
}

// parseInputURLs memisahkan input yang bisa berisi beberapa URL sekaligus
func parseInputURLs(rawInput string) []string {
	replacer := strings.NewReplacer("|", " ", ",", " ", "\n", " ", "\r", " ", "\t", " ")
	tokens := strings.Fields(replacer.Replace(rawInput))
	var urls []string
	for _, tok := range tokens {
		tok = strings.TrimSpace(tok)
		if strings.HasPrefix(tok, "http://") || strings.HasPrefix(tok, "https://") {
			urls = append(urls, tok)
		}
	}
	return urls
}

// runDiagnosticTest menjalankan diagnosa URL tanpa mendownload gambarnya
func runDiagnosticTest(rawURL string) {
	fmt.Println("==================================================")
	fmt.Printf("DIAGNOSA PARSER: %s\n", rawURL)
	fmt.Println("==================================================")

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		fmt.Println("[X] Format URL tidak valid!")
		return
	}

	// 1. Cek Router / Registry
	p, err := parsers.FindParser(parsed.Host)
	if err != nil {
		fmt.Printf("[X 1/4] Domain '%s' BELUM terdaftar di registry parsers!\n", parsed.Host)
		return
	}
	fmt.Printf("[✓ 1/4] Domain Match      : %s\n", p.Name())

	// 2. Cek Koneksi & Proteksi Anti-Bot
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", rawURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[X 2/4] Gagal menghubungi server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		fmt.Println("[X 2/4] DIBLOKIR (HTTP 403) - Terkena WAF / Cloudflare Anti-Bot!")
		return
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		fmt.Println("[X 2/4] DIBLOKIR (HTTP 503) - Cloudflare Turnstile / Under Attack Mode!")
		return
	} else if resp.StatusCode != http.StatusOK {
		fmt.Printf("[X 2/4] HTTP Error: Status %d %s\n", resp.StatusCode, resp.Status)
		return
	}
	fmt.Println("[✓ 2/4] Koneksi HTTP       : 200 OK (Akses aman, tidak terblokir)")

	// 3. Ekstraksi Gambar
	start := time.Now()
	images, err := p.GetImages(rawURL)
	dur := time.Since(start)

	if err != nil {
		fmt.Printf("[X 3/4] Ekstraksi GAGAL: %v\n", err)
		return
	}
	if len(images) == 0 {
		fmt.Println("[X 3/4] Ekstraksi GAGAL: 0 gambar ditemukan (Selector CSS atau script JSON berubah)!")
		return
	}
	fmt.Printf("[✓ 3/4] Ekstraksi Gambar  : Ditemukan %d gambar (waktu: %v)\n", len(images), dur)

	// 4. Validasi URL Gambar
	sampleURL := images[0]
	if idx := strings.Index(sampleURL, "#"); idx != -1 {
		sampleURL = sampleURL[:idx]
	}
	fmt.Printf("[✓ 4/4] Sampel Gambar     : %s\n", sampleURL)
	fmt.Println("==================================================")
	fmt.Printf("[SUKSES] Parser '%s' BERFUNGSI SEMPURNA!\n", p.Name())
}

func downloadAll(imageURLs []string, targetDir, host string, parser themes.Parser) {
	concurrency := 5
	guard := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	total := len(imageURLs)
	client := &http.Client{Timeout: 30 * time.Second}

	for i, rawURL := range imageURLs {
		wg.Add(1)
		guard <- struct{}{}

		go func(index int, imgURL string) {
			defer wg.Done()
			defer func() { <-guard }()

			ext := themes.GetFileExtension(imgURL)
			baseName := fmt.Sprintf("%03d", index+1)

			savedFileName, origName, w, h, err := downloadFile(client, imgURL, targetDir, baseName, ext, host, parser)
			if err != nil {
				fmt.Printf("[%02d/%02d] [GAGAL] %s: %v\n", index+1, total, origName, err)
			} else {
				dimStr := ""
				if w > 0 && h > 0 {
					dimStr = fmt.Sprintf(" (%dx%d)", w, h)
				}
				fmt.Printf("[%02d/%02d] [OK] %s -> %s%s\n", index+1, total, origName, savedFileName, dimStr)
			}
		}(i, rawURL)
	}

	wg.Wait()
}

func downloadFile(client *http.Client, fileURL, targetDir, baseName, defaultExt, host string, parser themes.Parser) (string, string, int, int, error) {
	fetchURL := fileURL
	if idx := strings.Index(fetchURL, "#"); idx != -1 {
		fetchURL = fetchURL[:idx]
	}

	// Ambil nama file asli dari URL
	origName := "image"
	if parsed, err := url.Parse(fetchURL); err == nil {
		b := filepath.Base(parsed.Path)
		if b != "" && b != "/" && b != "." {
			origName = b
		}
	}

	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return "", origName, 0, 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))

	resp, err := client.Do(req)
	if err != nil {
		return "", origName, 0, 0, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", origName, 0, 0, fmt.Errorf("status code %d", resp.StatusCode)
	}

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", origName, 0, 0, err
	}

	// 1. Jika parser memiliki logic ImageProcessor (seperti descrambler), jalankan prosesnya
	if processor, ok := parser.(themes.ImageProcessor); ok {
		processed, err := processor.ProcessImage(bodyBytes, fileURL)
		if err == nil {
			bodyBytes = processed
		}
	}

	// 2. Pastikan format akhir adalah JPG atau PNG:
	// Jika bukan JPG atau PNG (seperti WebP, GIF), otomatis convert ke JPG
	finalBytes, finalExt, err := themes.EnsureJPGorPNG(bodyBytes, defaultExt)
	if err == nil {
		bodyBytes = finalBytes
		defaultExt = finalExt
	}

	// 3. Baca dimensi gambar (lebar x tinggi)
	width, height := 0, 0
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(bodyBytes)); err == nil {
		width, height = cfg.Width, cfg.Height
	}

	fileName := fmt.Sprintf("%s%s", baseName, defaultExt)
	destPath := filepath.Join(targetDir, fileName)

	out, err := os.Create(destPath)
	if err != nil {
		return "", origName, width, height, err
	}
	defer out.Close()

	_, err = out.Write(bodyBytes)
	return fileName, origName, width, height, err
}
