package parsers

import (
	"fmt"
	"strings"

	"manga-downloader/themes"
)

// allParsers menampung seluruh parser yang otomatis mendaftar lewat fungsi init()
var allParsers []themes.Parser

// Register dipanggil otomatis oleh fungsi init() di tiap file parser
func Register(p themes.Parser) {
	allParsers = append(allParsers, p)
}

// FindParser mencari parser yang cocok berdasarkan domain URL
func FindParser(host string) (themes.Parser, error) {
	cleanHost := strings.ToLower(host)

	for _, p := range allParsers {
		for _, domain := range p.Domains() {
			if strings.Contains(cleanHost, strings.ToLower(domain)) {
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("tidak ditemukan parser untuk domain '%s'", host)
}
