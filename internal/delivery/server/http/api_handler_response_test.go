package http

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestParseOptionalQueryIntDefaultAndClamp(t *testing.T) {
	handler := NewAPIHandler(nil, nil, nil, nil, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions", nil)
	value, ok := handler.parseOptionalQueryInt(
		rec,
		req,
		"limit",
		50,
		1,
		200,
		"limit must be a positive integer",
		nil,
	)
	if !ok {
		t.Fatal("expected parser to accept missing query value")
	}
	if value != 50 {
		t.Fatalf("expected default value 50, got %d", value)
	}
	if rec.Body.Len() != 0 {
		t.Fatalf("expected no response body on success, got %q", rec.Body.String())
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/sessions?limit=999", nil)
	value, ok = handler.parseOptionalQueryInt(
		rec,
		req,
		"limit",
		50,
		1,
		200,
		"limit must be a positive integer",
		nil,
	)
	if !ok {
		t.Fatal("expected parser to accept numeric query value")
	}
	if value != 200 {
		t.Fatalf("expected capped value 200, got %d", value)
	}
}

func TestParseOptionalQueryIntInvalidSyntax(t *testing.T) {
	handler := NewAPIHandler(nil, nil, nil, nil, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions?limit=abc", nil)
	_, ok := handler.parseOptionalQueryInt(
		rec,
		req,
		"limit",
		50,
		1,
		200,
		"limit must be a positive integer",
		nil,
	)
	if ok {
		t.Fatal("expected parser to reject invalid integer")
	}
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400, got %d", rec.Code)
	}

	var payload apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Error != "limit must be a positive integer" {
		t.Fatalf("unexpected error message: %q", payload.Error)
	}
	if !strings.Contains(payload.Details, "invalid syntax") {
		t.Fatalf("expected parse details, got %q", payload.Details)
	}
}

func TestParseOptionalQueryIntRangeErrors(t *testing.T) {
	handler := NewAPIHandler(nil, nil, nil, nil, false)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/api/sessions?limit=0", nil)
	_, ok := handler.parseOptionalQueryInt(
		rec,
		req,
		"limit",
		50,
		1,
		200,
		"limit must be a positive integer",
		nil,
	)
	if ok {
		t.Fatal("expected parser to reject non-positive integer")
	}
	var payload apiErrorResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Details != "" {
		t.Fatalf("expected empty details when no range error provided, got %q", payload.Details)
	}

	rec = httptest.NewRecorder()
	req = httptest.NewRequest(http.MethodGet, "/api/dev/logs/index?limit=0", nil)
	rangeErr := errors.New("limit must be > 0")
	_, ok = handler.parseOptionalQueryInt(
		rec,
		req,
		"limit",
		80,
		1,
		0,
		"limit must be a positive integer",
		rangeErr,
	)
	if ok {
		t.Fatal("expected parser to reject non-positive integer")
	}
	payload = apiErrorResponse{}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if payload.Details != rangeErr.Error() {
		t.Fatalf("expected range error details %q, got %q", rangeErr.Error(), payload.Details)
	}
}
