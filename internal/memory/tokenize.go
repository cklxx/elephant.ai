package memory

import "strings"

// tokenize performs a light-weight whitespace and punctuation based split
// intended for demo purposes only. It favours simplicity over linguistic
// accuracy to keep the dependency surface minimal.
func tokenize(text string) []string {
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
		// Keep CJK characters as part of tokens by treating them as non-separators.
		if r >= 0x4E00 && r <= 0x9FFF {
			return false
		}
		return true
	})

	terms := make([]string, 0, len(fields))
	for _, field := range fields {
		trimmed := strings.TrimSpace(field)
		if trimmed == "" {
			continue
		}
		terms = append(terms, trimmed)
		terms = append(terms, cjkNGrams(trimmed, 2, 3)...)
	}
	return terms
}

func cjkNGrams(token string, minN, maxN int) []string {
	if token == "" || minN <= 0 || maxN < minN {
		return nil
	}
	runes := []rune(token)
	if len(runes) < minN {
		return nil
	}

	const maxGenerated = 128
	const maxSegmentRunes = 64
	out := make([]string, 0, 32)
	start := -1

	flush := func(end int) {
		if start < 0 || end <= start {
			return
		}
		segment := runes[start:end]
		if len(segment) > maxSegmentRunes {
			segment = segment[:maxSegmentRunes]
		}
		segLen := len(segment)
		if segLen < minN {
			return
		}
		for n := minN; n <= maxN; n++ {
			if segLen < n {
				continue
			}
			for i := 0; i+n <= segLen; i++ {
				out = append(out, string(segment[i:i+n]))
				if len(out) >= maxGenerated {
					return
				}
			}
			if len(out) >= maxGenerated {
				return
			}
		}
	}

	for i, r := range runes {
		if isCJK(r) {
			if start < 0 {
				start = i
			}
			continue
		}
		if start >= 0 {
			flush(i)
			if len(out) >= maxGenerated {
				return out[:maxGenerated]
			}
			start = -1
		}
	}
	if start >= 0 {
		flush(len(runes))
	}
	if len(out) == 0 {
		return nil
	}
	if len(out) > maxGenerated {
		return out[:maxGenerated]
	}
	return out
}

func isCJK(r rune) bool {
	return r >= 0x4E00 && r <= 0x9FFF
}
