package llm

import (
	"alex/internal/shared/utils/clilatency"
)

func logCLILatencyf(format string, args ...any) {
	clilatency.Printf(format, args...)
}
