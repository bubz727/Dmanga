package downloader

import (
	"fmt"
	"strings"
	"sync"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

// FormatBytes converts raw bytes into a human-readable string (e.g. 512 B, 1.4 MB)
func FormatBytes(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatSpeed returns a human-readable download rate (e.g. 14.2 MB/s)
func FormatSpeed(bytesPerSec float64) string {
	if bytesPerSec <= 0 {
		return "0 B/s"
	}
	return FormatBytes(int64(bytesPerSec)) + "/s"
}

// RenderProgressBar generates an ASCII progress bar (e.g. [=========>     ])
func RenderProgressBar(current, total, width int) string {
	if total <= 0 || width <= 0 {
		return "[" + strings.Repeat(" ", width) + "]"
	}
	filled := (current * width) / total
	if filled >= width {
		return "[" + strings.Repeat("=", width) + "]"
	}
	if filled == 0 {
		if current > 0 {
			return "[>" + strings.Repeat(" ", width-1) + "]"
		}
		return "[" + strings.Repeat(" ", width) + "]"
	}
	return "[" + strings.Repeat("=", filled-1) + ">" + strings.Repeat(" ", width-filled) + "]"
}

// DownloadAll coordinates concurrent image downloading using a worker pool and renders a single-line progress bar
func DownloadAll(imageURLs []string, targetDir, host string, parser core.Parser) {
	total := len(imageURLs)
	if total == 0 {
		return
	}

	concurrency := 5
	guard := make(chan struct{}, concurrency)
	var wg sync.WaitGroup

	client := fetcher.ImageClient()

	var mu sync.Mutex
	completedCount := 0
	failedCount := 0
	var totalBytes int64
	startTime := time.Now()
	barWidth := 16

	// Render initial progress bar at 0%
	initBar := RenderProgressBar(0, total, barWidth)
	initLine := fmt.Sprintf("Downloading %s 0/%d (0%%) | 0 B | 0 B/s", initBar, total)
	fmt.Printf("\r%-79s", initLine)

	for i, rawURL := range imageURLs {
		wg.Add(1)
		guard <- struct{}{}

		go func(index int, imgURL string) {
			defer wg.Done()
			defer func() { <-guard }()

			ext := utils.GetFileExtension(imgURL)
			baseName := utils.FormatImageIndex(index+1, total)

			_, size, err := DownloadFile(client, imgURL, targetDir, baseName, ext, host, parser)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				failedCount++
				fmt.Printf("\r%-79s\n", fmt.Sprintf("[!] [FAILED] %s: %v", baseName+ext, err))
			} else {
				completedCount++
				totalBytes += size
			}

			finishedCount := completedCount + failedCount
			elapsed := time.Since(startTime)
			speed := 0.0
			if elapsed.Seconds() > 0 {
				speed = float64(totalBytes) / elapsed.Seconds()
			}

			bar := RenderProgressBar(finishedCount, total, barWidth)
			pct := (finishedCount * 100) / total
			countStr := fmt.Sprintf("%d/%d (%d%%)", finishedCount, total, pct)
			sizeStr := FormatBytes(totalBytes)
			speedStr := FormatSpeed(speed)

			var line string
			if finishedCount >= total {
				line = fmt.Sprintf("Downloading %s %s | %s | %s (%.1fs)", bar, countStr, sizeStr, speedStr, elapsed.Seconds())
			} else {
				etaStr := "ETA: --"
				if speed > 0 && finishedCount > 0 {
					remainingItems := total - finishedCount
					avgItemTime := elapsed / time.Duration(finishedCount)
					secs := int((avgItemTime * time.Duration(remainingItems)).Seconds())
					if secs < 60 {
						etaStr = fmt.Sprintf("ETA: %ds", secs)
					} else {
						etaStr = fmt.Sprintf("ETA: %dm%02ds", secs/60, secs%60)
					}
				}
				line = fmt.Sprintf("Downloading %s %s | %s | %s | %s", bar, countStr, sizeStr, speedStr, etaStr)
			}

			fmt.Printf("\r%-79s", line)
		}(i, rawURL)
	}

	wg.Wait()
	fmt.Println()

	if failedCount > 0 {
		fmt.Printf("[!] Warning: %d out of %d images failed to download.\n", failedCount, total)
	}
}
