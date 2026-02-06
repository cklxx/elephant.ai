package textutil

// SimilarityScore returns a Jaccard similarity score for the token sets.
func SimilarityScore(a, b string) float64 {
	tokensA := tokenize(a, DefaultMinTokenLen, nil, 0)
	tokensB := tokenize(b, DefaultMinTokenLen, nil, 0)
	if len(tokensA) == 0 || len(tokensB) == 0 {
		return 0
	}
	setA := make(map[string]bool, len(tokensA))
	for _, token := range tokensA {
		setA[token] = true
	}
	intersection := 0
	for _, token := range tokensB {
		if setA[token] {
			intersection++
		}
	}
	union := len(tokensA) + len(tokensB) - intersection
	if union == 0 {
		return 0
	}
	return float64(intersection) / float64(union)
}
