package cli

import (
	"fmt"
	"runtime"
)

var (
	// Version is injected via linker flag: -X 'dmanga/internal/cli.Version=...'
	Version = "dev"
	// GitCommit is injected via linker flag: -X 'dmanga/internal/cli.GitCommit=...'
	GitCommit = "none"
	// BuildTime is injected via linker flag: -X 'dmanga/internal/cli.BuildTime=...'
	BuildTime = "unknown"
)

// ShowVersion prints the single source of truth version and build metadata
func ShowVersion() {
	fmt.Printf("Dmanga %s\n", Version)
	fmt.Printf("Git Commit : %s\n", GitCommit)
	fmt.Printf("Build Time : %s\n", BuildTime)
	fmt.Printf("Go Runtime : %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
