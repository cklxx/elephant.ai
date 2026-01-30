package textutil

import "testing"

func TestSmartTruncateShort(t *testing.T) {
	input := "short"
	if got := SmartTruncate(input, 10); got != input {
		t.Fatalf("got=%q want=%q", got, input)
	}
}

func TestSmartTruncateLimitUnderTen(t *testing.T) {
	input := "0123456789abcdef"
	got := SmartTruncate(input, 10)
	want := "0123456789"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

func TestSmartTruncateHeadTail(t *testing.T) {
	input := "0123456789abcdef"
	got := SmartTruncate(input, 15)
	want := "012345678 ... f"
	if got != want {
		t.Fatalf("got=%q want=%q", got, want)
	}
}

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
