package utils

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	chapterKeywordsRegex      = regexp.MustCompile(`(?i)\b(?:chapter|chap|ch|c|episode|ep)[.\s-_:#]*([0-9]+(?:\.[0-9]+)?)`)
	koreanHwaRegex            = regexp.MustCompile(`([0-9]+(?:\.[0-9]+)?)\s*화`)
	hashChapterRegex          = regexp.MustCompile(`(?i)#\s*([0-9]+(?:\.[0-9]+)?)`)
	mangaraw4uChapterURLRegex = regexp.MustCompile(`(?i)chapter-([0-9]+)(?:-([0-9]+))?`)
)

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
	if matches := koreanHwaRegex.FindStringSubmatch(s); len(matches) > 1 {
		return matches[1]
	}
	if matches := hashChapterRegex.FindStringSubmatch(s); len(matches) > 1 {
		trimmed := strings.TrimLeft(matches[1], "0")
		if trimmed == "" {
			return "0"
		}
		return trimmed
	}
	return ""
}

// ExtractMangaRaw4UChapterTitle extracts the chapter number from a MangaRaw4U chapter URL
func ExtractMangaRaw4UChapterTitle(chapterURL string) string {
	if m := mangaraw4uChapterURLRegex.FindStringSubmatch(chapterURL); len(m) > 1 {
		num := m[1]
		if len(m) > 2 && m[2] != "" {
			part := m[2]
			if len(part) == 1 {
				part = part + "0"
			}
			return fmt.Sprintf("Ch %s.%s", num, part)
		}
		return fmt.Sprintf("Ch %s", num)
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
