package http

import (
	"encoding/json"
	"net/http/httptest"
	"testing"

	"alex/internal/di"
	"alex/internal/server/app"
	"alex/internal/server/ports"
)

func TestHealthEndpoint_Integration(t *testing.T) {
	// Create container with features disabled for clean test
	config := di.Config{
		LLMProvider: "mock",
		LLMModel:    "test",
		EnableMCP:   false,
	}

	container, err := di.BuildContainer(config)
	if err != nil {
		t.Fatalf("BuildContainer failed: %v", err)
	}
	defer func() { _ = container.Cleanup() }()

	// Start lifecycle
	if err := container.Start(); err != nil {
		t.Fatalf("Start failed: %v", err)
	}

	// Create server components
	broadcaster := app.NewEventBroadcaster()
	taskStore := app.NewInMemoryTaskStore()
	broadcaster.SetTaskStore(taskStore)

	serverCoordinator := app.NewServerCoordinator(
		container.AgentCoordinator,
		broadcaster,
		container.SessionStore,
		taskStore,
	)

	// Setup health checker
	healthChecker := app.NewHealthChecker()
	healthChecker.RegisterProbe(app.NewMCPProbe(container, false))
	healthChecker.RegisterProbe(app.NewLLMFactoryProbe(container))
	healthChecker.RegisterProbe(app.NewSandboxProbe(container.SandboxManager))

	// Create router
	router := NewRouter(serverCoordinator, broadcaster, healthChecker, "development")

	// Test health endpoint
	req := httptest.NewRequest("GET", "/health", nil)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	// Verify response
	if w.Code != 200 {
		t.Errorf("Expected status 200, got %d", w.Code)
	}

	contentType := w.Header().Get("Content-Type")
	if contentType != "application/json" {
		t.Errorf("Expected Content-Type application/json, got %s", contentType)
	}

	// Parse response
	var response struct {
		Status     string                  `json:"status"`
		Components []ports.ComponentHealth `json:"components"`
	}

	if err := json.Unmarshal(w.Body.Bytes(), &response); err != nil {
		t.Fatalf("Failed to parse response: %v", err)
	}

	// Verify overall status
	if response.Status != "healthy" {
		t.Errorf("Expected status 'healthy', got '%s'", response.Status)
	}

	// Verify components
	if len(response.Components) != 3 {
		t.Errorf("Expected 3 components, got %d", len(response.Components))
	}

	// Check component names
	componentNames := make(map[string]bool)
	for _, comp := range response.Components {
		componentNames[comp.Name] = true
	}

	expectedComponents := []string{"mcp", "llm_factory", "sandbox"}
	for _, name := range expectedComponents {
		if !componentNames[name] {
			t.Errorf("Expected component '%s' not found", name)
		}
	}

	// Verify disabled components report correctly
	for _, comp := range response.Components {
		if comp.Name == "mcp" {
			if comp.Status != ports.HealthStatusDisabled {
				t.Errorf("Expected mcp to be disabled, got %s", comp.Status)
			}
		}
		if comp.Name == "llm_factory" {
			if comp.Status != ports.HealthStatusReady {
				t.Errorf("Expected llm_factory to be ready, got %s", comp.Status)
			}
		}
		if comp.Name == "sandbox" {
			if comp.Status != ports.HealthStatusDisabled {
				t.Errorf("Expected sandbox to be disabled, got %s", comp.Status)
			}
		}
	}
}
