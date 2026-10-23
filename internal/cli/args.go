package cli

import (
	"strings"
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
