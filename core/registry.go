package core

import (
	"strings"
)

// allParsers holds all registered parsers populated automatically via init() functions
var allParsers []Parser

// Register is called automatically by init() in each parser file
func Register(p Parser) {
	allParsers = append(allParsers, p)
}

// FindParser looks up a matching parser based on the URL domain
func FindParser(host string) (Parser, error) {
	cleanHost := strings.ToLower(host)

	for _, p := range allParsers {
		for _, domain := range p.Domains() {
			if strings.Contains(cleanHost, strings.ToLower(domain)) {
				return p, nil
			}
		}
	}

	return nil, NoParserFoundError(host)
}

// GetAllParsers returns a copy of all registered parsers
func GetAllParsers() []Parser {
	result := make([]Parser, len(allParsers))
	copy(result, allParsers)
	return result
}
