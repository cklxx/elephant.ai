package react

import (
	"strings"
)

type contextOverflowClassification struct {
	Matched bool
	Rule    string
}

func classifyContextOverflow(err error) contextOverflowClassification {
	if err == nil {
		return contextOverflowClassification{}
	}
	msg := strings.ToLower(strings.TrimSpace(err.Error()))
	if msg == "" {
		return contextOverflowClassification{}
	}

	// Explicit non-overflow failures that often contain the word "exceeded".
	if containsAny(msg,
		"rate limit exceeded",
		"rate_limit_exceeded",
		"quota exceeded",
		"deadline exceeded",
		"context deadline exceeded",
		"timed out",
		"timeout",
		"max retries exceeded",
	) {
		return contextOverflowClassification{}
	}

	if containsAny(msg,
		`"code":"context_length_exceeded"`,
		"context_length_exceeded",
	) {
		return contextOverflowClassification{Matched: true, Rule: "context_length_exceeded"}
	}

	if containsAny(msg,
		"maximum context length",
		"context window of this model",
		"exceeds the context window",
		"exceeds the model's maximum context",
		"exceeds your current quota for context",
	) {
		return contextOverflowClassification{Matched: true, Rule: "context_window_phrase"}
	}

	if containsAny(msg,
		"prompt is too long",
		"too many tokens",
		"token limit exceeded",
		"request exceeds maximum allowed",
		"input length and `max_tokens` exceed context limit",
	) {
		return contextOverflowClassification{Matched: true, Rule: "token_limit_phrase"}
	}

	if containsAny(msg, "status 413", "http 413", "413 request entity too large") &&
		containsAny(msg, "token", "context", "prompt", "input") {
		return contextOverflowClassification{Matched: true, Rule: "http_413_with_context_tokens"}
	}

	return contextOverflowClassification{}
}

func containsAny(input string, needles ...string) bool {
	for _, needle := range needles {
		if strings.Contains(input, needle) {
			return true
		}
	}
	return false
}
