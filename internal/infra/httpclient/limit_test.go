package httpclient

import (
	"bytes"
	"testing"
)

func TestReadAllWithLimitWithinLimit(t *testing.T) {
	payload := []byte("hello")
	got, err := ReadAllWithLimit(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %q, got %q", payload, got)
	}
}

func TestReadAllWithLimitTooLarge(t *testing.T) {
	payload := []byte("hello")
	_, err := ReadAllWithLimit(bytes.NewReader(payload), 2)
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsResponseTooLarge(err) {
		t.Fatalf("expected ResponseTooLargeError, got %v", err)
	}
}

func TestReadAllWithLimitUnlimited(t *testing.T) {
	payload := []byte("hello")
	got, err := ReadAllWithLimit(bytes.NewReader(payload), 0)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !bytes.Equal(got, payload) {
		t.Fatalf("expected %q, got %q", payload, got)
	}
}
