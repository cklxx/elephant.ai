package textutil

import "testing"

func TestTruncateWithEllipsis(t *testing.T) {
	got := TruncateWithEllipsis("hello world", 5)
	want := "hell…"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestTruncateWithEllipsisLimitOne(t *testing.T) {
	got := TruncateWithEllipsis("hello", 1)
	want := "…"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}
