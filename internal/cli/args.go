package cli

import (
	"fmt"
	"os"
	"strconv"
	"strings"

	"dmanga/internal/downloader"
)

// ParseInputURLs splits an input string containing one or more URLs
func ParseInputURLs(rawInput string) []string {
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

// HasFolderFlag checks if -f or --folder flag is present in any slice of strings or arguments
func HasFolderFlag(items ...string) bool {
	for _, item := range items {
		for _, tok := range strings.Fields(item) {
			lower := strings.ToLower(strings.TrimSpace(tok))
			if lower == "-f" || lower == "--folder" {
				return true
			}
		}
	}
	return false
}

// HasNoUpdateCheckFlag checks if --no-update-check is present in arguments
func HasNoUpdateCheckFlag(items ...string) bool {
	for _, item := range items {
		for _, tok := range strings.Fields(item) {
			lower := strings.ToLower(strings.TrimSpace(tok))
			if lower == "--no-update-check" || lower == "-no-update-check" {
				return true
			}
		}
	}
	return false
}

// scanIntFlag looks for a flag in any of the given forms ("-x N", "-x=N", "--name N", "--name=N") across the
// arguments (each item may hold several space-separated tokens, like prompt input).
// It returns the raw value text (empty when missing) and whether the flag was present.
func scanIntFlag(items []string, short, long string) (val string, found bool) {
	var tokens []string
	for _, item := range items {
		tokens = append(tokens, strings.Fields(item)...)
	}
	for i, tok := range tokens {
		lower := strings.ToLower(strings.TrimSpace(tok))
		switch {
		case (short != "" && lower == "-"+short) || lower == "--"+long || lower == "-"+long:
			if i+1 < len(tokens) {
				val = tokens[i+1]
			}
			return strings.TrimSpace(val), true
		case (short != "" && strings.HasPrefix(lower, "-"+short+"=")) || strings.HasPrefix(lower, "--"+long+"=") || strings.HasPrefix(lower, "-"+long+"="):
			return strings.TrimSpace(tok[strings.Index(tok, "=")+1:]), true
		}
	}
	return "", false
}

// ParseConcurrencyFlag looks for -c N, -c=N, --concurrency N or --concurrency=N across the given arguments.
// found reports whether the flag was present; n is 0 when the value is missing or not a positive integer.
func ParseConcurrencyFlag(items ...string) (n int, found bool) {
	val, found := scanIntFlag(items, "c", "concurrency")
	if !found {
		return 0, false
	}
	if parsed, err := strconv.Atoi(val); err == nil && parsed > 0 {
		return parsed, true
	}
	return 0, true
}

// ParsePrefetchFlag looks for --prefetch N or --prefetch=N across the given arguments.
// found reports whether the flag was present; n is -1 when the value is missing or not a non-negative integer
// (0 is valid and disables prefetching).
func ParsePrefetchFlag(items ...string) (n int, found bool) {
	val, found := scanIntFlag(items, "", "prefetch")
	if !found {
		return 0, false
	}
	if parsed, err := strconv.Atoi(val); err == nil && parsed >= 0 {
		return parsed, true
	}
	return -1, true
}

// ResolveConcurrency picks the worker count: flag value, then DMANGA_CONCURRENCY, then the default.
// The result is clamped to the supported range; warn is non-empty when the user should be told something.
func ResolveConcurrency(flagValue int, flagFound bool) (n int, warn string) {
	requested := 0
	switch {
	case flagFound && flagValue <= 0:
		warn = fmt.Sprintf("Invalid concurrency value, using default (%d)", downloader.DefaultConcurrency)
	case flagFound:
		requested = flagValue
	default:
		if env := strings.TrimSpace(os.Getenv("DMANGA_CONCURRENCY")); env != "" {
			if v, err := strconv.Atoi(env); err == nil && v > 0 {
				requested = v
			} else {
				warn = fmt.Sprintf("Invalid DMANGA_CONCURRENCY value %q, using default (%d)", env, downloader.DefaultConcurrency)
			}
		}
	}
	if requested > downloader.MaxConcurrency {
		warn = fmt.Sprintf("Concurrency %d exceeds the maximum, using %d", requested, downloader.MaxConcurrency)
	}
	return downloader.ClampConcurrency(requested), warn
}

// ResolvePrefetch picks the prefetch lookahead: flag value, then DMANGA_PREFETCH, then the default.
// The result is clamped to 0..MaxPrefetch; warn is non-empty when the user should be told something.
func ResolvePrefetch(flagValue int, flagFound bool) (n int, warn string) {
	n = DefaultPrefetch
	switch {
	case flagFound && flagValue < 0:
		warn = fmt.Sprintf("Invalid prefetch value, using default (%d)", DefaultPrefetch)
	case flagFound:
		n = flagValue
	default:
		if env := strings.TrimSpace(os.Getenv("DMANGA_PREFETCH")); env != "" {
			if v, err := strconv.Atoi(env); err == nil && v >= 0 {
				n = v
			} else {
				warn = fmt.Sprintf("Invalid DMANGA_PREFETCH value %q, using default (%d)", env, DefaultPrefetch)
			}
		}
	}
	if n > MaxPrefetch {
		warn = fmt.Sprintf("Prefetch %d exceeds the maximum, using %d", n, MaxPrefetch)
		n = MaxPrefetch
	}
	return n, warn
}

// HasForceFlag checks if --force is present in any slice of strings or arguments
func HasForceFlag(items ...string) bool {
	for _, item := range items {
		for _, tok := range strings.Fields(item) {
			lower := strings.ToLower(strings.TrimSpace(tok))
			if lower == "--force" || lower == "-force" {
				return true
			}
		}
	}
	return false
}
