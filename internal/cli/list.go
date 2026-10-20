package cli

import (
	"fmt"
	"sort"
	"strings"

	"dmanga/internal/parser"
)

// ShowSupportedParsers displays all available parsers and their covered domains
func ShowSupportedParsers() {
	all := parser.GetAllParsers()
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

func pluralS(count int) string {
	if count == 1 {
		return ""
	}
	return "s"
}
