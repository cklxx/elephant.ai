package agent_eval

import (
	"testing"
	"time"
)

func TestParseOptionalRFC3339(t *testing.T) {
	t.Run("empty input", func(t *testing.T) {
		got, ok, err := ParseOptionalRFC3339("   ")
		if err != nil {
			t.Fatalf("ParseOptionalRFC3339 returned error: %v", err)
		}
		if ok {
			t.Fatalf("ok = true, want false")
		}
		if !got.IsZero() {
			t.Fatalf("expected zero time, got %v", got)
		}
	})

	t.Run("valid timestamp", func(t *testing.T) {
		got, ok, err := ParseOptionalRFC3339(" 2026-02-01T12:13:14Z ")
		if err != nil {
			t.Fatalf("ParseOptionalRFC3339 returned error: %v", err)
		}
		if !ok {
			t.Fatalf("ok = false, want true")
		}
		want := time.Date(2026, 2, 1, 12, 13, 14, 0, time.UTC)
		if !got.Equal(want) {
			t.Fatalf("got %s, want %s", got.Format(time.RFC3339), want.Format(time.RFC3339))
		}
	})

	t.Run("invalid timestamp", func(t *testing.T) {
		_, ok, err := ParseOptionalRFC3339("invalid")
		if err == nil {
			t.Fatal("expected error")
		}
		if ok {
			t.Fatalf("ok = true, want false")
		}
	})
}
