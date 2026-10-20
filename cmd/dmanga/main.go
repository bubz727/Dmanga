package main

import (
	"os"

	"dmanga/internal/cli"
	_ "dmanga/internal/parser"
)

func main() {
	cli.Run(os.Args)
}
