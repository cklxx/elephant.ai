package main

import "testing"

func TestEmitTypewriterPreservesRunes(t *testing.T) {
	var out string
	emitTypewriter("hi好", func(s string) {
		out += s
	})
	if out != "hi好" {
		t.Fatalf("expected output to preserve runes, got %q", out)
	}
}
