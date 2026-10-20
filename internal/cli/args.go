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
