package browser

import (
	"time"

	tools "alex/internal/agent/ports/tools"
)

// Config configures local browser tooling.
type Config struct {
	CDPURL       string
	ChromePath   string
	Headless     bool
	UserDataDir  string
	Timeout      time.Duration
	VisionTool   tools.ToolExecutor
	VisionPrompt string
}

func (c Config) timeoutOrDefault() time.Duration {
	if c.Timeout > 0 {
		return c.Timeout
	}
	return 60 * time.Second
}
