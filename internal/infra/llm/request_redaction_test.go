package llm

import (
	"bytes"
	"testing"
)

func TestRedactDataURIs(t *testing.T) {
	tests := []struct {
		name     string
		input    []byte
		expected []byte
	}{
		{
			name:     "empty input returns empty",
			input:    []byte{},
			expected: []byte{},
		},
		{
			name:     "nil input returns nil",
			input:    nil,
			expected: nil,
		},
		{
			name:     "no data URIs returns input unchanged",
			input:    []byte(`{"text":"hello world"}`),
			expected: []byte(`{"text":"hello world"}`),
		},
		{
			name:     "single data URI image png redacted",
			input:    []byte(`data:image/png;base64,iVBORw0KGgo=`),
			expected: []byte(`data:image/png;base64,<redacted>`),
		},
		{
			name: "multiple data URIs all redacted",
			input: []byte(
				`data:image/png;base64,abc123 and data:audio/mp3;base64,xyz789 and data:application/pdf;base64,AA_BB+CC/==`,
			),
			expected: []byte(
				`data:image/png;base64,<redacted> and data:audio/mp3;base64,<redacted> and data:application/pdf;base64,<redacted>`,
			),
		},
		{
			name: "data URI in JSON payload preserves structure",
			input: []byte(
				`{"messages":[{"role":"user","content":"describe this","image_url":"data:image/jpeg;base64,/9j/4AAQSkZJRgABAQAAAQABAAD"}],"meta":{"request_id":"req-1"}}`,
			),
			expected: []byte(
				`{"messages":[{"role":"user","content":"describe this","image_url":"data:image/jpeg;base64,<redacted>"}],"meta":{"request_id":"req-1"}}`,
			),
		},
		{
			name:     "already redacted URI unchanged",
			input:    []byte(`data:image/png;base64,<redacted>`),
			expected: []byte(`data:image/png;base64,<redacted>`),
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			got := redactDataURIs(tt.input)
			if tt.expected == nil {
				if got != nil {
					t.Fatalf("expected nil, got %q", got)
				}
				return
			}
			if got == nil {
				t.Fatalf("expected %q, got nil", tt.expected)
			}
			if !bytes.Equal(got, tt.expected) {
				t.Fatalf("expected %q, got %q", tt.expected, got)
			}
		})
	}
}
