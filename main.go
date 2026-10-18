package main

import (
	"bufio"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"manga-downloader/parsers"
	"manga-downloader/themes"
)

func main() {
	fmt.Println("========================================")
	fmt.Println("       CLI MANGA/COMIC DOWNLOADER       ")
	fmt.Println("========================================")

	targetDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[!] Gagal membaca folder saat ini: %v\n", err)
		return
	}
	fmt.Printf("[*] Folder simpan: %s\n\n", targetDir)

	// Ambil input URL chapter dari argumen atau prompt
	chapterURL := ""
	if len(os.Args) > 1 {
		chapterURL = os.Args[1]
	} else {
		fmt.Print("Masukkan URL Chapter: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			chapterURL = strings.TrimSpace(scanner.Text())
		}
	}

	if chapterURL == "" {
		fmt.Println("[!] URL tidak boleh kosong.")
		return
	}

	parsedURL, err := url.Parse(chapterURL)
	if err != nil || parsedURL.Host == "" {
		fmt.Println("[!] Format URL tidak valid.")
		return
	}

	// Cari parser berdasarkan domain
	parser, err := parsers.FindParser(parsedURL.Host)
	if err != nil {
		fmt.Printf("[!] %v\n", err)
		return
	}

	fmt.Printf("[+] Menggunakan Parser: %s\n", parser.Name())
	fmt.Println("[*] Mengambil daftar halaman gambar...")

	imageURLs, err := parser.GetImages(chapterURL)
	if err != nil {
		fmt.Printf("[!] Gagal mengekstrak gambar: %v\n", err)
		return
	}

	total := len(imageURLs)
	if total == 0 {
		fmt.Println("[!] Tidak ada gambar yang ditemukan.")
		return
	}

	fmt.Printf("[+] Ditemukan %d gambar. Memulai proses download...\n\n", total)

	// Download paralel dengan 5 worker
	downloadAll(imageURLs, targetDir, parsedURL.Host)

	fmt.Println("\n[✓] Selesai! Semua gambar berhasil disimpan.")
}

func downloadAll(imageURLs []string, targetDir, host string) {
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
			fileName := fmt.Sprintf("%03d%s", index+1, ext)
			filePath := filepath.Join(targetDir, fileName)

			err := downloadFile(client, imgURL, filePath, host)
			if err != nil {
				fmt.Printf("[%02d/%02d] [GAGAL] %s: %v\n", index+1, total, fileName, err)
			} else {
				fmt.Printf("[%02d/%02d] [OK] Tersimpan: %s\n", index+1, total, fileName)
			}
		}(i, rawURL)
	}

	wg.Wait()
}

func downloadFile(client *http.Client, fileURL, destPath, host string) error {
	req, err := http.NewRequest("GET", fileURL, nil)
	if err != nil {
		return err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))

	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("status code %d", resp.StatusCode)
	}

	out, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer out.Close()

	_, err = io.Copy(out, resp.Body)
	return err
}
