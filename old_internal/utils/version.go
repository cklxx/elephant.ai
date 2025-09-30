package utils

import (
	"fmt"
	"runtime"
)

// Version information set at build time via -ldflags
var (
	// Version is the main version number that is being built
	Version = "dev"

	// GitCommit is the git sha1 that was used to build the binary
	GitCommit = "unknown"

	// BuildTime is the time when the binary was built
	BuildTime = "unknown"

	// GoVersion is the version of the Go that was used to build the binary
	GoVersion = runtime.Version()

	// Platform is the os/arch combination that was used to build the binary
	Platform = fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH)
)

// GetVersion returns the version string for --version flag
func GetVersion() string {
	return Version
}
