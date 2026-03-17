package utils

import "testing"

func TestTruncate(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		max      int
		suffix   string
		expected string
	}{
		{"within limit", "hello", 10, "...", "hello"},
		{"exact limit", "hello", 5, "...", "hello"},
		{"over limit", "hello world", 8, "...", "hello..."},
		{"no suffix", "hello world", 5, "", "hello"},
		{"zero limit", "hello", 0, "...", "..."},
		{"max equals suffix len", "hello", 3, "...", "..."},
		{"rune aware", "你好世界再见", 5, "...", "你好..."},
		{"single char suffix", "hello world", 6, "…", "hello…"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := Truncate(tc.input, tc.max, tc.suffix)
			if got != tc.expected {
				t.Errorf("Truncate(%q, %d, %q) = %q, want %q", tc.input, tc.max, tc.suffix, got, tc.expected)
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
		{"over limit", "hello world", 8, "hello..."},
		{"max equals 3", "hello", 3, "..."},
		{"rune aware", "你好世界再见", 5, "你好..."},
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
