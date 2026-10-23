package cli

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	// Version is injected via linker flag: -X 'dmanga/internal/cli.Version=...'
	Version = "dev"
	// GitCommit is injected via linker flag: -X 'dmanga/internal/cli.GitCommit=...'
	GitCommit = "none"
	// BuildTime is injected via linker flag: -X 'dmanga/internal/cli.BuildTime=...'
	BuildTime = "unknown"
)

// GetVersion returns the formatted version string (e.g. "v1.0.0" or "dev")
func GetVersion() string {
	clean := strings.TrimSpace(Version)
	if clean == "" || strings.EqualFold(clean, "dev") {
		return "dev"
	}
	return NormalizeVersion(clean)
}

// ShowVersion prints the version directly (e.g. "v1.0.0")
func ShowVersion() {
	fmt.Println(GetVersion())
}

// ShowVerboseVersion prints full version and build metadata
func ShowVerboseVersion() {
	fmt.Printf("Dmanga %s\n", GetVersion())
	fmt.Printf("Git Commit : %s\n", GitCommit)
	fmt.Printf("Build Time : %s\n", BuildTime)
	fmt.Printf("Go Runtime : %s (%s/%s)\n", runtime.Version(), runtime.GOOS, runtime.GOARCH)
}
