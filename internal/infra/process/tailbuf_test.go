package process

import (
	"strings"
	"sync"
	"testing"
)

func TestTailBuffer_Empty(t *testing.T) {
	tb := NewTailBuffer(64)
	if s := tb.String(); s != "" {
		t.Fatalf("expected empty, got %q", s)
	}
}

func TestTailBuffer_SmallWrite(t *testing.T) {
	tb := NewTailBuffer(64)
	tb.Write([]byte("hello"))
	if s := tb.String(); s != "hello" {
		t.Fatalf("got %q", s)
	}
}

func TestTailBuffer_ExactMax(t *testing.T) {
	tb := NewTailBuffer(5)
	tb.Write([]byte("abcde"))
	if s := tb.String(); s != "abcde" {
		t.Fatalf("got %q", s)
	}
}

func TestTailBuffer_OverflowSingleWrite(t *testing.T) {
	tb := NewTailBuffer(5)
	tb.Write([]byte("abcdefgh"))
	if s := tb.String(); s != "defgh" {
		t.Fatalf("got %q, want %q", s, "defgh")
	}
}

func TestTailBuffer_OverflowMultipleWrites(t *testing.T) {
	tb := NewTailBuffer(8)
	tb.Write([]byte("aaaa"))
	tb.Write([]byte("bbbb"))
	tb.Write([]byte("cc"))
	// total 10 bytes, keep last 8: "aabbbbcc" → overflow → "bbbbcc" with max 8
	s := tb.String()
	if len(s) > 8 {
		t.Fatalf("exceeded max: len=%d", len(s))
	}
	if !strings.HasSuffix(s, "cc") {
		t.Fatalf("expected suffix 'cc', got %q", s)
	}
}

func TestTailBuffer_ZeroWrite(t *testing.T) {
	tb := NewTailBuffer(64)
	n, err := tb.Write(nil)
	if n != 0 || err != nil {
		t.Fatalf("n=%d err=%v", n, err)
	}
	if s := tb.String(); s != "" {
		t.Fatalf("got %q", s)
	}
}

func TestTailBuffer_DefaultSize(t *testing.T) {
	tb := NewTailBuffer(0)
	if tb.max != DefaultStderrTail {
		t.Fatalf("expected default %d, got %d", DefaultStderrTail, tb.max)
	}
}

func TestTailBuffer_Concurrent(t *testing.T) {
	tb := NewTailBuffer(1024)
	var wg sync.WaitGroup
	for i := range 10 {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for range 100 {
				tb.Write([]byte("data"))
				_ = tb.String()
			}
		}(i)
	}
	wg.Wait()
	s := tb.String()
	if len(s) > 1024 {
		t.Fatalf("exceeded max: len=%d", len(s))
	}
}
