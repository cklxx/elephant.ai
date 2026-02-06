package tokenutil

import (
	"strings"
	"testing"
)

func TestCountTokens_Empty(t *testing.T) {
	if got := CountTokens(""); got != 0 {
		t.Errorf("CountTokens(\"\") = %d, want 0", got)
	}
}

func TestCountTokens_Simple(t *testing.T) {
	got := CountTokens("hello world")
	if got <= 0 {
		t.Errorf("CountTokens(\"hello world\") = %d, want > 0", got)
	}
	// "hello world" is 2 tokens with cl100k_base
	if encoding != nil && got != 2 {
		t.Errorf("CountTokens(\"hello world\") = %d, want 2 (tiktoken)", got)
	}
}

func TestCountTokens_LongerText(t *testing.T) {
	text := "The quick brown fox jumps over the lazy dog"
	got := CountTokens(text)
	if got <= 0 {
		t.Errorf("CountTokens(%q) = %d, want > 0", text, got)
	}
	// Should produce a reasonable count (not the runes/4 fallback)
	if encoding != nil && got > 20 {
		t.Errorf("CountTokens(%q) = %d, suspiciously high for tiktoken", text, got)
	}
}

func TestEstimateFast_Empty(t *testing.T) {
	if got := EstimateFast(""); got != 0 {
		t.Errorf("EstimateFast(\"\") = %d, want 0", got)
	}
}

func TestEstimateFast_Whitespace(t *testing.T) {
	if got := EstimateFast("   \n\t  "); got != 0 {
		t.Errorf("EstimateFast(whitespace) = %d, want 0", got)
	}
}

func TestEstimateFast_MinWordCount(t *testing.T) {
	// "a b c d" has 4 words, 7 runes → runes/4=1, but word count=4 → max is 4
	got := EstimateFast("a b c d")
	if got != 4 {
		t.Errorf("EstimateFast(\"a b c d\") = %d, want 4", got)
	}
}

func TestTruncateToTokens_NoTruncation(t *testing.T) {
	text := "short"
	got := TruncateToTokens(text, 100)
	if got != text {
		t.Errorf("TruncateToTokens(%q, 100) = %q, want %q", text, got, text)
	}
}

func TestTruncateToTokens_ZeroMax(t *testing.T) {
	text := "anything"
	got := TruncateToTokens(text, 0)
	if got != text {
		t.Errorf("TruncateToTokens(%q, 0) = %q, want %q (no-op for zero)", text, got, text)
	}
}

func TestTruncateToTokens_ActualTruncation(t *testing.T) {
	text := strings.Repeat("hello world ", 100) // ~200+ tokens
	got := TruncateToTokens(text, 5)
	if got == text {
		t.Error("TruncateToTokens should have truncated long text")
	}
	if !strings.HasSuffix(got, "...") {
		t.Errorf("truncated result should end with '...', got %q", got[len(got)-20:])
	}
}
