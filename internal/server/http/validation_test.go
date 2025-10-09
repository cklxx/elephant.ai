package http

import (
	"strings"
	"testing"
)

func TestValidateSessionID(t *testing.T) {
	tests := []struct {
		name    string
		id      string
		wantErr bool
	}{
		{name: "valid uuid", id: "session-1234", wantErr: false},
		{name: "valid underscore", id: "session_test", wantErr: false},
		{name: "empty", id: "", wantErr: true},
		{name: "whitespace", id: "   ", wantErr: true},
		{name: "too long", id: strings.Repeat("a", maxSessionIDLength+1), wantErr: true},
		{name: "path traversal", id: "../etc/passwd", wantErr: true},
		{name: "invalid chars", id: "session$bad", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateSessionID(tt.id)
			if tt.wantErr && err == nil {
				t.Fatalf("expected error for %q", tt.id)
			}
			if !tt.wantErr && err != nil {
				t.Fatalf("unexpected error for %q: %v", tt.id, err)
			}
		})
	}
}

func TestIsValidOptionalSessionID(t *testing.T) {
	id, err := isValidOptionalSessionID("  session-abc  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id != "session-abc" {
		t.Fatalf("expected trimmed session id, got %q", id)
	}

	if _, err := isValidOptionalSessionID("../../etc/passwd"); err == nil {
		t.Fatalf("expected error for traversal id")
	}
}
