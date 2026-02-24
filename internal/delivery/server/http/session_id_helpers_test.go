package http

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestExtractRequiredSessionIDFromPath(t *testing.T) {
	tests := []struct {
		name      string
		pathValue string
		wantID    string
		wantErr   string
	}{
		{
			name:      "trimmed valid id",
			pathValue: "  sess-1  ",
			wantID:    "sess-1",
		},
		{
			name:      "empty id",
			pathValue: "   ",
			wantErr:   "session_id is required",
		},
		{
			name:      "invalid characters",
			pathValue: "../bad",
			wantErr:   "session_id contains invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sessions/demo", nil)
			req.SetPathValue("session_id", tt.pathValue)

			gotID, err := extractRequiredSessionIDFromPath(req)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotID != tt.wantID {
				t.Fatalf("expected session id %q, got %q", tt.wantID, gotID)
			}
		})
	}
}

func TestExtractRequiredSessionIDFromQuery(t *testing.T) {
	tests := []struct {
		name       string
		queryValue string
		wantID     string
		wantErr    string
	}{
		{
			name:       "trimmed valid id",
			queryValue: "  sess-2  ",
			wantID:     "sess-2",
		},
		{
			name:       "missing id",
			queryValue: "   ",
			wantErr:    "session_id is required",
		},
		{
			name:       "invalid characters",
			queryValue: "../bad",
			wantErr:    "session_id contains invalid characters",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, "/api/sse", nil)
			query := req.URL.Query()
			query.Set("session_id", tt.queryValue)
			req.URL.RawQuery = query.Encode()

			gotID, err := extractRequiredSessionIDFromQuery(req)
			if tt.wantErr != "" {
				if err == nil {
					t.Fatalf("expected error %q, got nil", tt.wantErr)
				}
				if err.Error() != tt.wantErr {
					t.Fatalf("expected error %q, got %q", tt.wantErr, err.Error())
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotID != tt.wantID {
				t.Fatalf("expected session id %q, got %q", tt.wantID, gotID)
			}
		})
	}
}
