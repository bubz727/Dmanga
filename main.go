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
	"sort"
	"strings"
	"sync"
	"time"

	_ "golang.org/x/image/webp"

	"manga-downloader/parsers"
	"manga-downloader/themes"
)

func main() {
	if len(os.Args) >= 2 {
		arg := strings.ToLower(os.Args[1])

		// Mode 1: LIST SUPPORTED PARSERS (-l, --list, list, parsers)
		if arg == "-l" || arg == "--list" || arg == "list" || arg == "-p" || arg == "--parsers" || arg == "parsers" {
			showSupportedParsers()
			return
		}

		// Mode 2: HELP (-h, --help, help)
		if arg == "-h" || arg == "--help" || arg == "help" {
			showHelp()
			return
		}

		// Mode 3: DIAGNOSTIC TEST MODE (-t or --test)
		if arg == "-t" || arg == "--test" {
			testURL := ""
			if len(os.Args) >= 3 {
				testURL = os.Args[2]
			} else {
				fmt.Print("Enter URL to test: ")
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					testURL = strings.TrimSpace(scanner.Text())
				}
			}
			runDiagnosticTest(testURL)
			return
		}
	}

	// Mode 4: NORMAL DOWNLOAD MODE
	fmt.Println("========================================")
	fmt.Println("       CLI MANGA/COMIC DOWNLOADER       ")
	fmt.Println("========================================")

	targetDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[!] Failed to get current working directory: %v\n", err)
		return
	}
	fmt.Printf("[*] Save directory: %s\n\n", targetDir)

	rawInput := ""
	if len(os.Args) > 1 {
		rawInput = strings.Join(os.Args[1:], " ")
	} else {
		fmt.Print("Enter Chapter URL (supports multiple URLs separated by space or |): ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			rawInput = strings.TrimSpace(scanner.Text())
		}
	}

	urls := parseInputURLs(rawInput)
	if len(urls) == 0 {
		fmt.Println("[!] No valid chapter URL provided.")
		return
	}

	isMulti := len(urls) > 1
	if isMulti {
		fmt.Printf("[+] Multi-Link Mode Detected (%d Chapters)\n\n", len(urls))
	}

	for idx, chapterURL := range urls {
		parsedURL, err := url.Parse(chapterURL)
		if err != nil || parsedURL.Host == "" {
			fmt.Printf("[!] [%d/%d] Invalid URL format: %s\n\n", idx+1, len(urls), chapterURL)
			continue
		}

		parser, err := parsers.FindParser(parsedURL.Host)
		if err != nil {
			fmt.Printf("[!] [%d/%d] %v\n\n", idx+1, len(urls), err)
			continue
		}

		if isMulti {
			fmt.Printf("==================================================\n")
			fmt.Printf("[%d/%d] Processing Chapter: %s\n", idx+1, len(urls), chapterURL)
			fmt.Printf("[+] Parser: %s\n", parser.Name())
		} else {
			fmt.Printf("[+] Using Parser: %s\n", parser.Name())
		}

		fmt.Println("[*] Fetching chapter image URLs...")
		imageURLs, err := parser.GetImages(chapterURL)
		if err != nil {
			fmt.Printf("[!] Failed to extract images: %v\n\n", err)
			continue
		}

		total := len(imageURLs)
		if total == 0 {
			fmt.Println("[!] No images found.")
			continue
		}

		// Determine destination folder
		saveDir := targetDir
		if isMulti {
			folderName := fmt.Sprintf("Chapter %d", idx+1)
			saveDir = filepath.Join(targetDir, folderName)
			if err := os.MkdirAll(saveDir, 0755); err != nil {
				fmt.Printf("[!] Failed to create folder '%s': %v\n\n", saveDir, err)
				continue
			}
			fmt.Printf("[+] Target Folder: %s\n", folderName)
		}

		fmt.Printf("[+] Found %d images. Starting download...\n\n", total)
		downloadAll(imageURLs, saveDir, parsedURL.Host, parser)

		if isMulti {
			fmt.Printf("\n[OK] [%d/%d] Chapter download complete!\n\n", idx+1, len(urls))
		} else {
			fmt.Println("\n[OK] Complete! All images saved successfully.")
		}
	}

	if isMulti {
		fmt.Println("==================================================")
		fmt.Println("[OK] ALL CHAPTER DOWNLOADS FINISHED!")
		fmt.Println("==================================================")
	}
}

// parseInputURLs splits an input string containing one or more URLs
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

// runDiagnosticTest performs diagnostics on a URL without downloading images
func runDiagnosticTest(rawURL string) {
	fmt.Println("==================================================")
	fmt.Printf("PARSER DIAGNOSTIC: %s\n", rawURL)
	fmt.Println("==================================================")

	parsed, err := url.Parse(rawURL)
	if err != nil || parsed.Host == "" {
		fmt.Println("[X] Invalid URL format!")
		return
	}

	// 1. Check Router / Registry
	p, err := parsers.FindParser(parsed.Host)
	if err != nil {
		fmt.Printf("[X 1/4] Domain '%s' is NOT registered in parser registry!\n", parsed.Host)
		return
	}
	fmt.Printf("[OK 1/4] Domain Match      : %s\n", p.Name())

	// 2. Check Connection & Anti-Bot Protection
	client := &http.Client{Timeout: 15 * time.Second}
	req, _ := http.NewRequest("GET", rawURL, nil)
	req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/124.0.0.0 Safari/537.36")
	req.Header.Set("Referer", fmt.Sprintf("https://%s/", parsed.Host))

	resp, err := client.Do(req)
	if err != nil {
		fmt.Printf("[X 2/4] Failed to connect to server: %v\n", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusForbidden {
		fmt.Println("[X 2/4] BLOCKED (HTTP 403) - Anti-Bot / Cloudflare WAF detected!")
		return
	} else if resp.StatusCode == http.StatusServiceUnavailable {
		fmt.Println("[X 2/4] BLOCKED (HTTP 503) - Cloudflare Turnstile / Under Attack Mode!")
		return
	} else if resp.StatusCode != http.StatusOK {
		fmt.Printf("[X 2/4] HTTP Error: Status %d %s\n", resp.StatusCode, resp.Status)
		return
	}
	fmt.Println("[OK 2/4] HTTP Connection   : 200 OK (Clean access, unblocked)")

	// 3. Image Extraction
	start := time.Now()
	images, err := p.GetImages(rawURL)
	dur := time.Since(start)

	if err != nil {
		fmt.Printf("[X 3/4] Extraction FAILED: %v\n", err)
		return
	}
	if len(images) == 0 {
		fmt.Println("[X 3/4] Extraction FAILED: 0 images found (CSS selector or script structure may have changed)!")
		return
	}
	fmt.Printf("[OK 3/4] Image Extraction  : Found %d images (time: %v)\n", len(images), dur)

	// 4. Sample Image URL Validation
	sampleURL := images[0]
	if idx := strings.Index(sampleURL, "#"); idx != -1 {
		sampleURL = sampleURL[:idx]
	}
	fmt.Printf("[OK 4/4] Sample Image URL  : %s\n", sampleURL)
	fmt.Println("==================================================")
	fmt.Printf("[SUCCESS] Parser '%s' IS FULLY FUNCTIONAL!\n", p.Name())
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

func downloadFile(client *http.Client, fileURL, targetDir, baseName, defaultExt, host string, parser themes.Parser) (string, string, int, int, error) {
	fetchURL := fileURL
	if idx := strings.Index(fetchURL, "#"); idx != -1 {
		fetchURL = fetchURL[:idx]
	}

	// Extract original file name from URL
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

	// 1. Process image with ImageProcessor (e.g. descrambler) if supported
	if processor, ok := parser.(themes.ImageProcessor); ok {
		processed, err := processor.ProcessImage(bodyBytes, fileURL)
		if err == nil {
			bodyBytes = processed
		}
	}

	// 2. Ensure final format is JPG or PNG (convert WebP, GIF, etc. to JPG)
	finalBytes, finalExt, err := themes.EnsureJPGorPNG(bodyBytes, defaultExt)
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

func showSupportedParsers() {
	all := parsers.GetAllParsers()
	sort.Slice(all, func(i, j int) bool {
		return strings.ToLower(all[i].Name()) < strings.ToLower(all[j].Name())
	})

	totalDomains := 0
	for _, p := range all {
		totalDomains += len(p.Domains())
	}

	fmt.Println("==================================================")
	fmt.Println("            SUPPORTED MANGA PARSERS               ")
	fmt.Println("==================================================")
	for idx, p := range all {
		domains := p.Domains()
		sort.Strings(domains)
		fmt.Printf("[%02d] %s (%d domain%s)\n", idx+1, p.Name(), len(domains), pluralS(len(domains)))

		if len(domains) <= 5 {
			fmt.Printf("     %s\n", strings.Join(domains, ", "))
		} else {
			line := "     "
			for dIdx, d := range domains {
				if len(line)+len(d)+2 > 75 {
					fmt.Println(strings.TrimRight(line, " "))
					line = "     "
				}
				line += d
				if dIdx < len(domains)-1 {
					line += ", "
				}
			}
			if strings.TrimSpace(line) != "" {
				fmt.Println(line)
			}
		}
		fmt.Println()
	}
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Total: %d Parsers, %d Supported Domains\n", len(all), totalDomains)
	fmt.Println("==================================================")
	fmt.Println("Usage Commands:")
	fmt.Println("  Dmanga <chapter-url>                  Download chapter")
	fmt.Println("  Dmanga <url-1> <url-2>                Multi-chapter download")
	fmt.Println("  Dmanga -t <chapter-url>               Run parser diagnostics")
	fmt.Println("  Dmanga -l                             List all supported parsers")
	fmt.Println("==================================================")
}

func showHelp() {
	fmt.Println("==================================================")
	fmt.Println("            CLI MANGA DOWNLOADER HELP             ")
	fmt.Println("==================================================")
	fmt.Println("Commands & Flags:")
	fmt.Println("  -l, --list, list        Display all supported parsers and domains")
	fmt.Println("  -t, --test <url>        Run diagnostic checks on a URL")
	fmt.Println("  -h, --help, help        Show this help message")
	fmt.Println()
	fmt.Println("Download Usage:")
	fmt.Println("  Dmanga <url>              Single chapter download")
	fmt.Println("  Dmanga <url1> <url2> ...  Multi-link download (folders: Chapter 1, Chapter 2, ...)")
	fmt.Println("  Dmanga \"<url1> | <url2>\"  Pipe-separated multi-link download")
	fmt.Println("  Dmanga                    Interactive prompt mode")
	fmt.Println("==================================================")
}

func pluralS(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
