package downloader

import (
	"fmt"
	"net/http"
	"sync"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/utils"
)

// DownloadAll coordinates concurrent image downloading using a worker pool
func DownloadAll(imageURLs []string, targetDir, host string, parser core.Parser) {
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

			ext := utils.GetFileExtension(imgURL)
			baseName := utils.FormatImageIndex(index+1, total)

			savedFileName, origName, w, h, err := DownloadFile(client, imgURL, targetDir, baseName, ext, host, parser)
			idxStr := utils.FormatImageIndex(index+1, total)
			totStr := utils.FormatImageIndex(total, total)
			if err != nil {
				fmt.Printf("[%s/%s] [FAILED] %s: %v\n", idxStr, totStr, origName, err)
			} else {
				dimStr := ""
				if w > 0 && h > 0 {
					dimStr = fmt.Sprintf(" (%dx%d)", w, h)
				}
				fmt.Printf("[%s/%s] [OK] %s -> %s%s\n", idxStr, totStr, origName, savedFileName, dimStr)
			}
		}(i, rawURL)
	}

	wg.Wait()
}
