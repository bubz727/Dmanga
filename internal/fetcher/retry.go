package fetcher

import (
	"context"
	"math/rand"
	"sync"
	"time"
)

const (
	backoffBase = 300 * time.Millisecond
	backoffCap  = 5 * time.Second
)

var (
	ctxMu  sync.RWMutex
	appCtx = context.Background()
)

// SetContext installs the application-wide context (cancelled on Ctrl+C). All requests and retry waits
// issued through this package observe it, so an interrupt stops work promptly.
func SetContext(ctx context.Context) {
	ctxMu.Lock()
	appCtx = ctx
	ctxMu.Unlock()
}

// Context returns the application-wide context (context.Background() unless SetContext was called).
func Context() context.Context {
	ctxMu.RLock()
	defer ctxMu.RUnlock()
	return appCtx
}

// Backoff returns the wait before retry number attempt (0-based): exponential from 300 ms, capped at 5 s,
// with +/-25% jitter so parallel workers do not retry in lockstep.
func Backoff(attempt int) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	d := backoffBase
	for i := 0; i < attempt && d < backoffCap; i++ {
		d *= 2
	}
	if d > backoffCap {
		d = backoffCap
	}
	// jitter in [0.75, 1.25)
	factor := 0.75 + rand.Float64()*0.5
	return time.Duration(float64(d) * factor)
}

// Sleep waits for d or until the application context is cancelled.
// It returns false if the wait was interrupted by cancellation.
func Sleep(d time.Duration) bool {
	if d <= 0 {
		return Context().Err() == nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-Context().Done():
		return false
	case <-timer.C:
		return true
	}
}
