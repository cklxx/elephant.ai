package app

import (
	"testing"
	"time"

	"alex/internal/llm"
)

// Mock logger for testing
type testLogger struct {
	messages []string
}

func (l *testLogger) Debug(format string, args ...interface{}) {
	l.messages = append(l.messages, "DEBUG")
}
func (l *testLogger) Info(format string, args ...interface{}) {
	l.messages = append(l.messages, "INFO")
}
func (l *testLogger) Warn(format string, args ...interface{}) {
	l.messages = append(l.messages, "WARN")
}
func (l *testLogger) Error(format string, args ...interface{}) {
	l.messages = append(l.messages, "ERROR")
}

// Mock clock for testing
type testClock struct {
	fixedTime time.Time
}

func (c *testClock) Now() time.Time {
	return c.fixedTime
}

func TestWithLogger(t *testing.T) {
	logger := &testLogger{}
	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		WithLogger(logger),
	)

	// Verify logger is set
	if coordinator.logger != logger {
		t.Fatal("expected custom logger to be set")
	}

	// Trigger a log call to verify it works
	coordinator.logger.Info("test message")
	if len(logger.messages) != 1 || logger.messages[0] != "INFO" {
		t.Fatalf("expected logger to capture message, got %v", logger.messages)
	}
}

func TestWithClock(t *testing.T) {
	fixedTime := time.Date(2023, 1, 1, 12, 0, 0, 0, time.UTC)
	clock := &testClock{fixedTime: fixedTime}

	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		WithClock(clock),
	)

	// Verify clock is set
	if coordinator.clock != clock {
		t.Fatal("expected custom clock to be set")
	}

	// Verify clock works
	if got := coordinator.clock.Now(); !got.Equal(fixedTime) {
		t.Fatalf("expected clock to return %v, got %v", fixedTime, got)
	}
}

func TestWithCostTrackingDecorator(t *testing.T) {
	logger := &testLogger{}
	clock := &testClock{fixedTime: time.Now()}
	decorator := NewCostTrackingDecorator(nil, logger, clock)

	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		WithCostTrackingDecorator(decorator),
	)

	// Verify decorator is set
	if coordinator.costDecorator != decorator {
		t.Fatal("expected custom cost decorator to be set")
	}
}

func TestMultipleOptions(t *testing.T) {
	logger := &testLogger{}
	clock := &testClock{fixedTime: time.Now()}

	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		WithLogger(logger),
		WithClock(clock),
	)

	// Verify all options are applied
	if coordinator.logger != logger {
		t.Fatal("expected custom logger to be set")
	}
	if coordinator.clock != clock {
		t.Fatal("expected custom clock to be set")
	}
}

func TestOptionWithNilValue(t *testing.T) {
	// Create coordinator with default logger
	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
	)
	defaultLogger := coordinator.logger

	// Apply nil logger option - should not change the logger
	coordinator2 := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		WithLogger(nil),
	)

	// Verify logger is still set to default (not nil)
	if coordinator2.logger == nil {
		t.Fatal("expected logger to remain set when nil option is provided")
	}
	// Verify default logger is not nil
	if defaultLogger == nil {
		t.Fatal("expected default logger to be set")
	}
}

func TestOptionsBackwardCompatibility(t *testing.T) {
	// Test that coordinator works without any options (backward compatibility)
	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
	)

	// Verify all fields are initialized with defaults
	if coordinator.logger == nil {
		t.Fatal("expected default logger to be set")
	}
	if coordinator.clock == nil {
		t.Fatal("expected default clock to be set")
	}
	if coordinator.costDecorator == nil {
		t.Fatal("expected default cost decorator to be set")
	}
}

func TestOptionsAreAppliedBeforeServiceInitialization(t *testing.T) {
	logger := &testLogger{}

	// Create coordinator with custom logger
	coordinator := NewAgentCoordinator(
		llm.NewFactory(),
		stubToolRegistry{},
		&stubSessionStore{},
		stubContextManager{},
		nil,
		stubParser{},
		nil,
		Config{LLMProvider: "mock", LLMModel: "test", MaxIterations: 5},
		WithLogger(logger),
	)

	// Verify that the custom logger was used during initialization
	// This would show in the prepService which should use the custom logger
	if coordinator.prepService == nil {
		t.Fatal("expected prepService to be initialized")
	}

	// The prep service should have been initialized with the custom logger
	// We can't directly verify this without exposing internal fields,
	// but we can verify the coordinator logger is set correctly
	if coordinator.logger != logger {
		t.Fatal("expected coordinator to use custom logger")
	}
}
