package main

import (
	"bufio"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"manga-downloader/core"
	_ "manga-downloader/parsers"
)

func main() {
	if len(os.Args) >= 2 {
		arg := strings.ToLower(os.Args[1])

		// Mode 1: LIST SUPPORTED PARSERS (-l, --list, list, parsers)
		if arg == "-l" || arg == "--list" || arg == "list" || arg == "-p" || arg == "--parsers" || arg == "parsers" {
			core.ShowSupportedParsers()
			return
		}

		// Mode 2: HELP (-h, --help, help)
		if arg == "-h" || arg == "--help" || arg == "help" {
			core.ShowHelp()
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
			core.RunDiagnosticTest(testURL)
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

	urls := core.ParseInputURLs(rawInput)
	if len(urls) == 0 {
		fmt.Println("[!] No valid chapter URL provided.")
		return
	}

	isMulti := len(urls) > 1
	if isMulti {
		fmt.Printf("[+] Multi-Link Mode Detected (%d Chapters)\n\n", len(urls))
	}

	usedFolders := make(map[string]int)

	for idx, chapterURL := range urls {
		parsedURL, err := url.Parse(chapterURL)
		if err != nil || parsedURL.Host == "" {
			fmt.Printf("[!] [%d/%d] Invalid URL format: %s\n\n", idx+1, len(urls), chapterURL)
			continue
		}

		parser, err := core.FindParser(parsedURL.Host)
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

		fmt.Println("[*] Fetching chapter information & image URLs...")
		chap, err := core.FetchChapter(parser, chapterURL)
		if err != nil {
			fmt.Printf("[!] Failed to extract chapter: %v\n\n", err)
			continue
		}

		imageURLs := chap.Images
		total := len(imageURLs)
		if total == 0 {
			fmt.Println("[!] No images found.")
			continue
		}

		if chap.Title != "" {
			fmt.Printf("[+] Chapter: %s\n", chap.Title)
		}

		// Determine destination folder
		saveDir := targetDir
		if isMulti {
			folderName := chap.Title
			if folderName == "" {
				folderName = fmt.Sprintf("Ch %d", idx+1)
			}

			// Prevent duplicate folder collisions
			if count, exists := usedFolders[strings.ToLower(folderName)]; exists {
				usedFolders[strings.ToLower(folderName)] = count + 1
				folderName = fmt.Sprintf("%s (%d)", folderName, count+1)
			} else {
				usedFolders[strings.ToLower(folderName)] = 1
			}

			saveDir = filepath.Join(targetDir, folderName)
			if err := os.MkdirAll(saveDir, 0755); err != nil {
				fmt.Printf("[!] Failed to create folder '%s': %v\n\n", saveDir, err)
				continue
			}
			fmt.Printf("[+] Target Folder: %s\n", folderName)
		}

		fmt.Printf("[+] Found %d images. Starting download...\n\n", total)
		core.DownloadAll(imageURLs, saveDir, parsedURL.Host, parser)

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
