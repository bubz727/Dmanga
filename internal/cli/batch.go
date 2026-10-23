package cli

import (
	"errors"
	"net/url"
	"sync"
	"time"

	"dmanga/internal/core"
	"dmanga/internal/parser"
)

const (
	// DefaultPrefetch is how many upcoming chapters have their metadata fetched while the current one downloads.
	DefaultPrefetch = 2
	// MaxPrefetch is the upper bound accepted for the prefetch lookahead.
	MaxPrefetch = 5

	// prefetchSpacing is the minimum gap between two metadata requests issued by the prefetcher,
	// so prefetching does not itself trigger rate limiting.
	prefetchSpacing = 150 * time.Millisecond
)

// errInvalidURL marks a chapter URL that cannot be parsed or has no host.
var errInvalidURL = errors.New("invalid URL format")

// ChapterTarget is a chapter URL resolved to its host and parser (no network access needed).
type ChapterTarget struct {
	URL    string
	Host   string
	Parser core.Parser
	Err    error
}

// resolveTarget parses the URL and finds its parser. It never touches the network.
func resolveTarget(chapterURL string) ChapterTarget {
	t := ChapterTarget{URL: chapterURL}
	parsed, err := url.Parse(chapterURL)
	if err != nil || parsed.Host == "" {
		t.Err = errInvalidURL
		return t
	}
	t.Host = parsed.Host
	p, err := parser.FindParser(parsed.Host)
	if err != nil {
		t.Err = err
		return t
	}
	t.Parser = p
	return t
}

type fetchResult struct {
	chapter *core.Chapter
	err     error
}

type fetchTask struct {
	target ChapterTarget
	done   chan struct{}
	result fetchResult
}

// Prefetcher fetches chapter metadata for upcoming chapters in the background while the current chapter downloads.
//
// Safety rules:
//   - Only chapters whose host has already been fetched successfully by the main flow are prefetched.
//     The first chapter of every host is always fetched synchronously, so an interactive Cloudflare solve
//     happens in the foreground, never behind the progress bar.
//   - A single worker goroutine runs all prefetches in order with a small gap between requests.
//   - Results are consumed strictly in input order by the caller, so folder naming and output stay deterministic.
type Prefetcher struct {
	lookahead int
	targets   []ChapterTarget
	fetch     ChapterFetchFunc

	mu      sync.Mutex
	tasks   chan *fetchTask
	pending map[int]*fetchTask
	warmed  map[string]bool
	wg      sync.WaitGroup
}

// ChapterFetchFunc fetches chapter metadata for a parser and URL (core.FetchChapter by default).
type ChapterFetchFunc func(p core.Parser, chapterURL string) (*core.Chapter, error)

// NewPrefetcher creates a prefetcher over the given targets. A nil fetch uses core.FetchChapter.
func NewPrefetcher(targets []ChapterTarget, lookahead int, fetch ChapterFetchFunc) *Prefetcher {
	if fetch == nil {
		fetch = core.FetchChapter
	}
	if lookahead < 0 {
		lookahead = 0
	}
	if lookahead > MaxPrefetch {
		lookahead = MaxPrefetch
	}
	pf := &Prefetcher{
		lookahead: lookahead,
		targets:   targets,
		fetch:     fetch,
		tasks:     make(chan *fetchTask, MaxPrefetch+1),
		pending:   make(map[int]*fetchTask),
		warmed:    make(map[string]bool),
	}
	if lookahead > 0 {
		pf.wg.Add(1)
		go pf.worker()
	}
	return pf
}

func (pf *Prefetcher) worker() {
	defer pf.wg.Done()
	first := true
	for t := range pf.tasks {
		if !first {
			time.Sleep(prefetchSpacing)
		}
		first = false
		chap, err := pf.fetch(t.target.Parser, t.target.URL)
		t.result = fetchResult{chapter: chap, err: err}
		close(t.done)
	}
}

// Get returns the chapter for target idx. It uses a finished or in-flight prefetch when available and
// otherwise fetches synchronously. A failed prefetch is retried once in the foreground.
func (pf *Prefetcher) Get(idx int) (*core.Chapter, error) {
	t := pf.targets[idx]

	pf.mu.Lock()
	task := pf.pending[idx]
	delete(pf.pending, idx)
	pf.mu.Unlock()

	if task != nil {
		<-task.done
		if task.result.err == nil {
			return task.result.chapter, nil
		}
		// Fall through to one synchronous retry
	}

	return pf.fetch(t.Parser, t.URL)
}

// MarkWarmed records that a host has been fetched successfully, which makes later chapters of that host
// eligible for prefetching.
func (pf *Prefetcher) MarkWarmed(host string) {
	pf.mu.Lock()
	pf.warmed[host] = true
	pf.mu.Unlock()
}

// Kick schedules prefetches for the chapters following current, up to the lookahead window.
func (pf *Prefetcher) Kick(current int) {
	if pf.lookahead == 0 {
		return
	}
	pf.mu.Lock()
	defer pf.mu.Unlock()

	for j := current + 1; j < len(pf.targets) && j <= current+pf.lookahead; j++ {
		if _, exists := pf.pending[j]; exists {
			continue
		}
		t := pf.targets[j]
		if t.Err != nil || t.Parser == nil || !pf.warmed[t.Host] {
			continue
		}
		task := &fetchTask{target: t, done: make(chan struct{})}
		select {
		case pf.tasks <- task:
			pf.pending[j] = task
		default:
			return
		}
	}
}

// Close stops the background worker after in-flight work completes.
func (pf *Prefetcher) Close() {
	if pf.lookahead > 0 {
		close(pf.tasks)
		pf.wg.Wait()
	}
}
