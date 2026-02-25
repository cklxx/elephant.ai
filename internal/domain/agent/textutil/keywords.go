package textutil

import "strings"

const (
	DefaultMaxKeywords = 10
	DefaultMinTokenLen = 2
)

// KeywordOptions controls keyword extraction behavior.
// MaxKeywords == 0 uses DefaultMaxKeywords. MaxKeywords < 0 means unlimited.
// MinTokenLen <= 0 uses DefaultMinTokenLen.
type KeywordOptions struct {
	MaxKeywords int
	MinTokenLen int
	StopWords   map[string]struct{}
}

// ExtractKeywords tokenizes text into distinct, lowercased keywords.
func ExtractKeywords(text string, opts KeywordOptions) []string {
	maxKeywords := opts.MaxKeywords
	if maxKeywords == 0 {
		maxKeywords = DefaultMaxKeywords
	} else if maxKeywords < 0 {
		maxKeywords = 0
	}
	minTokenLen := opts.MinTokenLen
	if minTokenLen <= 0 {
		minTokenLen = DefaultMinTokenLen
	}
	return tokenize(text, minTokenLen, opts.StopWords, maxKeywords)
}

func tokenize(text string, minTokenLen int, stopWords map[string]struct{}, maxKeywords int) []string {
	fields := strings.FieldsFunc(text, func(r rune) bool {
		if r >= 'a' && r <= 'z' {
			return false
		}
		if r >= 'A' && r <= 'Z' {
			return false
		}
		if r >= '0' && r <= '9' {
			return false
		}
		if r >= 0x4E00 && r <= 0x9FFF {
			return false
		}
		return true
	})

	tokens := make([]string, 0, len(fields))
	seen := make(map[string]bool, len(fields))
	for _, field := range fields {
		lower := strings.ToLower(strings.TrimSpace(field))
		if lower == "" || len(lower) < minTokenLen {
			continue
		}
		if stopWords != nil {
			if _, ok := stopWords[lower]; ok {
				continue
			}
		}
		if seen[lower] {
			continue
		}
		seen[lower] = true
		tokens = append(tokens, lower)
		if maxKeywords > 0 && len(tokens) >= maxKeywords {
			break
		}
	}
	return tokens
}
