package cli

import (
	"fmt"
)

// ShowHelp prints CLI usage help
func ShowHelp() {
	fmt.Println()
	fmt.Println("Commands & Flags:")
	fmt.Println("  -l, --list, list        Display all supported parsers and domains")
	fmt.Println("  -t, --test <url>        Run diagnostic checks on a URL")
	fmt.Println("  -f, --folder            Download chapter into its own subfolder")
	fmt.Println("  -h, --help, help        Show this help message")
	fmt.Println()
	fmt.Println("Download Usage:")
	fmt.Println("  Dmanga <url>              Single chapter download (saves to current directory)")
	fmt.Println("  Dmanga -f <url>           Single chapter download into a dedicated folder")
	fmt.Println("  Dmanga <url1> <url2> ...  Multi-link download (folders: Ch 1, Ch 2, ...)")
	fmt.Println("  Dmanga \"<url1> | <url2>\"  Pipe-separated multi-link download")
	fmt.Println("  Dmanga                    prompt mode")
	fmt.Println()
}
