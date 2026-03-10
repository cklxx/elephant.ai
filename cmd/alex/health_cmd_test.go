package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestFetchHealth_Success(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status: "healthy",
			Components: []healthComponent{
				{Name: "llm_factory", Status: "ready", Message: "LLM factory initialized"},
				{Name: "bootstrap", Status: "ready", Message: "All optional components initialized"},
				{Name: "llm_models", Status: "ready", Message: "3 models healthy"},
			},
		})
	}))
	defer srv.Close()

	resp, err := fetchHealth(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "healthy" {
		t.Errorf("status = %q, want %q", resp.Status, "healthy")
	}
	if len(resp.Components) != 3 {
		t.Errorf("components count = %d, want 3", len(resp.Components))
	}
}

func TestFetchHealth_Degraded(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status: "degraded",
			Components: []healthComponent{
				{Name: "llm_factory", Status: "ready"},
				{Name: "bootstrap", Status: "not_ready", Message: "Some optional components failed"},
			},
		})
	}))
	defer srv.Close()

	resp, err := fetchHealth(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "degraded" {
		t.Errorf("status = %q, want %q", resp.Status, "degraded")
	}
}

func TestFetchHealth_Unhealthy(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusServiceUnavailable)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status: "unhealthy",
			Components: []healthComponent{
				{Name: "llm_factory", Status: "error", Message: "LLM factory not initialized"},
			},
		})
	}))
	defer srv.Close()

	resp, err := fetchHealth(srv.URL)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp.Status != "unhealthy" {
		t.Errorf("status = %q, want %q", resp.Status, "unhealthy")
	}
}

func TestFetchHealth_ServerDown(t *testing.T) {
	_, err := fetchHealth("http://127.0.0.1:19999/health")
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
	if !strings.Contains(err.Error(), "connect to server") {
		t.Errorf("error = %q, want it to mention connection failure", err.Error())
	}
}

func TestRunHealthCommand_JSON(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(healthResponse{
			Status: "healthy",
			Components: []healthComponent{
				{Name: "llm_factory", Status: "ready", Message: "ok"},
			},
		})
	}))
	defer srv.Close()

	err := runHealthCommand([]string{"--json", "--url", srv.URL})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestRunHealthCommand_Unreachable(t *testing.T) {
	err := runHealthCommand([]string{"--url", "http://127.0.0.1:19999/health"})
	if err == nil {
		t.Fatal("expected error for unreachable server")
	}
}

func TestRunHealthCommand_UnreachableJSON(t *testing.T) {
	err := runHealthCommand([]string{"--json", "--url", "http://127.0.0.1:19999/health"})
	// --json unreachable still returns nil (prints JSON to stdout) but the JSON has status "unreachable"
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPrintHealthHuman_Healthy(t *testing.T) {
	var buf strings.Builder
	printHealthHuman(&buf, &healthResponse{
		Status: "healthy",
		Components: []healthComponent{
			{Name: "llm_factory", Status: "ready", Message: "initialized"},
			{Name: "bootstrap", Status: "disabled"},
		},
	})
	out := buf.String()
	if !strings.Contains(out, "HEALTHY") {
		t.Errorf("output should contain HEALTHY, got:\n%s", out)
	}
	if !strings.Contains(out, "llm_factory") {
		t.Errorf("output should contain component name, got:\n%s", out)
	}
}

func TestPrintHealthHuman_Unhealthy(t *testing.T) {
	var buf strings.Builder
	printHealthHuman(&buf, &healthResponse{
		Status: "unhealthy",
		Components: []healthComponent{
			{Name: "llm_factory", Status: "error", Message: "down"},
		},
	})
	out := buf.String()
	if !strings.Contains(out, "UNHEALTHY") {
		t.Errorf("output should contain UNHEALTHY, got:\n%s", out)
	}
}

func TestStatusIcon(t *testing.T) {
	tests := []struct {
		status string
		want   string
	}{
		{"ready", "✓"},
		{"disabled", "-"},
		{"error", "✗"},
		{"not_ready", "!"},
		{"unknown", "!"},
	}
	for _, tt := range tests {
		if got := statusIcon(tt.status); got != tt.want {
			t.Errorf("statusIcon(%q) = %q, want %q", tt.status, got, tt.want)
		}
	}
}
