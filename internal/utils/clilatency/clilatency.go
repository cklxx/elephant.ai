package clilatency

import (
	"fmt"
	"os"
	"strings"
)

func Enabled() bool {
	value, ok := os.LookupEnv("ALEX_CLI_LATENCY")
	if !ok {
		return false
	}
	value = strings.TrimSpace(strings.ToLower(value))
	return value == "" || value == "1" || value == "true" || value == "yes"
}

func Printf(format string, args ...any) {
	if !Enabled() {
		return
	}
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}
