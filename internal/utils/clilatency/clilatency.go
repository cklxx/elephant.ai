package clilatency

import (
	"context"
	"fmt"
	"os"
	"strings"

	id "alex/internal/utils/id"
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

func PrintfWithContext(ctx context.Context, format string, args ...any) {
	if !Enabled() {
		return
	}
	logID := strings.TrimSpace(id.LogIDFromContext(ctx))
	if logID != "" {
		format = "[log_id=%s] " + format
		args = append([]any{logID}, args...)
	}
	_, _ = fmt.Fprintf(os.Stderr, format, args...)
}
