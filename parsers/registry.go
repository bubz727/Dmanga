package parsers

import (
	"strings"

	"manga-downloader/core"
	"manga-downloader/themes"
)

// allParsers holds all registered parsers populated automatically via init() functions
var allParsers []themes.Parser

// Register is called automatically by init() in each parser file
func Register(p themes.Parser) {
	allParsers = append(allParsers, p)
}

// FindParser looks up a matching parser based on the URL domain
func FindParser(host string) (themes.Parser, error) {
	cleanHost := strings.ToLower(host)

	for _, p := range allParsers {
		for _, domain := range p.Domains() {
			if strings.Contains(cleanHost, strings.ToLower(domain)) {
				return p, nil
			}
		}
	}

	return nil, core.NoParserFoundError(host)
}

// GetAllParsers returns a copy of all registered parsers
func GetAllParsers() []themes.Parser {
	result := make([]themes.Parser, len(allParsers))
	copy(result, allParsers)
	return result
}
