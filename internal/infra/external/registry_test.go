package external

import (
	"testing"
)

func TestPickFirstNonEmpty(t *testing.T) {
	tests := []struct {
		values []string
		want   string
	}{
		{nil, ""},
		{[]string{}, ""},
		{[]string{"", "", "c"}, "c"},
		{[]string{"a", "b"}, "a"},
		{[]string{"  ", "b"}, "b"},
		{[]string{" a "}, "a"},
	}
	for _, tt := range tests {
		got := pickFirstNonEmpty(tt.values...)
		if got != tt.want {
			t.Errorf("pickFirstNonEmpty(%v) = %q, want %q", tt.values, got, tt.want)
		}
	}
}

func TestRequestKey(t *testing.T) {
	got := requestKey("task-1", "req-1")
	if got != "task-1:req-1" {
		t.Errorf("expected task-1:req-1, got %s", got)
	}
}

func TestRequestKey_WithSpaces(t *testing.T) {
	got := requestKey("  task  ", "  req  ")
	if got != "task:req" {
		t.Errorf("expected task:req, got %s", got)
	}
}
