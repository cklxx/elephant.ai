package inlinepayload

import "testing"

func TestShouldRetain(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name      string
		mediaType string
		size      int
		limit     int
		want      bool
	}{
		{
			name:      "reject non-positive size",
			mediaType: "text/plain",
			size:      0,
			limit:     128 * 1024,
			want:      false,
		},
		{
			name:      "reject over limit",
			mediaType: "text/plain",
			size:      128*1024 + 1,
			limit:     128 * 1024,
			want:      false,
		},
		{
			name:      "reject empty media type",
			mediaType: "   ",
			size:      10,
			limit:     128 * 1024,
			want:      false,
		},
		{
			name:      "retain text media",
			mediaType: " Text/Plain ",
			size:      10,
			limit:     128 * 1024,
			want:      true,
		},
		{
			name:      "retain json media",
			mediaType: "application/vnd.api+json",
			size:      10,
			limit:     128 * 1024,
			want:      true,
		},
		{
			name:      "retain markdown media",
			mediaType: "application/markdown",
			size:      10,
			limit:     128 * 1024,
			want:      true,
		},
		{
			name:      "reject binary media",
			mediaType: "application/octet-stream",
			size:      10,
			limit:     128 * 1024,
			want:      false,
		},
		{
			name:      "respect caller-specific limit",
			mediaType: "text/plain",
			size:      65,
			limit:     64,
			want:      false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			if got := ShouldRetain(tt.mediaType, tt.size, tt.limit); got != tt.want {
				t.Fatalf("ShouldRetain(%q, %d, %d) = %v, want %v", tt.mediaType, tt.size, tt.limit, got, tt.want)
			}
		})
	}
}
