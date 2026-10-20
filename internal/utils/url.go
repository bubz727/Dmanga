package utils

import (
	"fmt"
	"strings"
)

// CleanImageURL resolves relative URLs, protocol-relative schemes (//), and removes WordPress resize query params (?w=...)
func CleanImageURL(rawURL, baseHost string) string {
	val := strings.TrimSpace(rawURL)
	if val == "" {
		return ""
	}

	// Strip WordPress resize query parameters (e.g. ?w=640&quality=80)
	if idx := strings.Index(val, "?"); idx != -1 {
		val = val[:idx]
	}

	// Handle protocol-relative URL
	if strings.HasPrefix(val, "//") {
		return "https:" + val
	}

	// Handle relative path (/wp-content/...)
	if strings.HasPrefix(val, "/") && baseHost != "" {
		return fmt.Sprintf("https://%s%s", baseHost, val)
	}

	return val
}
