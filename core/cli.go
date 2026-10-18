package core

import (
	"fmt"
	"sort"
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

// ShowSupportedParsers displays all available parsers and their covered domains
func ShowSupportedParsers() {
	all := GetAllParsers()
	sort.Slice(all, func(i, j int) bool {
		return strings.ToLower(all[i].Name()) < strings.ToLower(all[j].Name())
	})

	totalDomains := 0
	for _, p := range all {
		totalDomains += len(p.Domains())
	}

	fmt.Println("==================================================")
	fmt.Println("            SUPPORTED MANGA PARSERS               ")
	fmt.Println("==================================================")
	for idx, p := range all {
		domains := p.Domains()
		sort.Strings(domains)
		fmt.Printf("[%02d] %s (%d domain%s)\n", idx+1, p.Name(), len(domains), pluralS(len(domains)))

		if len(domains) <= 5 {
			fmt.Printf("     %s\n", strings.Join(domains, ", "))
		} else {
			line := "     "
			for dIdx, d := range domains {
				if len(line)+len(d)+2 > 75 {
					fmt.Println(strings.TrimRight(line, " "))
					line = "     "
				}
				line += d
				if dIdx < len(domains)-1 {
					line += ", "
				}
			}
			if strings.TrimSpace(line) != "" {
				fmt.Println(line)
			}
		}
		fmt.Println()
	}
	fmt.Println("--------------------------------------------------")
	fmt.Printf("Total: %d Parsers, %d Supported Domains\n", len(all), totalDomains)
	fmt.Println("==================================================")
	fmt.Println("Usage Commands:")
	fmt.Println("  Dmanga <chapter-url>                  Download chapter")
	fmt.Println("  Dmanga <url-1> <url-2>                Multi-chapter download")
	fmt.Println("  Dmanga -t <chapter-url>               Run parser diagnostics")
	fmt.Println("  Dmanga -l                             List all supported parsers")
	fmt.Println("==================================================")
}

// ShowHelp prints CLI usage help
func ShowHelp() {
	fmt.Println("==================================================")
	fmt.Println("            CLI MANGA DOWNLOADER HELP             ")
	fmt.Println("==================================================")
	fmt.Println("Commands & Flags:")
	fmt.Println("  -l, --list, list        Display all supported parsers and domains")
	fmt.Println("  -t, --test <url>        Run diagnostic checks on a URL")
	fmt.Println("  -h, --help, help        Show this help message")
	fmt.Println()
	fmt.Println("Download Usage:")
	fmt.Println("  Dmanga <url>              Single chapter download")
	fmt.Println("  Dmanga <url1> <url2> ...  Multi-link download (folders: Chapter 1, Chapter 2, ...)")
	fmt.Println("  Dmanga \"<url1> | <url2>\"  Pipe-separated multi-link download")
	fmt.Println("  Dmanga                    Interactive prompt mode")
	fmt.Println("==================================================")
}

func pluralS(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
