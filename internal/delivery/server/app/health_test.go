package app

import (
	"context"
	"testing"

	"alex/internal/di"
	"alex/internal/server/ports"
)

func TestHealthChecker(t *testing.T) {
	t.Run("registers and checks probes", func(t *testing.T) {
		checker := NewHealthChecker()

		// Register a mock probe
		mockProbe := &mockHealthProbe{
			health: ports.ComponentHealth{
				Name:    "test_component",
				Status:  ports.HealthStatusReady,
				Message: "All good",
			},
		}
		checker.RegisterProbe(mockProbe)

		// Check all
		results := checker.CheckAll(context.Background())
		if len(results) != 1 {
			t.Errorf("Expected 1 result, got %d", len(results))
		}

		if results[0].Name != "test_component" {
			t.Errorf("Expected name 'test_component', got '%s'", results[0].Name)
		}

		if results[0].Status != ports.HealthStatusReady {
			t.Errorf("Expected status 'ready', got '%s'", results[0].Status)
		}
	})

	t.Run("handles multiple probes", func(t *testing.T) {
		checker := NewHealthChecker()

		probe1 := &mockHealthProbe{
			health: ports.ComponentHealth{Name: "component1", Status: ports.HealthStatusReady},
		}
		probe2 := &mockHealthProbe{
			health: ports.ComponentHealth{Name: "component2", Status: ports.HealthStatusDisabled},
		}

		checker.RegisterProbe(probe1)
		checker.RegisterProbe(probe2)

		results := checker.CheckAll(context.Background())
		if len(results) != 2 {
			t.Errorf("Expected 2 results, got %d", len(results))
		}
	})
}

func TestMCPProbe(t *testing.T) {
	t.Run("disabled state", func(t *testing.T) {
		config := di.Config{
			LLMProvider: "mock",
			LLMModel:    "test",
			EnableMCP:   false,
		}
		container, err := di.BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer failed: %v", err)
		}
		defer func() { _ = container.Shutdown() }()

		probe := NewMCPProbe(container, false)
		health := probe.Check(context.Background())

		if health.Name != "mcp" {
			t.Errorf("Expected name 'mcp', got '%s'", health.Name)
		}

		if health.Status != ports.HealthStatusDisabled {
			t.Errorf("Expected status 'disabled', got '%s'", health.Status)
		}
	})

	t.Run("not started", func(t *testing.T) {
		config := di.Config{
			LLMProvider: "mock",
			LLMModel:    "test",
			EnableMCP:   false, // Not started
		}
		container, err := di.BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer failed: %v", err)
		}
		defer func() { _ = container.Shutdown() }()

		probe := NewMCPProbe(container, true) // Probe thinks it's enabled
		health := probe.Check(context.Background())

		if health.Status != ports.HealthStatusNotReady {
			t.Errorf("Expected status 'not_ready', got '%s'", health.Status)
		}
	})
}

func TestLLMFactoryProbe(t *testing.T) {
	t.Run("ready when factory initialized", func(t *testing.T) {
		config := di.Config{
			LLMProvider: "mock",
			LLMModel:    "test",
		}
		container, err := di.BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer failed: %v", err)
		}
		defer func() { _ = container.Shutdown() }()

		probe := NewLLMFactoryProbe(container)
		health := probe.Check(context.Background())

		if health.Name != "llm_factory" {
			t.Errorf("Expected name 'llm_factory', got '%s'", health.Name)
		}

		if health.Status != ports.HealthStatusReady {
			t.Errorf("Expected status 'ready', got '%s'", health.Status)
		}
	})

	t.Run("not ready when factory missing", func(t *testing.T) {
		// A bare Container without BuildContainer has no llmFactory.
		container := &di.Container{}

		probe := NewLLMFactoryProbe(container)
		health := probe.Check(context.Background())

		if health.Status != ports.HealthStatusNotReady {
			t.Errorf("Expected status 'not_ready', got '%s'", health.Status)
		}
		if health.Message != "LLM factory not initialized" {
			t.Errorf("Unexpected message: %s", health.Message)
		}
	})
}

func TestDegradedProbe(t *testing.T) {
	t.Run("ready when no degraded components", func(t *testing.T) {
		source := &mockDegradedSource{components: nil}
		probe := NewDegradedProbe(source)
		health := probe.Check(context.Background())

		if health.Name != "bootstrap" {
			t.Errorf("Expected name 'bootstrap', got '%s'", health.Name)
		}
		if health.Status != ports.HealthStatusReady {
			t.Errorf("Expected status 'ready', got '%s'", health.Status)
		}
	})

	t.Run("not ready when components degraded", func(t *testing.T) {
		source := &mockDegradedSource{
			components: map[string]string{
				"event-history": "connection refused",
				"analytics":     "invalid API key",
			},
		}
		probe := NewDegradedProbe(source)
		health := probe.Check(context.Background())

		if health.Status != ports.HealthStatusNotReady {
			t.Errorf("Expected status 'not_ready', got '%s'", health.Status)
		}
		details, ok := health.Details.(map[string]string)
		if !ok {
			t.Fatalf("Expected details map[string]string, got %T", health.Details)
		}
		if details["event-history"] != "connection refused" {
			t.Errorf("Expected event-history detail, got %v", details)
		}
		if details["analytics"] != "invalid API key" {
			t.Errorf("Expected analytics detail, got %v", details)
		}
	})

	t.Run("ready when source is nil", func(t *testing.T) {
		probe := NewDegradedProbe(nil)
		health := probe.Check(context.Background())
		if health.Status != ports.HealthStatusReady {
			t.Errorf("Expected 'ready' for nil source, got '%s'", health.Status)
		}
	})
}

// --- test doubles ---

// Mock probe for testing
type mockHealthProbe struct {
	health ports.ComponentHealth
}

func (m *mockHealthProbe) Check(ctx context.Context) ports.ComponentHealth {
	return m.health
}

type mockDegradedSource struct {
	components map[string]string
}

func (m *mockDegradedSource) Map() map[string]string {
	return m.components
}

func (m *mockDegradedSource) IsEmpty() bool {
	return len(m.components) == 0
}
