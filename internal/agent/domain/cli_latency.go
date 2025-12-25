package domain

import (
	"fmt"
	"os"
	"strings"
)

func shouldLogCLILatency() bool {
	value, ok := os.LookupEnv("ALEX_CLI_LATENCY")
	if !ok {
		return false
	}
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

func logCLILatencyf(format string, args ...any) {
	if !shouldLogCLILatency() {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}

