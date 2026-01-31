package lark

import (
	"reflect"
	"testing"
)

func TestResolveEmojiPoolUsesDefaultWhenTooSmall(t *testing.T) {
	pool := resolveEmojiPool("")
	if !reflect.DeepEqual(pool, defaultEmojiPool) {
		t.Fatalf("expected default pool for empty input, got %v", pool)
	}

	pool = resolveEmojiPool("SMILE")
	if !reflect.DeepEqual(pool, defaultEmojiPool) {
		t.Fatalf("expected default pool for single emoji, got %v", pool)
	}
}

func TestParseEmojiPoolDedupesAndParses(t *testing.T) {
	pool := parseEmojiPool(" SMILE, WAVE ,SMILE\tTHINKING ")
	want := []string{"SMILE", "WAVE", "THINKING"}
	if !reflect.DeepEqual(pool, want) {
		t.Fatalf("expected %v, got %v", want, pool)
	}
}

func TestEmojiPickerPickStartEndDistinct(t *testing.T) {
	picker := newEmojiPicker(1, []string{"A", "B", "C"})
	start, end := picker.pickStartEnd()
	if start == "" || end == "" {
		t.Fatalf("expected non-empty emojis, got %q/%q", start, end)
	}
	if start == end {
		t.Fatalf("expected distinct emojis, got %q/%q", start, end)
	}
}

func TestEmojiPickerPickStartEndSingle(t *testing.T) {
	picker := newEmojiPicker(1, []string{"ONLY"})
	start, end := picker.pickStartEnd()
	if start != "ONLY" || end != "ONLY" {
		t.Fatalf("expected ONLY/ONLY, got %q/%q", start, end)
	}
}
