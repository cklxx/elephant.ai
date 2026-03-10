package http

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestLeaderOpenAPISpec_ValidJSON(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(LeaderOpenAPISpec), &parsed); err != nil {
		t.Fatalf("LeaderOpenAPISpec is not valid JSON: %v", err)
	}
}

func TestLeaderOpenAPISpec_HasRequiredTopLevelKeys(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(LeaderOpenAPISpec), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}

	for _, key := range []string{"openapi", "info", "paths", "components"} {
		if _, ok := parsed[key]; !ok {
			t.Errorf("missing top-level key %q", key)
		}
	}

	if v, _ := parsed["openapi"].(string); v != "3.0.3" {
		t.Errorf("openapi version = %q, want 3.0.3", v)
	}
}

func TestLeaderOpenAPISpec_ContainsAllPaths(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(LeaderOpenAPISpec), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}

	paths, ok := parsed["paths"].(map[string]any)
	if !ok {
		t.Fatal("paths is not an object")
	}

	expected := []string{"/dashboard", "/openapi.json", "/tasks", "/tasks/{id}/unblock"}
	for _, p := range expected {
		if _, ok := paths[p]; !ok {
			t.Errorf("missing path %q", p)
		}
	}
}

func TestLeaderOpenAPISpec_DashboardIsGet(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(LeaderOpenAPISpec), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}

	dashboard := parsed["paths"].(map[string]any)["/dashboard"].(map[string]any)
	if _, ok := dashboard["get"]; !ok {
		t.Error("/dashboard should have a GET operation")
	}
}

func TestLeaderOpenAPISpec_UnblockIsPost(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(LeaderOpenAPISpec), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}

	unblock := parsed["paths"].(map[string]any)["/tasks/{id}/unblock"].(map[string]any)
	if _, ok := unblock["post"]; !ok {
		t.Error("/tasks/{id}/unblock should have a POST operation")
	}
}

func TestLeaderOpenAPISpec_ComponentSchemas(t *testing.T) {
	var parsed map[string]any
	if err := json.Unmarshal([]byte(LeaderOpenAPISpec), &parsed); err != nil {
		t.Fatalf("parse: %v", err)
	}

	components, ok := parsed["components"].(map[string]any)
	if !ok {
		t.Fatal("components is not an object")
	}
	schemas, ok := components["schemas"].(map[string]any)
	if !ok {
		t.Fatal("components.schemas is not an object")
	}

	expected := []string{
		"DashboardResponse",
		"TaskStatusCounts",
		"BlockerAlert",
		"DailySummary",
		"ScheduledJob",
		"TaskListResponse",
		"TaskSummary",
		"UnblockRequest",
		"UnblockResponse",
		"ErrorResponse",
	}
	for _, name := range expected {
		if _, ok := schemas[name]; !ok {
			t.Errorf("missing schema %q", name)
		}
	}
}

func TestHandleLeaderOpenAPISpec_StatusAndContentType(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/leader/openapi.json", nil)
	rec := httptest.NewRecorder()

	HandleLeaderOpenAPISpec(rec, req)

	if rec.Code != http.StatusOK {
		t.Errorf("status = %d, want %d", rec.Code, http.StatusOK)
	}
	ct := rec.Header().Get("Content-Type")
	if ct != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", ct)
	}
	cc := rec.Header().Get("Cache-Control")
	if cc == "" {
		t.Error("expected Cache-Control header to be set")
	}
}

func TestHandleLeaderOpenAPISpec_BodyIsValidJSON(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/leader/openapi.json", nil)
	rec := httptest.NewRecorder()

	HandleLeaderOpenAPISpec(rec, req)

	var parsed map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &parsed); err != nil {
		t.Fatalf("response body is not valid JSON: %v", err)
	}
	if _, ok := parsed["openapi"]; !ok {
		t.Error("response body missing openapi key")
	}
}

func TestHandleLeaderOpenAPISpec_BodyMatchesConstant(t *testing.T) {
	req := httptest.NewRequest(http.MethodGet, "/api/leader/openapi.json", nil)
	rec := httptest.NewRecorder()

	HandleLeaderOpenAPISpec(rec, req)

	body := rec.Body.String()
	if body != LeaderOpenAPISpec {
		t.Error("response body does not match LeaderOpenAPISpec constant")
	}
}
