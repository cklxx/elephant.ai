package app

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"alex/internal/app/di"
	"alex/internal/delivery/server/ports"
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

	t.Run("not ready when container nil", func(t *testing.T) {
		probe := NewLLMFactoryProbe(nil)
		health := probe.Check(context.Background())

		if health.Status != ports.HealthStatusNotReady {
			t.Errorf("Expected status 'not_ready', got '%s'", health.Status)
		}
		if health.Message != "LLM factory container not initialized" {
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

func TestLLMModelHealthProbe_PublicEndpointShowsAggregateOnly(t *testing.T) {
	provider := &mockModelHealthProvider{
		healthy: false,
		message: "1 models tracked, avg health score 44",
		details: []map[string]interface{}{
			{
				"model":        "gpt-4",
				"state":        "degraded",
				"error_class":  "transient",
				"error_rate":   0.15,
				"health_score": 44.0,
			},
		},
	}

	probe := NewLLMModelHealthProbe(provider)
	health := probe.Check(context.Background())
	if health.Name != "llm_models" {
		t.Fatalf("expected name llm_models, got %s", health.Name)
	}

	// Public endpoint must NOT expose per-model details.
	if health.Details != nil {
		t.Fatalf("expected nil Details on public health check, got %v", health.Details)
	}

	// Serialize to JSON — this is what the public /health handler sends.
	data, err := json.Marshal(health)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	output := string(data)

	// Must NOT contain any model-level telemetry.
	for _, needle := range []string{
		"gpt-4",
		"error_rate",
		"health_score",
		"latency",
		"failure_count",
	} {
		if strings.Contains(output, needle) {
			t.Errorf("public health output contains %q:\n%s", needle, output)
		}
	}

	// Message should contain aggregate info.
	if !strings.Contains(health.Message, "1 models tracked") {
		t.Errorf("expected aggregate message, got %q", health.Message)
	}

	// Degraded model → status should reflect degradation.
	if health.Status != ports.HealthStatusNotReady {
		t.Errorf("expected not_ready for degraded model, got %s", health.Status)
	}
}

func TestLLMModelHealthProbe_DebugEndpointShowsPerModelDetails(t *testing.T) {
	provider := &mockModelHealthProvider{
		healthy: false,
		message: "1 models tracked, avg health score 44",
		details: []map[string]interface{}{
			{
				"model":        "gpt-4",
				"state":        "degraded",
				"error_class":  "transient",
				"error_rate":   0.15,
				"health_score": 44.0,
			},
		},
	}

	probe := NewLLMModelHealthProbe(provider)
	details := probe.DetailedHealth()
	if details == nil {
		t.Fatal("expected non-nil detailed health")
	}

	data, err := json.Marshal(details)
	if err != nil {
		t.Fatalf("json marshal: %v", err)
	}
	output := string(data)

	// Debug endpoint SHOULD contain per-model telemetry.
	for _, needle := range []string{
		"gpt-4",
		"degraded",
		"transient",
		"error_rate",
		"health_score",
	} {
		if !strings.Contains(output, needle) {
			t.Errorf("debug output missing expected field %q:\n%s", needle, output)
		}
	}
}

func TestLLMModelHealthProbe_NilProvider(t *testing.T) {
	probe := NewLLMModelHealthProbe(nil)
	health := probe.Check(context.Background())
	if health.Status != ports.HealthStatusDisabled {
		t.Errorf("expected disabled, got %s", health.Status)
	}
}

func TestLLMModelHealthProbe_NoModels(t *testing.T) {
	provider := &mockModelHealthProvider{
		healthy: true,
		message: "No models tracked yet",
		details: nil,
	}
	probe := NewLLMModelHealthProbe(provider)
	health := probe.Check(context.Background())
	if health.Status != ports.HealthStatusReady {
		t.Errorf("expected ready, got %s", health.Status)
	}
	if health.Message != "No models tracked yet" {
		t.Errorf("expected 'No models tracked yet' message, got %q", health.Message)
	}
}

func TestModelHealthDetails_ReturnsNilForNilProvider(t *testing.T) {
	probe := NewLLMModelHealthProbe(nil)
	details := probe.DetailedHealth()
	if details != nil {
		t.Errorf("expected nil details for nil provider, got %v", details)
	}
}

func TestHealthCheckerImpl_ModelHealthDetails(t *testing.T) {
	checker := NewHealthChecker()
	provider := &mockModelHealthProvider{
		healthy: true,
		message: "1 models tracked, avg health score 100",
		details: []map[string]interface{}{
			{"model": "gpt-4", "state": "healthy", "health_score": 100.0},
		},
	}
	probe := NewLLMModelHealthProbe(provider)
	checker.RegisterProbe(probe)

	details := checker.ModelHealthDetails()
	if details == nil {
		t.Fatal("expected non-nil model health details")
	}

	sanitized, ok := details.([]map[string]interface{})
	if !ok {
		t.Fatalf("expected []map[string]interface{}, got %T", details)
	}
	if len(sanitized) != 1 {
		t.Fatalf("expected 1 model, got %d", len(sanitized))
	}
	if sanitized[0]["model"] != "gpt-4" {
		t.Errorf("expected gpt-4, got %v", sanitized[0]["model"])
	}
}

func TestHealthCheckerImpl_ModelHealthDetails_NoProbe(t *testing.T) {
	checker := NewHealthChecker()
	// Register a non-model probe.
	checker.RegisterProbe(&mockHealthProbe{
		health: ports.ComponentHealth{Name: "test", Status: ports.HealthStatusReady},
	})
	details := checker.ModelHealthDetails()
	if details != nil {
		t.Errorf("expected nil when no model health probe, got %v", details)
	}
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

// mockModelHealthProvider implements ports.ModelHealthProvider for testing.
type mockModelHealthProvider struct {
	healthy bool
	message string
	details interface{}
}

func (m *mockModelHealthProvider) AggregateModelHealth() (bool, string) {
	return m.healthy, m.message
}

func (m *mockModelHealthProvider) SanitizedModelHealth() interface{} {
	return m.details
}
