package downloader

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/fetcher"
	"dmanga/internal/utils"
)

const (
	// DefaultConcurrency is the number of parallel image workers used when the user does not override it.
	DefaultConcurrency = 10
	// MaxConcurrency is the upper bound accepted for the worker count.
	MaxConcurrency = 32

	progressBarWidth = 16
	progressInterval = 100 * time.Millisecond
	minThrottleLimit = 2

	// Circuit breaker: if more than breakerMaxFailed of the first breakerSample finished images failed,
	// the remaining images of the chapter are skipped instead of grinding through every retry.
	breakerSample    = 10
	breakerMaxFailed = 5
)

// Options controls how DownloadAll runs.
type Options struct {
	// Concurrency is the number of parallel workers. Values <= 0 use DefaultConcurrency; values above MaxConcurrency are clamped.
	Concurrency int
	// Client is the HTTP client used for image requests. Defaults to fetcher.ImageClient().
	Client *http.Client
	// Out receives progress output. Defaults to os.Stdout.
	Out io.Writer
	// Force re-downloads images even when a completed file with the same name already exists.
	Force bool
}

// ClampConcurrency normalizes a requested worker count into the supported range.
func ClampConcurrency(n int) int {
	if n <= 0 {
		return DefaultConcurrency
	}
	if n > MaxConcurrency {
		return MaxConcurrency
	}
	return n
}

// limiter is a resizable semaphore. When new HTTP 429 events are observed it halves the allowed
// number of simultaneously active workers (never below minThrottleLimit) so the host can recover.
type limiter struct {
	mu       sync.Mutex
	cond     *sync.Cond
	limit    int
	floor    int
	active   int
	seenHits int64
}

func newLimiter(limit int) *limiter {
	l := &limiter{limit: limit, floor: minThrottleLimit, seenHits: fetcher.RateLimitHits()}
	if l.floor > limit {
		l.floor = limit
	}
	l.cond = sync.NewCond(&l.mu)
	return l
}

func (l *limiter) acquire() {
	l.mu.Lock()
	defer l.mu.Unlock()

	if hits := fetcher.RateLimitHits(); hits > l.seenHits {
		l.seenHits = hits
		l.limit /= 2
		if l.limit < l.floor {
			l.limit = l.floor
		}
	}
	for l.active >= l.limit {
		l.cond.Wait()
	}
	l.active++
}

func (l *limiter) release() {
	l.mu.Lock()
	l.active--
	l.mu.Unlock()
	l.cond.Signal()
}

type job struct {
	index int
	url   string
}

// DownloadAll downloads all images with a fixed worker pool and renders a single-line progress bar.
// Workers only update atomic counters; one renderer goroutine redraws the line on a timer, so no console
// I/O happens inside the hot path. It returns the number of completed and failed images.
func DownloadAll(imageURLs []string, targetDir, host string, parser core.Parser, opts Options) (completed, failed int) {
	total := len(imageURLs)
	if total == 0 {
		return 0, 0
	}

	concurrency := ClampConcurrency(opts.Concurrency)
	if concurrency > total {
		concurrency = total
	}
	client := opts.Client
	if client == nil {
		client = fetcher.ImageClient()
	}
	out := opts.Out
	if out == nil {
		out = os.Stdout
	}

	var completedCount, failedCount, totalBytes, finishedCount, skippedCount atomic.Int64
	var aborted atomic.Bool
	startTime := time.Now()
	failCh := make(chan string, total+1)

	draw := func() {
		done := int(completedCount.Load() + failedCount.Load())
		bytesNow := totalBytes.Load()
		elapsed := time.Since(startTime)
		speed := 0.0
		if elapsed.Seconds() > 0 {
			speed = float64(bytesNow) / elapsed.Seconds()
		}

		bar := RenderProgressBar(done, total, progressBarWidth)
		pct := (done * 100) / total
		countStr := fmt.Sprintf("%d/%d (%d%%)", done, total, pct)
		sizeStr := FormatBytes(bytesNow)
		speedStr := FormatSpeed(speed)

		var line string
		if done >= total {
			line = fmt.Sprintf("Downloading %s %s | %s | %s (%.1fs)", bar, countStr, sizeStr, speedStr, elapsed.Seconds())
		} else {
			etaStr := "ETA: --"
			if speed > 0 && done > 0 {
				avgItemTime := elapsed / time.Duration(done)
				secs := int((avgItemTime * time.Duration(total-done)).Seconds())
				if secs < 60 {
					etaStr = fmt.Sprintf("ETA: %ds", secs)
				} else {
					etaStr = fmt.Sprintf("ETA: %dm%02ds", secs/60, secs%60)
				}
			}
			line = fmt.Sprintf("Downloading %s %s | %s | %s | %s", bar, countStr, sizeStr, speedStr, etaStr)
		}
		fmt.Fprintf(out, "\r%-79s", line)
	}

	// Single renderer goroutine: owns all console output while downloading.
	stop := make(chan struct{})
	rendered := make(chan struct{})
	go func() {
		defer close(rendered)
		ticker := time.NewTicker(progressInterval)
		defer ticker.Stop()
		for {
			select {
			case msg := <-failCh:
				fmt.Fprintf(out, "\r%-79s\n", msg)
			case <-ticker.C:
				draw()
			case <-stop:
				for {
					select {
					case msg := <-failCh:
						fmt.Fprintf(out, "\r%-79s\n", msg)
					default:
						draw()
						fmt.Fprintln(out)
						return
					}
				}
			}
		}
	}()

	jobs := make(chan job, total)
	for i, u := range imageURLs {
		jobs <- job{index: i, url: u}
	}
	close(jobs)

	lim := newLimiter(concurrency)
	var wg sync.WaitGroup
	for w := 0; w < concurrency; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := range jobs {
				if aborted.Load() || fetcher.Context().Err() != nil {
					failedCount.Add(1)
					continue
				}

				ext := utils.GetFileExtension(j.url)
				baseName := utils.FormatImageIndex(j.index+1, total)

				if !opts.Force {
					if _, exists := ExistingImage(targetDir, baseName, ext); exists {
						completedCount.Add(1)
						skippedCount.Add(1)
						continue
					}
				}

				lim.acquire()

				_, size, err := DownloadFile(client, j.url, targetDir, baseName, ext, host, parser)

				lim.release()

				if err != nil {
					failedCount.Add(1)
					if fetcher.Context().Err() == nil {
						failCh <- fmt.Sprintf("[!] [FAILED] %s: %v", baseName+ext, err)
					}
				} else {
					completedCount.Add(1)
					totalBytes.Add(size)
				}

				if finishedCount.Add(1) == breakerSample && total > breakerSample && failedCount.Load() > breakerMaxFailed {
					if aborted.CompareAndSwap(false, true) {
						failCh <- fmt.Sprintf("[!] %d of the first %d images failed; skipping the remaining images of this chapter", failedCount.Load(), breakerSample)
					}
				}
			}
		}()
	}

	wg.Wait()
	close(stop)
	<-rendered

	completed, failed = int(completedCount.Load()), int(failedCount.Load())
	if n := skippedCount.Load(); n > 0 {
		fmt.Fprintf(out, "[*] Skipped %d image(s) that already exist (use --force to re-download).\n", n)
	}
	if fetcher.Context().Err() != nil {
		fmt.Fprintln(out, "[!] Interrupted by user.")
	}
	if failed > 0 {
		fmt.Fprintf(out, "[!] Warning: %d out of %d images failed to download.\n", failed, total)
	}
	return completed, failed
}
