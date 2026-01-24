package app

import "alex/internal/agent/ports"

type toolConcurrencyLimiter struct {
	limit int
}

// NewToolExecutionLimiter returns a semaphore-based limiter for tool calls.
func NewToolExecutionLimiter(maxConcurrent int) ports.ToolExecutionLimiter {
	if maxConcurrent <= 0 {
		return nil
	}
	return &toolConcurrencyLimiter{limit: maxConcurrent}
}

func (l *toolConcurrencyLimiter) Limit() int {
	if l == nil {
		return 0
	}
	return l.limit
}
