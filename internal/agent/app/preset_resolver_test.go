package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
	"alex/internal/prompts"
)

func TestPresetResolver_ResolveSystemPrompt_DefaultBehavior(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})

	prompt := resolver.ResolveSystemPrompt(context.Background(), "test task", nil, "")

	// Should return default prompt from loader
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	if prompt == "You are ALEX, a helpful AI coding assistant. Use available tools to help solve the user's task." {
		t.Fatal("expected default prompt from loader, not fallback")
	}
}

func TestPresetResolver_ResolveSystemPrompt_WithConfigPreset(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})

	prompt := resolver.ResolveSystemPrompt(context.Background(), "test task", nil, "code-expert")

	// Should use code-expert preset
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
	// Check for code-expert specific content
	if len(prompt) < 100 {
		t.Fatal("expected detailed code-expert prompt")
	}
	// Code expert prompt should mention "Code Expert"
	if !containsText(prompt, "Code Expert") && !containsText(prompt, "code quality") {
		t.Fatalf("expected code-expert preset content, got: %s", prompt[:100])
	}
}

func TestPresetResolver_ResolveSystemPrompt_WithContextPreset(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})

	ctx := context.WithValue(context.Background(), PresetContextKey{}, PresetConfig{
		AgentPreset: "researcher",
	})

	prompt := resolver.ResolveSystemPrompt(ctx, "test task", nil, "code-expert")

	// Context preset should override config preset
	if !containsText(prompt, "Research") && !containsText(prompt, "information gathering") {
		t.Fatalf("expected researcher preset to override config, got: %s", prompt[:100])
	}
}

func TestPresetResolver_ResolveSystemPrompt_WithAnalysis(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})

	analysis := &ports.TaskAnalysisInfo{
		Action:   "Analyzing codebase",
		Goal:     "Find performance issues",
		Approach: "Use profiling tools",
	}

	prompt := resolver.ResolveSystemPrompt(context.Background(), "test task", analysis, "")

	// Should include analysis info (when using default prompt loader)
	if prompt == "" {
		t.Fatal("expected non-empty prompt")
	}
}

func TestPresetResolver_ResolveSystemPrompt_InvalidPreset(t *testing.T) {
	logger := &testLogger{}
	resolver := NewPresetResolver(prompts.New(), logger)

	prompt := resolver.ResolveSystemPrompt(context.Background(), "test task", nil, "invalid-preset")

	// Should fall back to default prompt
	if prompt == "" {
		t.Fatal("expected non-empty fallback prompt")
	}
	// Should have logged a warning
	if len(logger.messages) == 0 {
		t.Fatal("expected warning to be logged for invalid preset")
	}
}

func TestPresetResolver_ResolveToolRegistry_DefaultBehavior(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})
	baseRegistry := stubToolRegistry{}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "")

	// Should return base registry unchanged
	if registry != baseRegistry {
		t.Fatal("expected base registry when no preset specified")
	}
}

func TestPresetResolver_ResolveToolRegistry_WithConfigPreset(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "file_read"},
			{Name: "file_write"},
			{Name: "bash"},
		},
	}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "read-only")

	// Should return filtered registry
	if registry == baseRegistry {
		t.Fatal("expected filtered registry, got base registry")
	}

	// Verify filtering works
	tools := registry.List()
	for _, tool := range tools {
		if tool.Name == "bash" {
			t.Fatal("expected bash to be filtered out in read-only preset")
		}
	}
}

func TestPresetResolver_ResolveToolRegistry_WithContextPreset(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "file_read"},
			{Name: "file_write"},
			{Name: "web_search"},
		},
	}

	ctx := context.WithValue(context.Background(), PresetContextKey{}, PresetConfig{
		ToolPreset: "web-only",
	})

	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, "read-only")

	// Context preset should override config preset
	tools := registry.List()
	hasWebSearch := false
	hasFileRead := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			hasWebSearch = true
		}
		if tool.Name == "file_read" {
			hasFileRead = true
		}
	}

	if !hasWebSearch {
		t.Fatal("expected web_search in web-only preset")
	}
	if hasFileRead {
		t.Fatal("expected file_read to be filtered out in web-only preset")
	}
}

func TestPresetResolver_ResolveToolRegistry_InvalidPreset(t *testing.T) {
	logger := &testLogger{}
	resolver := NewPresetResolver(prompts.New(), logger)
	baseRegistry := stubToolRegistry{}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "invalid-preset")

	// Should return base registry on invalid preset
	if registry != baseRegistry {
		t.Fatal("expected base registry when preset is invalid")
	}
}

func TestPresetResolver_AllValidAgentPresets(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})

	validPresets := []string{"default", "code-expert", "researcher", "devops", "security-analyst", "designer"}

	for _, preset := range validPresets {
		t.Run(preset, func(t *testing.T) {
			prompt := resolver.ResolveSystemPrompt(context.Background(), "test", nil, preset)
			if prompt == "" {
				t.Fatalf("expected non-empty prompt for preset %s", preset)
			}
			if len(prompt) < 50 {
				t.Fatalf("expected detailed prompt for preset %s, got %d chars", preset, len(prompt))
			}
		})
	}
}

func TestPresetResolver_AllValidToolPresets(t *testing.T) {
	resolver := NewPresetResolver(prompts.New(), &testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "file_read"},
			{Name: "file_write"},
			{Name: "bash"},
			{Name: "web_search"},
			{Name: "think"},
		},
	}

	validPresets := []string{"full", "read-only", "code-only", "web-only", "safe"}

	for _, preset := range validPresets {
		t.Run(preset, func(t *testing.T) {
			registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, preset)
			if registry == nil {
				t.Fatalf("expected non-nil registry for preset %s", preset)
			}

			// Verify we get some tools back
			tools := registry.List()
			if preset != "full" && len(tools) >= len(baseRegistry.tools) {
				t.Fatalf("expected filtered tools for preset %s", preset)
			}
		})
	}
}

func TestPresetResolver_NilDependencies(t *testing.T) {
	// Should create resolver with defaults when optional dependencies are nil
	// PromptLoader is required, but logger can be nil
	resolver := NewPresetResolver(prompts.New(), nil)

	if resolver == nil {
		t.Fatal("expected resolver to be created")
	}

	// Should work with default logger (nil becomes NoopLogger)
	prompt := resolver.ResolveSystemPrompt(context.Background(), "test", nil, "")
	if prompt == "" {
		t.Fatal("expected resolver to work with default logger")
	}
}

func TestPresetResolver_ContextPriorityOverConfig(t *testing.T) {
	logger := &testLogger{}
	resolver := NewPresetResolver(prompts.New(), logger)

	// Set different presets in context vs config
	ctx := context.WithValue(context.Background(), PresetContextKey{}, PresetConfig{
		AgentPreset: "researcher",
		ToolPreset:  "web-only",
	})

	// Test agent preset priority
	prompt := resolver.ResolveSystemPrompt(ctx, "test", nil, "code-expert")
	if !containsText(prompt, "Research") && !containsText(prompt, "information gathering") {
		t.Fatal("expected context preset (researcher) to override config preset (code-expert)")
	}

	// Test tool preset priority
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "file_read"},
			{Name: "web_search"},
		},
	}
	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, "read-only")
	tools := registry.List()

	hasWebSearch := false
	hasFileRead := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			hasWebSearch = true
		}
		if tool.Name == "file_read" {
			hasFileRead = true
		}
	}

	if !hasWebSearch {
		t.Fatal("expected web_search in web-only preset (context)")
	}
	if hasFileRead {
		t.Fatal("expected file_read filtered out in web-only preset (context)")
	}
}

// Helper functions

func containsText(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && findSubstring(s, substr))
}

func findSubstring(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

// Mock tool registry for testing
type mockToolRegistry struct {
	tools []ports.ToolDefinition
}

func (r *mockToolRegistry) Register(tool ports.ToolExecutor) error {
	return nil
}

func (r *mockToolRegistry) Get(name string) (ports.ToolExecutor, error) {
	for _, t := range r.tools {
		if t.Name == name {
			return nil, nil // Return nil executor for testing
		}
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}

func (r *mockToolRegistry) List() []ports.ToolDefinition {
	return r.tools
}

func (r *mockToolRegistry) Unregister(name string) error {
	return nil
}

func (r *mockToolRegistry) WithoutSubagent() ports.ToolRegistry {
	filtered := make([]ports.ToolDefinition, 0)
	for _, t := range r.tools {
		if t.Name != "subagent" {
			filtered = append(filtered, t)
		}
	}
	return &mockToolRegistry{tools: filtered}
}

func TestPresetResolver_EmitsToolFilteringEvent(t *testing.T) {
	logger := &testLogger{}
	mockClk := newMockClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eventCapturer := &eventCapturer{}

	resolver := NewPresetResolverWithDeps(PresetResolverDeps{
		PromptLoader: prompts.New(),
		Logger:       logger,
		Clock:        mockClk,
		EventEmitter: eventCapturer,
	})

	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "file_read"},
			{Name: "file_write"},
			{Name: "bash"},
			{Name: "web_search"},
			{Name: "think"},
		},
	}

	// Add session ID to context
	ctx := context.WithValue(context.Background(), ports.SessionContextKey{}, "test-session-123")

	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, "read-only")

	// Verify event was emitted
	if len(eventCapturer.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventCapturer.events))
	}

	event := eventCapturer.events[0]
	filterEvent, ok := event.(*domain.ToolFilteringEvent)
	if !ok {
		t.Fatalf("expected ToolFilteringEvent, got %T", event)
	}

	// Verify event fields
	if filterEvent.GetSessionID() != "test-session-123" {
		t.Errorf("expected sessionID 'test-session-123', got '%s'", filterEvent.GetSessionID())
	}

	if filterEvent.PresetName != "Read-Only Access" {
		t.Errorf("expected preset name 'Read-Only Access', got '%s'", filterEvent.PresetName)
	}

	if filterEvent.OriginalCount != 5 {
		t.Errorf("expected original count 5, got %d", filterEvent.OriginalCount)
	}

	if filterEvent.FilteredCount <= 0 || filterEvent.FilteredCount >= 5 {
		t.Errorf("expected filtered count between 1-4, got %d", filterEvent.FilteredCount)
	}

	// Verify ratio is calculated correctly
	expectedRatio := float64(filterEvent.FilteredCount) / float64(filterEvent.OriginalCount) * 100.0
	if filterEvent.ToolFilterRatio != expectedRatio {
		t.Errorf("expected ratio %.2f%%, got %.2f%%", expectedRatio, filterEvent.ToolFilterRatio)
	}

	// Verify filtered tools list is populated
	if len(filterEvent.FilteredTools) != filterEvent.FilteredCount {
		t.Errorf("expected %d tool names, got %d", filterEvent.FilteredCount, len(filterEvent.FilteredTools))
	}

	// Verify registry is actually filtered
	if registry == baseRegistry {
		t.Error("expected filtered registry, got base registry")
	}
}

func TestPresetResolver_NoEventWhenNoPreset(t *testing.T) {
	eventCapturer := &eventCapturer{}
	resolver := NewPresetResolverWithDeps(PresetResolverDeps{
		PromptLoader: prompts.New(),
		Logger:       &testLogger{},
		EventEmitter: eventCapturer,
	})

	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{
			{Name: "file_read"},
			{Name: "bash"},
		},
	}

	// No preset specified
	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "")

	// Should not emit event when no filtering occurs
	if len(eventCapturer.events) != 0 {
		t.Errorf("expected no events, got %d", len(eventCapturer.events))
	}

	// Should return base registry unchanged
	if registry != baseRegistry {
		t.Error("expected base registry when no preset specified")
	}
}

// Event capturer for testing
type eventCapturer struct {
	events []ports.AgentEvent
}

func (e *eventCapturer) OnEvent(event ports.AgentEvent) {
	e.events = append(e.events, event)
}

func TestToolFilteringEventImplementation(t *testing.T) {
	// Create a ToolFilteringEvent
	event := domain.NewToolFilteringEvent(
		ports.LevelCore,
		"test-session",
		"task-1",
		"parent-task",
		"Read-Only Access",
		10,
		5,
		[]string{"file_read", "grep", "find", "ripgrep", "list_files"},
		time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	)

	// Verify event type
	if event.EventType() != "tool_filtering" {
		t.Errorf("Expected event type 'tool_filtering', got '%s'", event.EventType())
	}

	// Verify session ID
	if event.GetSessionID() != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", event.GetSessionID())
	}

	if event.GetTaskID() != "task-1" {
		t.Errorf("Expected task ID 'task-1', got '%s'", event.GetTaskID())
	}

	if event.GetParentTaskID() != "parent-task" {
		t.Errorf("Expected parent task ID 'parent-task', got '%s'", event.GetParentTaskID())
	}

	// Verify agent level
	if event.GetAgentLevel() != ports.LevelCore {
		t.Errorf("Expected agent level LevelCore, got %v", event.GetAgentLevel())
	}

	// Verify fields
	if event.PresetName != "Read-Only Access" {
		t.Errorf("Expected preset name 'Read-Only Access', got '%s'", event.PresetName)
	}

	if event.OriginalCount != 10 {
		t.Errorf("Expected original count 10, got %d", event.OriginalCount)
	}

	if event.FilteredCount != 5 {
		t.Errorf("Expected filtered count 5, got %d", event.FilteredCount)
	}

	if event.ToolFilterRatio != 50.0 {
		t.Errorf("Expected ratio 50%%, got %.2f%%", event.ToolFilterRatio)
	}

	if len(event.FilteredTools) != 5 {
		t.Errorf("Expected 5 filtered tools, got %d", len(event.FilteredTools))
	}
}

// Verify presets.FilteredToolRegistry satisfies ToolRegistry interface
var _ ports.ToolRegistry = (*presets.FilteredToolRegistry)(nil)
