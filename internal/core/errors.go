package core

import (
	"errors"
	"fmt"
)

// Common sentinel errors across parsers and download workflows
var (
	ErrNoImagesFound     = errors.New("no images found")
	ErrChapterIDNotFound = errors.New("chapter ID not found in URL")
	ErrScriptNotFound    = errors.New("required script not found")
	ErrManifestNotFound  = errors.New("manifest data not found")
	ErrViewerIDNotFound  = errors.New("viewer ID not found")
	ErrInvalidURL        = errors.New("invalid URL")
	ErrNoParserFound     = errors.New("no parser found for domain")
)

// EnsureImages verifies that at least one image was extracted.
// Returns ErrNoImagesFound if images is empty.
func EnsureImages(images []string) ([]string, error) {
	if len(images) == 0 {
		return nil, ErrNoImagesFound
	}
	return images, nil
}

// ChapterIDNotFoundError formats a standardized chapter ID extraction error.
func ChapterIDNotFoundError(chapterURL string) error {
	return fmt.Errorf("%w: %s", ErrChapterIDNotFound, chapterURL)
}

// JSONParseError formats a standardized JSON unmarshal or parse error.
func JSONParseError(target string, err error) error {
	return fmt.Errorf("failed to parse %s JSON: %w", target, err)
}

// HTTPStatusError formats a standardized HTTP error.
func HTTPStatusError(statusCode int, status string) error {
	return fmt.Errorf("HTTP error: %d %s", statusCode, status)
}

// NoParserFoundError formats an error when no registered parser matches the given host.
func NoParserFoundError(host string) error {
	return fmt.Errorf("%w: %s", ErrNoParserFound, host)
}
