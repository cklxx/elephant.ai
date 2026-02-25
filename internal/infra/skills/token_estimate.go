package skills

import "alex/internal/shared/token"

// EstimateTokens returns a token count using tiktoken (cl100k_base) when
// available, falling back to a character-based heuristic.
func EstimateTokens(text string) int {
	return tokenutil.CountTokens(text)
}
