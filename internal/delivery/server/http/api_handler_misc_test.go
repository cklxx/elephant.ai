package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestHandleDevLogIndexRequiresDevMode(t *testing.T) {
	handler := NewAPIHandler(nil, nil, nil, nil, false)
	req := httptest.NewRequest(http.MethodGet, "/api/dev/logs/index", nil)
	rec := httptest.NewRecorder()

	handler.HandleDevLogIndex(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("expected status 404, got %d", rec.Code)
	}
}

func TestHandleDevLogIndexRejectsInvalidLimit(t *testing.T) {
	handler := NewAPIHandler(nil, nil, nil, nil, false, WithDevMode(true))
	req := httptest.NewRequest(http.MethodGet, "/api/dev/logs/index?limit=0", nil)
	rec := httptest.NewRecorder()

	handler.HandleDevLogIndex(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}
}

func TestHandleDevLogIndexReturnsEntries(t *testing.T) {
	logDir := t.TempDir()
	requestDir := filepath.Join(t.TempDir(), "requests")
	if err := os.MkdirAll(requestDir, 0o755); err != nil {
		t.Fatalf("mkdir request dir: %v", err)
	}

	servicePath := filepath.Join(logDir, "alex-service.log")
	if err := os.WriteFile(servicePath, []byte(strings.Join([]string{
		"2026-02-07 12:00:00 [INFO] [SERVICE] [API] [log_id=log-ui] boot",
		"2026-02-07 12:00:01 [INFO] [SERVICE] [API] [log_id=log-ui] ready",
		"2026-02-07 12:00:02 [INFO] [SERVICE] [API] [log_id=log-ui] processing",
	}, "\n")+"\n"), 0o644); err != nil {
		t.Fatalf("write service log: %v", err)
	}

	t.Setenv("ALEX_LOG_DIR", logDir)
	t.Setenv("ALEX_REQUEST_LOG_DIR", requestDir)

	handler := NewAPIHandler(nil, nil, nil, nil, false, WithDevMode(true))
	req := httptest.NewRequest(http.MethodGet, "/api/dev/logs/index?limit=5", nil)
	rec := httptest.NewRecorder()

	handler.HandleDevLogIndex(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d", rec.Code)
	}

	var payload struct {
		Entries []struct {
			LogID string `json:"log_id"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if len(payload.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(payload.Entries))
	}
	if strings.TrimSpace(payload.Entries[0].LogID) != "log-ui" {
		t.Fatalf("unexpected log_id: %q", payload.Entries[0].LogID)
	}
}
