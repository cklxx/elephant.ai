package textutil

import "testing"

func TestExtractKeywordsDefaults(t *testing.T) {
	input := "Hello, world! Hello again."
	got := ExtractKeywords(input, KeywordOptions{})
	want := []string{"hello", "world", "again"}
	assertStringSlice(t, got, want)
}

func TestExtractKeywordsStopWords(t *testing.T) {
	input := "Hello world from hello"
	stop := map[string]struct{}{"hello": {}}
	got := ExtractKeywords(input, KeywordOptions{StopWords: stop})
	want := []string{"world", "from"}
	assertStringSlice(t, got, want)
}

func TestExtractKeywordsMaxKeywords(t *testing.T) {
	input := "one two three four"
	got := ExtractKeywords(input, KeywordOptions{MaxKeywords: 2})
	want := []string{"one", "two"}
	assertStringSlice(t, got, want)
}

func TestExtractKeywordsMinTokenLen(t *testing.T) {
	input := "go api dev"
	got := ExtractKeywords(input, KeywordOptions{MinTokenLen: 3})
	want := []string{"api", "dev"}
	assertStringSlice(t, got, want)
}

func TestExtractKeywordsCJK(t *testing.T) {
	input := "你好 世界 hello"
	got := ExtractKeywords(input, KeywordOptions{})
	want := []string{"你好", "世界", "hello"}
	assertStringSlice(t, got, want)
}

func assertStringSlice(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("len=%d want=%d got=%v", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("idx=%d got=%q want=%q (all=%v)", i, got[i], want[i], got)
		}
	}
}
