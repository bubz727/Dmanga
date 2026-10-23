package cli

import (
	"bufio"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	"dmanga/internal/downloader"
	"dmanga/internal/fetcher"
)

// Run executes the CLI command workflow based on provided args
func Run(args []string) {
	if len(args) >= 2 {
		arg := strings.ToLower(args[1])

		// Mode 1: LIST SUPPORTED PARSERS (-l, --list, list, parsers)
		if arg == "-l" || arg == "--list" || arg == "list" || arg == "-p" || arg == "--parsers" || arg == "parsers" {
			ShowSupportedParsers()
			return
		}

		// Mode 2: VERSION (-v, --version, version)
		if arg == "-v" || arg == "--version" || arg == "version" {
			if len(args) >= 3 && (strings.ToLower(args[2]) == "--verbose" || strings.ToLower(args[2]) == "-v") {
				ShowVerboseVersion()
				return
			}
			ShowVersion()
			return
		}

		// Mode 3: HELP (-h, --help, help)
		if arg == "-h" || arg == "--help" || arg == "help" {
			ShowHelp()
			return
		}

		// Mode 4: CHECK UPDATE (--check-update, check-update)
		if arg == "--check-update" || arg == "check-update" {
			CheckUpdateExplicit()
			return
		}

		// Mode 5: SELF-UPDATE (update, --update)
		if arg == "update" || arg == "--update" {
			if len(args) >= 3 && (strings.ToLower(args[2]) == "--check" || strings.ToLower(args[2]) == "-c" || strings.ToLower(args[2]) == "check") {
				CheckUpdateExplicit()
				return
			}
			ExecuteSelfUpdate()
			return
		}

		// Mode 6: DIAGNOSTIC TEST MODE (-t or --test)
		if arg == "-t" || arg == "--test" {
			testURL := ""
			if len(args) >= 3 {
				testURL = args[2]
			} else {
				fmt.Print("Enter URL to test: ")
				scanner := bufio.NewScanner(os.Stdin)
				if scanner.Scan() {
					testURL = strings.TrimSpace(scanner.Text())
				}
			}
			RunDiagnosticTest(testURL)
			return
		}
	}

	// Mode 4: NORMAL DOWNLOAD MODE
	fmt.Println()

	targetDir, err := os.Getwd()
	if err != nil {
		fmt.Printf("[!] Failed to get current working directory: %v\n", err)
		return
	}
	fmt.Printf("[*] Save directory: %s\n\n", targetDir)

	hasFolderFlag := HasFolderFlag(args...)
	noCheck := HasNoUpdateCheckFlag(args...)
	concurrencyVal, concurrencyFound := ParseConcurrencyFlag(args...)
	prefetchVal, prefetchFound := ParsePrefetchFlag(args...)
	updateCh := StartPassiveUpdateCheck(noCheck)

	rawInput := ""
	if len(args) > 1 {
		rawInput = strings.Join(args[1:], " ")
	}

	urls := ParseInputURLs(rawInput)
	if len(urls) == 0 {
		fmt.Print("Enter Chapter URL: ")
		scanner := bufio.NewScanner(os.Stdin)
		if scanner.Scan() {
			promptInput := strings.TrimSpace(scanner.Text())
			if HasFolderFlag(promptInput) {
				hasFolderFlag = true
			}
			if HasNoUpdateCheckFlag(promptInput) {
				noCheck = true
			}
			if !concurrencyFound {
				concurrencyVal, concurrencyFound = ParseConcurrencyFlag(promptInput)
			}
			if !prefetchFound {
				prefetchVal, prefetchFound = ParsePrefetchFlag(promptInput)
			}
			urls = ParseInputURLs(promptInput)
		}
	}

	if len(urls) == 0 {
		fmt.Println("[!] No valid chapter URL provided.")
		return
	}

	concurrency, concurrencyWarn := ResolveConcurrency(concurrencyVal, concurrencyFound)
	if concurrencyWarn != "" {
		fmt.Printf("[!] %s\n", concurrencyWarn)
	}

	prefetch, prefetchWarn := ResolvePrefetch(prefetchVal, prefetchFound)
	if prefetchWarn != "" {
		fmt.Printf("[!] %s\n", prefetchWarn)
	}

	isMulti := len(urls) > 1
	createFolder := isMulti || hasFolderFlag
	if isMulti {
		fmt.Printf("[+] Multi-Link Mode Detected (%d Chapters)\n\n", len(urls))
	} else if hasFolderFlag {
		fmt.Printf("[+] Subfolder Mode Enabled (-f)\n\n")
	}

	usedFolders := make(map[string]int)

	targets := make([]ChapterTarget, len(urls))
	for i, u := range urls {
		targets[i] = resolveTarget(u)
	}
	pf := NewPrefetcher(targets, prefetch, nil)
	defer pf.Close()

	for idx, chapterURL := range urls {
		if fetcher.Context().Err() != nil {
			fmt.Println("\n[!] Interrupted by user. Remaining chapters were skipped.")
			break
		}
		target := targets[idx]
		if target.Err != nil {
			if errors.Is(target.Err, errInvalidURL) {
				fmt.Printf("[!] [%d/%d] Invalid URL format: %s\n\n", idx+1, len(urls), chapterURL)
			} else {
				fmt.Printf("[!] [%d/%d] %v\n\n", idx+1, len(urls), target.Err)
			}
			continue
		}
		parsedURL := &url.URL{Host: target.Host}
		p := target.Parser

		if isMulti {
			fmt.Println()
			fmt.Printf("[%d/%d] Processing Chapter: %s\n", idx+1, len(urls), chapterURL)
			fmt.Printf("[+] Parser: %s\n", p.Name())
		} else {
			fmt.Printf("[+] Using Parser: %s\n", p.Name())
		}

		fmt.Println("[*] Fetching chapter information & image URLs...")
		chap, err := pf.Get(idx)
		if err != nil {
			fmt.Printf("[!] Failed to extract chapter: %v\n\n", err)
			continue
		}

		// This host is now known to work (any Cloudflare solve already happened in the foreground),
		// so upcoming chapters can have their metadata fetched while this one downloads.
		pf.MarkWarmed(target.Host)
		pf.Kick(idx)

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
		if createFolder {
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
		downloader.DownloadAll(imageURLs, saveDir, parsedURL.Host, p, downloader.Options{Concurrency: concurrency})

		if isMulti {
			fmt.Printf("\n[OK] [%d/%d] Chapter download complete!\n\n", idx+1, len(urls))
		} else {
			fmt.Println("\n[OK] All images saved successfully.")
		}
	}

	if isMulti {
		fmt.Println()
		fmt.Println("Downloads Finished.")
	}

	// Display update banner if passive check completed before exit
	select {
	case info := <-updateCh:
		PrintUpdateBanner(info)
	default:
	}
}
