package utils

import "testing"

func TestTruncateWithSuffix(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		value    string
		maxRunes int
		suffix   string
		want     string
	}{
		{
			name:     "no truncation",
			value:    "hello",
			maxRunes: 8,
			suffix:   "...",
			want:     "hello",
		},
		{
			name:     "ascii truncation",
			value:    "hello world",
			maxRunes: 5,
			suffix:   "...",
			want:     "hello...",
		},
		{
			name:     "unicode truncation",
			value:    "你好世界和平",
			maxRunes: 2,
			suffix:   "…",
			want:     "你好…",
		},
		{
			name:     "empty suffix",
			value:    "abcdef",
			maxRunes: 3,
			suffix:   "",
			want:     "abc",
		},
		{
			name:     "non-positive max",
			value:    "abcdef",
			maxRunes: 0,
			suffix:   "...",
			want:     "",
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateWithSuffix(tt.value, tt.maxRunes, tt.suffix)
			if got != tt.want {
				t.Fatalf("TruncateWithSuffix(%q, %d, %q) = %q, want %q", tt.value, tt.maxRunes, tt.suffix, got, tt.want)
			}
		})
	}
}
