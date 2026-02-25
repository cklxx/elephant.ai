package llm

import (
	"bytes"
	"testing"
)

func TestReadLimitedBodyWithinLimit(t *testing.T) {
	payload := []byte("hello")
	got, err := readLimitedBody(bytes.NewBuffer(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(got) != string(payload) {
		t.Fatalf("expected %q, got %q", payload, got)
	}
}

func TestReadLimitedBodyRejectsLargePayload(t *testing.T) {
	payload := bytes.Repeat([]byte("a"), 11)
	_, err := readLimitedBody(bytes.NewBuffer(payload), 10)
	if err == nil {
		t.Fatal("expected error for oversized payload")
	}
}

func TestReadLimitedBodyUnlimited(t *testing.T) {
	payload := bytes.Repeat([]byte("b"), 32)
	got, err := readLimitedBody(bytes.NewBuffer(payload), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != len(payload) {
		t.Fatalf("expected %d bytes, got %d", len(payload), len(got))
	}
}
