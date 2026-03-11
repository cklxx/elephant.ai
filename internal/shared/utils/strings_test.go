package utils

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"within limit", "hello", 10, "hello"},
		{"exact limit", "hello", 5, "hello"},
		{"over limit", "hello world", 5, "hello"},
		{"zero limit", "hello", 0, ""},
		{"negative limit", "hello", -1, ""},
		{"empty input", "", 5, ""},
		{"whitespace only", "  ", 5, ""},
		{"trims input", "  hello  ", 5, "hello"},
		{"rune aware", "你好世界", 2, "你好"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.input, tc.max)
			if got != tc.expected {
				t.Errorf("Truncate(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.expected)
			}
		})
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		expected string
	}{
		{"within limit", "hello", 10, "hello"},
		{"exact limit", "hello", 5, "hello"},
		{"over limit", "hello world", 5, "hello..."},
		{"zero limit", "hello", 0, ""},
		{"negative limit", "hello", -1, ""},
		{"empty input", "", 5, ""},
		{"whitespace only", "  ", 5, ""},
		{"trims input", "  hello world  ", 5, "hello..."},
		{"rune aware", "你好世界再见", 3, "你好世..."},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := TruncateWithEllipsis(tc.input, tc.max)
			if got != tc.expected {
				t.Errorf("TruncateWithEllipsis(%q, %d) = %q, want %q", tc.input, tc.max, got, tc.expected)
			}
		})
	}
}
