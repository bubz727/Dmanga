package parsers

import (
	"fmt"
	"strings"

	"manga-downloader/themes"
)

// AllParsers mendaftarkan semua parser yang aktif
var AllParsers = []themes.Parser{
	NewApKomik(),
}

// FindParser mencari parser yang cocok berdasarkan domain URL
func FindParser(host string) (themes.Parser, error) {
	cleanHost := strings.ToLower(host)

	for _, p := range AllParsers {
		for _, domain := range p.Domains() {
			if strings.Contains(cleanHost, strings.ToLower(domain)) {
				return p, nil
			}
		}
	}

	return nil, fmt.Errorf("tidak ditemukan parser untuk domain '%s'", host)
}
