package textutil

import "testing"

func TestSimilarityScoreIdentical(t *testing.T) {
	score := SimilarityScore("hello world", "hello world")
	if score != 1 {
		t.Fatalf("score=%v want=1", score)
	}
}

func TestSimilarityScorePartial(t *testing.T) {
	score := SimilarityScore("hello world", "hello there")
	want := 1.0 / 3.0
	if score != want {
		t.Fatalf("score=%v want=%v", score, want)
	}
}

func TestSimilarityScoreEmpty(t *testing.T) {
	score := SimilarityScore(" ", "hello")
	if score != 0 {
		t.Fatalf("score=%v want=0", score)
	}
}
