package http

import (
	"fmt"
	"net/http"
	"testing"

	"alex/internal/delivery/server/app"
	storage "alex/internal/domain/agent/ports/storage"
)

func TestMapDomainError(t *testing.T) {
	tests := []struct {
		name       string
		err        error
		wantStatus int
		wantMsg    string
	}{
		{
			name:       "nil error",
			err:        nil,
			wantStatus: 0,
		},
		{
			name:       "ErrValidation",
			err:        app.ValidationError("bad input"),
			wantStatus: http.StatusBadRequest,
			wantMsg:    "bad input: validation error",
		},
		{
			name:       "ErrNotFound",
			err:        app.NotFoundError("session xyz"),
			wantStatus: http.StatusNotFound,
			wantMsg:    "session xyz: not found",
		},
		{
			name:       "ErrSessionNotFound from storage",
			err:        storage.ErrSessionNotFound,
			wantStatus: http.StatusNotFound,
			wantMsg:    "Session not found",
		},
		{
			name:       "wrapped ErrSessionNotFound",
			err:        fmt.Errorf("lookup failed: %w", storage.ErrSessionNotFound),
			wantStatus: http.StatusNotFound,
			wantMsg:    "Session not found",
		},
		{
			name:       "ErrShareTokenInvalid",
			err:        app.ErrShareTokenInvalid,
			wantStatus: http.StatusForbidden,
			wantMsg:    "Invalid share token",
		},
		{
			name:       "ErrConflict",
			err:        app.ConflictError("cannot cancel"),
			wantStatus: http.StatusConflict,
			wantMsg:    "cannot cancel: conflict",
		},
		{
			name:       "ErrUnavailable",
			err:        app.UnavailableError("not configured"),
			wantStatus: http.StatusServiceUnavailable,
			wantMsg:    "not configured: service unavailable",
		},
		{
			name:       "unknown error returns zero",
			err:        fmt.Errorf("something unexpected"),
			wantStatus: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, msg := mapDomainError(tt.err)
			if status != tt.wantStatus {
				t.Errorf("status = %d, want %d", status, tt.wantStatus)
			}
			if tt.wantMsg != "" && msg != tt.wantMsg {
				t.Errorf("msg = %q, want %q", msg, tt.wantMsg)
			}
		})
	}
}
