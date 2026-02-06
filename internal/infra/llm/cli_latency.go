package llm

import (
	"alex/internal/utils/clilatency"
)

func logCLILatencyf(format string, args ...any) {
	clilatency.Printf(format, args...)
}
