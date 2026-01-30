package textutil

import "testing"

func TestNormalizeWhitespace(t *testing.T) {
	input := "foo  \n  bar\tbaz"
	got := NormalizeWhitespace(input)
	want := "foo bar baz"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}
