package parser

import (
	"strings"

	"dmanga/internal/core"
)

// allParsers holds all registered parsers populated automatically via init() functions
var allParsers []core.Parser

// Register is called automatically by init() in each parser file
func Register(p core.Parser) {
	allParsers = append(allParsers, p)
}

// FindParser looks up a matching parser based on the URL domain
func FindParser(host string) (core.Parser, error) {
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
func GetAllParsers() []core.Parser {
	result := make([]core.Parser, len(allParsers))
	copy(result, allParsers)
	return result
}
