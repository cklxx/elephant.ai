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
	tests := []struct {
		name  string
		raw   string
		want  []string
		isNil bool
	}{
		{
			name: "mixed separators and duplicates",
			raw:  " SMILE, WAVE ;SMILE\tTHINKING|WAVE ",
			want: []string{"SMILE", "WAVE", "THINKING"},
		},
		{
			name:  "separator only",
			raw:   " , ; |\t\n ",
			isNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pool := parseEmojiPool(tt.raw)
			if tt.isNil {
				if pool != nil {
					t.Fatalf("expected nil pool, got %v", pool)
				}
				return
			}
			if !reflect.DeepEqual(pool, tt.want) {
				t.Fatalf("expected %v, got %v", tt.want, pool)
			}
		})
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
