package fetcher

import (
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

var (
	rateLimitMu       sync.RWMutex
	rateLimitCooldown time.Time
	rateLimitHits     atomic.Int64
)

// RateLimitHits returns how many times a global cooldown has been triggered (HTTP 429 events).
// Callers can compare successive values to detect new rate limiting and throttle concurrency.
func RateLimitHits() int64 {
	return rateLimitHits.Load()
}

// CheckRateLimitCooldown blocks if a global rate limit cooldown is active across all workers
func CheckRateLimitCooldown() {
	rateLimitMu.RLock()
	until := time.Until(rateLimitCooldown)
	rateLimitMu.RUnlock()

	if until > 0 {
		Sleep(until)
	}
}

// TriggerRateLimitCooldown activates a global cooldown duration for all workers
func TriggerRateLimitCooldown(d time.Duration) {
	rateLimitHits.Add(1)
	rateLimitMu.Lock()
	target := time.Now().Add(d)
	if target.After(rateLimitCooldown) {
		rateLimitCooldown = target
	}
	rateLimitMu.Unlock()
}

// ParseRetryAfter parses the HTTP Retry-After header or calculates exponential backoff
func ParseRetryAfter(headerVal string, attempt int) time.Duration {
	headerVal = strings.TrimSpace(headerVal)
	if headerVal != "" {
		if sec, err := strconv.Atoi(headerVal); err == nil && sec > 0 {
			if sec > 300 {
				sec = 300
			}
			return time.Duration(sec) * time.Second
		}
		for _, layout := range []string{time.RFC1123, time.RFC850, time.ANSIC} {
			if t, err := time.Parse(layout, headerVal); err == nil {
				d := time.Until(t)
				if d > 0 {
					if d > 5*time.Minute {
						d = 5 * time.Minute
					}
					return d
				}
				break
			}
		}
	}

	// Exponential backoff fallback: 2s, 4s, 8s, 16s
	backoff := time.Duration(1<<uint(attempt+1)) * time.Second
	if backoff > 30*time.Second {
		backoff = 30 * time.Second
	}
	return backoff
}
