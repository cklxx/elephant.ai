package memory

import "testing"

func TestMergeMatchesRanking(t *testing.T) {
	vecMatches := []VectorMatch{
		{
			Chunk: StoredChunk{
				ID:        1,
				Path:      "memory/2026-02-02.md",
				StartLine: 1,
				EndLine:   2,
				Text:      "Chunk one",
			},
			Distance: 0.1,
		},
		{
			Chunk: StoredChunk{
				ID:        2,
				Path:      "memory/2026-02-02.md",
				StartLine: 3,
				EndLine:   4,
				Text:      "Chunk two",
			},
			Distance: 0.0,
		},
	}
	textMatches := []TextMatch{
		{
			Chunk: StoredChunk{
				ID:        1,
				Path:      "memory/2026-02-02.md",
				StartLine: 1,
				EndLine:   2,
				Text:      "Chunk one",
			},
			BM25: 10,
		},
		{
			Chunk: StoredChunk{
				ID:        3,
				Path:      "MEMORY.md",
				StartLine: 5,
				EndLine:   6,
				Text:      "Chunk three",
			},
			BM25: 0,
		},
	}

	results := mergeMatches(vecMatches, textMatches, 5, 0.1, 0.7, 0.3)
	if len(results) < 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].StartLine != 3 {
		t.Fatalf("expected chunk with best vector distance first, got %+v", results[0])
	}
	if results[1].StartLine != 1 {
		t.Fatalf("expected blended chunk second, got %+v", results[1])
	}
	if results[2].Path != "MEMORY.md" {
		t.Fatalf("expected long-term memory third, got %+v", results[2])
	}
	if results[2].Source != "long_term" {
		t.Fatalf("expected long_term source, got %+v", results[2])
	}
}
