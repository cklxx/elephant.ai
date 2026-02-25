package llm

import (
	"strings"
	"testing"
)

func TestStreamScannerHandlesLargeSSELine(t *testing.T) {
	largePayload := "data: " + strings.Repeat("x", 700*1024)
	input := largePayload + "\n" + "data: [DONE]\n"

	scanner := newStreamScanner(strings.NewReader(input))
	var lines []string
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("scanner error: %v", err)
	}
	if len(lines) != 2 {
		t.Fatalf("expected 2 lines, got %d", len(lines))
	}
	if lines[0] != largePayload {
		t.Fatalf("large line mismatch, got len=%d want len=%d", len(lines[0]), len(largePayload))
	}
	if lines[1] != "data: [DONE]" {
		t.Fatalf("expected done line, got %q", lines[1])
	}
}

func TestStreamScannerPreservesFinalLineWithoutNewline(t *testing.T) {
	scanner := newStreamScanner(strings.NewReader("data: final"))

	if !scanner.Scan() {
		t.Fatalf("expected first scan token, err=%v", scanner.Err())
	}
	if scanner.Text() != "data: final" {
		t.Fatalf("unexpected token: %q", scanner.Text())
	}
	if scanner.Scan() {
		t.Fatalf("expected scanner to stop")
	}
	if err := scanner.Err(); err != nil {
		t.Fatalf("unexpected scanner error: %v", err)
	}
}
