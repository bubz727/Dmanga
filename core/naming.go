package core

import (
	"fmt"
	"regexp"
	"strings"
)

var chapterKeywordsRegex = regexp.MustCompile(`(?i)(?:chapter|ch|c|episode|ep)[.\s-_:#]*([0-9]+(?:\.[0-9]+)?)`)

// SanitizeFolderName removes or replaces invalid Windows path characters (\ / : * ? " < > |)
func SanitizeFolderName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" {
		return ""
	}

	// Replace illegal Windows characters: \ / : * ? " < > |
	invalid := regexp.MustCompile(`[\\/:*?"<>|]`)
	cleaned := invalid.ReplaceAllString(name, "-")

	// Collapse multiple spaces or dashes
	spaceRegex := regexp.MustCompile(`\s+`)
	cleaned = spaceRegex.ReplaceAllString(cleaned, " ")

	// Trim trailing periods and spaces (Windows restriction)
	cleaned = strings.Trim(cleaned, ". ")
	return cleaned
}

// ExtractChapterNumber finds the chapter or episode number from text when explicitly preceded by chapter keywords
func ExtractChapterNumber(s string) string {
	if matches := chapterKeywordsRegex.FindStringSubmatch(s); len(matches) > 1 {
		return matches[1]
	}
	return ""
}

// NormalizeChapterTitle extracts the chapter number and returns "Ch <number>".
// It only parses the rawTitle from the HTML/API and does NOT guess numbers from URLs.
func NormalizeChapterTitle(rawTitle string) string {
	rawTitle = strings.TrimSpace(rawTitle)
	if rawTitle == "" {
		return ""
	}

	if num := ExtractChapterNumber(rawTitle); num != "" {
		return fmt.Sprintf("Ch %s", num)
	}

	return SanitizeFolderName(rawTitle)
}

// FormatImageIndex returns a zero-padded index string based on total count (minimum 3 digits)
func FormatImageIndex(index, total int) string {
	digits := 3
	for n := total; n >= 1000; n /= 10 {
		digits++
	}
	format := fmt.Sprintf("%%0%dd", digits)
	return fmt.Sprintf(format, index)
}

