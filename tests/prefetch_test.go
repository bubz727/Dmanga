package tests

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"dmanga/internal/cli"
	"dmanga/internal/core"
)

type fakeParser struct{ name string }

func (f fakeParser) Name() string                       { return f.name }
func (f fakeParser) Domains() []string                  { return []string{"example.com"} }
func (f fakeParser) GetImages(string) ([]string, error) { return []string{"a"}, nil }

func makeTargets(hosts ...string) []cli.ChapterTarget {
	var targets []cli.ChapterTarget
	for i, h := range hosts {
		targets = append(targets, cli.ChapterTarget{
			URL:    "https://" + h + "/ch-" + string(rune('1'+i)),
			Host:   h,
			Parser: fakeParser{name: h},
		})
	}
	return targets
}

func TestParsePrefetchFlag(t *testing.T) {
	tests := []struct {
		name      string
		items     []string
		wantN     int
		wantFound bool
	}{
		{"space", []string{"--prefetch", "3", "https://a.com/1"}, 3, true},
		{"equals", []string{"--prefetch=4"}, 4, true},
		{"zero disables", []string{"--prefetch", "0"}, 0, true},
		{"prompt string", []string{"--prefetch 1 https://a.com/1"}, 1, true},
		{"missing value", []string{"--prefetch"}, -1, true},
		{"non numeric", []string{"--prefetch", "x"}, -1, true},
		{"negative", []string{"--prefetch", "-2"}, -1, true},
		{"absent", []string{"-c", "5", "https://a.com/1"}, 0, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			n, found := cli.ParsePrefetchFlag(tt.items...)
			if n != tt.wantN || found != tt.wantFound {
				t.Errorf("ParsePrefetchFlag(%v) = (%d, %v), expected (%d, %v)", tt.items, n, found, tt.wantN, tt.wantFound)
			}
		})
	}
}

func TestResolvePrefetch(t *testing.T) {
	t.Setenv("DMANGA_PREFETCH", "")

	if n, warn := cli.ResolvePrefetch(0, false); n != cli.DefaultPrefetch || warn != "" {
		t.Errorf("Default: got (%d, %q)", n, warn)
	}
	if n, _ := cli.ResolvePrefetch(0, true); n != 0 {
		t.Errorf("Flag 0 must disable prefetch, got %d", n)
	}
	if n, warn := cli.ResolvePrefetch(99, true); n != cli.MaxPrefetch || warn == "" {
		t.Errorf("Clamp: got (%d, %q)", n, warn)
	}
	if n, warn := cli.ResolvePrefetch(-1, true); n != cli.DefaultPrefetch || warn == "" {
		t.Errorf("Invalid flag: got (%d, %q)", n, warn)
	}

	t.Setenv("DMANGA_PREFETCH", "4")
	if n, _ := cli.ResolvePrefetch(0, false); n != 4 {
		t.Errorf("Env value: got %d, expected 4", n)
	}
	if n, _ := cli.ResolvePrefetch(1, true); n != 1 {
		t.Errorf("Flag must override env: got %d", n)
	}
}

func TestPrefetchPipelinesMetadataWithDownloads(t *testing.T) {
	const delay = 200 * time.Millisecond
	var calls sync.Map // url -> *atomic.Int64

	fetch := func(p core.Parser, u string) (*core.Chapter, error) {
		v, _ := calls.LoadOrStore(u, new(atomic.Int64))
		v.(*atomic.Int64).Add(1)
		time.Sleep(delay)
		return &core.Chapter{Title: u, Images: []string{"x"}}, nil
	}

	run := func(lookahead int) time.Duration {
		targets := makeTargets("example.com", "example.com", "example.com")
		pf := cli.NewPrefetcher(targets, lookahead, fetch)
		defer pf.Close()

		start := time.Now()
		for i := range targets {
			chap, err := pf.Get(i)
			if err != nil || chap.Title != targets[i].URL {
				t.Fatalf("Chapter %d: unexpected result (%v, %v)", i, chap, err)
			}
			pf.MarkWarmed(targets[i].Host)
			pf.Kick(i)
			time.Sleep(delay) // simulated image download
		}
		return time.Since(start)
	}

	sequential := run(0)
	pipelined := run(2)

	if pipelined >= sequential-150*time.Millisecond {
		t.Errorf("Prefetch should clearly beat sequential flow: sequential=%v pipelined=%v", sequential, pipelined)
	}

	// Each chapter must be fetched exactly once per run (2 runs): no duplicate fetches
	calls.Range(func(k, v any) bool {
		if got := v.(*atomic.Int64).Load(); got != 2 {
			t.Errorf("URL %v fetched %d times across 2 runs, expected 2", k, got)
		}
		return true
	})
}

func TestPrefetchOnlyWarmedHosts(t *testing.T) {
	var fetchedB atomic.Int64
	fetch := func(p core.Parser, u string) (*core.Chapter, error) {
		if p.Name() == "b.com" {
			fetchedB.Add(1)
		}
		return &core.Chapter{Title: u, Images: []string{"x"}}, nil
	}

	targets := makeTargets("a.com", "b.com")
	pf := cli.NewPrefetcher(targets, 2, fetch)
	defer pf.Close()

	pf.MarkWarmed("a.com")
	pf.Kick(0)
	time.Sleep(300 * time.Millisecond)

	if fetchedB.Load() != 0 {
		t.Errorf("Host b.com was prefetched before being warmed (first chapter per host must be fetched in the foreground)")
	}

	// Foreground fetch still works for the unwarmed host
	if _, err := pf.Get(1); err != nil {
		t.Errorf("Foreground Get failed: %v", err)
	}
	if fetchedB.Load() != 1 {
		t.Errorf("Expected exactly 1 fetch for b.com, got %d", fetchedB.Load())
	}
}

func TestPrefetchFailureRetriedInForeground(t *testing.T) {
	var attempts atomic.Int64
	fetch := func(p core.Parser, u string) (*core.Chapter, error) {
		if u == "https://example.com/ch-2" && attempts.Add(1) == 1 {
			return nil, errors.New("transient failure")
		}
		return &core.Chapter{Title: u, Images: []string{"x"}}, nil
	}

	targets := makeTargets("example.com", "example.com")
	pf := cli.NewPrefetcher(targets, 1, fetch)
	defer pf.Close()

	if _, err := pf.Get(0); err != nil {
		t.Fatal(err)
	}
	pf.MarkWarmed("example.com")
	pf.Kick(0)

	chap, err := pf.Get(1)
	if err != nil || chap == nil {
		t.Fatalf("Expected foreground retry to succeed after failed prefetch, got (%v, %v)", chap, err)
	}
	if attempts.Load() != 2 {
		t.Errorf("Expected 2 attempts (prefetch + retry), got %d", attempts.Load())
	}
}

func TestPrefetchDisabledNeverFetchesAhead(t *testing.T) {
	var calls atomic.Int64
	fetch := func(p core.Parser, u string) (*core.Chapter, error) {
		calls.Add(1)
		return &core.Chapter{Title: u, Images: []string{"x"}}, nil
	}

	targets := makeTargets("example.com", "example.com", "example.com")
	pf := cli.NewPrefetcher(targets, 0, fetch)
	defer pf.Close()

	pf.MarkWarmed("example.com")
	pf.Kick(0)
	time.Sleep(200 * time.Millisecond)

	if calls.Load() != 0 {
		t.Errorf("Prefetch disabled but %d background fetches happened", calls.Load())
	}
}
