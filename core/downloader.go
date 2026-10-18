package core

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
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"
)

// DownloadAll coordinates concurrent image downloading using a worker pool
func DownloadAll(imageURLs []string, targetDir, host string, parser Parser) {
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

			ext := GetFileExtension(imgURL)
			baseName := fmt.Sprintf("%03d", index+1)

			savedFileName, origName, w, h, err := DownloadFile(client, imgURL, targetDir, baseName, ext, host, parser)
			if err != nil {
				fmt.Printf("[%02d/%02d] [FAILED] %s: %v\n", index+1, total, origName, err)
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

// DownloadFile downloads a single image, processes it (descrambling/format conversion), and saves it to disk
func DownloadFile(client *http.Client, fileURL, targetDir, baseName, defaultExt, host string, parser Parser) (string, string, int, int, error) {
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

	req, err := http.NewRequest("GET", fetchURL, nil)
	if err != nil {
		return "", origName, 0, 0, err
	}

	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", host))
	ApplySessionHeaders(req, host)

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

	// 1. Process image with ImageProcessor (e.g. descrambler) if supported
	if processor, ok := parser.(ImageProcessor); ok {
		processed, err := processor.ProcessImage(bodyBytes, fileURL)
		if err == nil {
			bodyBytes = processed
		}
	}

	// 2. Ensure final format is JPG or PNG (convert WebP, GIF, etc. to JPG)
	finalBytes, finalExt, err := EnsureJPGorPNG(bodyBytes, defaultExt)
	if err == nil {
		bodyBytes = finalBytes
		defaultExt = finalExt
	}

	// 3. Read image dimensions (width x height)
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
