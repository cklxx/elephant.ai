package lark

import "alex/internal/shared/uxphrases"

// Package-level aliases so existing callers within this package keep working.
var (
	thinkingPhrases    = uxphrases.ThinkingPhrases
	summarizingPhrases = uxphrases.SummarizingPhrases
	defaultPhrases     = uxphrases.DefaultPhrases
)

// toolPhrase returns a friendly Chinese status phrase for the given tool name.
func toolPhrase(toolName string, selector int) string {
	return uxphrases.ToolPhrase(toolName, selector)
}

// toolPhraseForBackground is an alias used by the background progress listener.
func toolPhraseForBackground(toolName string, selector int) string {
	return uxphrases.ToolPhrase(toolName, selector)
}

// pickPhrase selects a phrase from the pool using deterministic rotation.
func pickPhrase(pool []string, selector int) string {
	return uxphrases.PickPhrase(pool, selector)
}
