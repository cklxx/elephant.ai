package utils

import "testing"

func TestTruncateTrimmedRunesASCII(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name    string
		input   string
		limit   int
		wantOut string
	}{
		{
			name:    "blank stays blank",
			input:   "   ",
			limit:   8,
			wantOut: "",
		},
		{
			name:    "limit non-positive returns trimmed",
			input:   "  hello  ",
			limit:   0,
			wantOut: "hello",
		},
		{
			name:    "within limit",
			input:   "  hello  ",
			limit:   10,
			wantOut: "hello",
		},
		{
			name:    "truncate ascii",
			input:   "  hello world  ",
			limit:   5,
			wantOut: "hello...",
		},
		{
			name:    "truncate rune aware",
			input:   "  你好世界再见  ",
			limit:   3,
			wantOut: "你好世...",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := TruncateTrimmedRunesASCII(tc.input, tc.limit)
			if got != tc.wantOut {
				t.Fatalf("TruncateTrimmedRunesASCII(%q, %d) = %q, want %q", tc.input, tc.limit, got, tc.wantOut)
			}
		})
	}
}
