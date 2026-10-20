package utils

import (
	"net/url"
	"path/filepath"
	"strings"
)

// GetFileExtension extracts the image extension (.jpg, .png, .webp). Defaults to .jpg.
func GetFileExtension(rawURL string) string {
	u, err := url.Parse(rawURL)
	if err == nil {
		ext := filepath.Ext(u.Path)
		if ext != "" && len(ext) <= 5 {
			return strings.ToLower(ext)
		}
	}
	return ".jpg"
}
