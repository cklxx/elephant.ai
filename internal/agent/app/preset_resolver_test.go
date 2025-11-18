package app

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/presets"
)

func TestPresetResolver_ResolveToolRegistry_DefaultBehavior(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := stubToolRegistry{}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "")
	if registry != baseRegistry {
		t.Fatal("expected base registry when no preset specified")
	}
}

func TestPresetResolver_ResolveToolRegistry_WithConfigPreset(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}},
	}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "read-only")
	if registry == baseRegistry {
		t.Fatal("expected filtered registry, got base registry")
	}

	for _, tool := range registry.List() {
		if tool.Name == "bash" {
			t.Fatal("expected bash to be filtered out in read-only preset")
		}
	}
}

func TestPresetResolver_ResolveToolRegistry_WithContextPreset(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "web_search"}},
	}

	ctx := context.WithValue(context.Background(), PresetContextKey{}, PresetConfig{ToolPreset: "web-only"})

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
		t.Fatal("expected web_search in web-only preset")
	}
	if hasFileRead {
		t.Fatal("expected file_read to be filtered out in web-only preset")
	}
}

func TestPresetResolver_ResolveToolRegistry_InvalidPreset(t *testing.T) {
	logger := &testLogger{}
	resolver := NewPresetResolver(logger)
	baseRegistry := stubToolRegistry{}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "invalid-preset")
	if registry != baseRegistry {
		t.Fatal("expected base registry when preset is invalid")
	}
}

func TestPresetResolver_AllValidToolPresets(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}, {Name: "web_search"}, {Name: "think"}},
	}

	validPresets := []string{"full", "read-only", "code-only", "web-only", "safe"}
	for _, preset := range validPresets {
		t.Run(preset, func(t *testing.T) {
			registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, preset)
			if registry == nil {
				t.Fatalf("expected non-nil registry for preset %s", preset)
			}
			tools := registry.List()
			if preset != "full" && len(tools) >= len(baseRegistry.tools) {
				t.Fatalf("expected filtered tools for preset %s", preset)
			}
		})
	}
}

func TestPresetResolver_ContextPriorityOverConfigForTools(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	ctx := context.WithValue(context.Background(), PresetContextKey{}, PresetConfig{ToolPreset: "web-only"})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "web_search"}},
	}

	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, "read-only")
	tools := registry.List()
	hasWeb := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			hasWeb = true
		}
		if tool.Name == "file_read" {
			t.Fatal("expected file_read filtered out in context preset")
		}
	}
	if !hasWeb {
		t.Fatal("expected web_search retained via context preset")
	}
}

func TestPresetResolver_EmitsToolFilteringEvent(t *testing.T) {
	logger := &testLogger{}
	mockClk := newMockClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eventCapturer := &eventCapturer{}

	resolver := NewPresetResolverWithDeps(PresetResolverDeps{
		Logger:       logger,
		Clock:        mockClk,
		EventEmitter: eventCapturer,
	})

	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}, {Name: "web_search"}, {Name: "think"}},
	}

	ctx := context.WithValue(context.Background(), ports.SessionContextKey{}, "test-session-123")
	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, "read-only")

	if len(eventCapturer.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventCapturer.events))
	}
	event := eventCapturer.events[0]
	filterEvent, ok := event.(*domain.ToolFilteringEvent)
	if !ok {
		t.Fatalf("expected ToolFilteringEvent, got %T", event)
	}
	if filterEvent.GetSessionID() != "test-session-123" {
		t.Errorf("expected sessionID 'test-session-123', got '%s'", filterEvent.GetSessionID())
	}
	if filterEvent.PresetName != "Read-Only Access" {
		t.Errorf("expected preset name 'Read-Only Access', got '%s'", filterEvent.PresetName)
	}
	if registry == baseRegistry {
		t.Error("expected filtered registry, got base registry")
	}
}

func TestPresetResolver_NoEventWhenNoPreset(t *testing.T) {
	eventCapturer := &eventCapturer{}
	resolver := NewPresetResolverWithDeps(PresetResolverDeps{
		Logger:       &testLogger{},
		EventEmitter: eventCapturer,
	})

	baseRegistry := &mockToolRegistry{tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "bash"}}}
	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, "")
	if len(eventCapturer.events) != 0 {
		t.Errorf("expected no events, got %d", len(eventCapturer.events))
	}
	if registry != baseRegistry {
		t.Error("expected base registry when no preset specified")
	}
}

// Mock tool registry and helpers

type mockToolRegistry struct {
	tools []ports.ToolDefinition
}

func (r *mockToolRegistry) Register(tool ports.ToolExecutor) error { return nil }
func (r *mockToolRegistry) Get(name string) (ports.ToolExecutor, error) {
	for _, t := range r.tools {
		if t.Name == name {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}
func (r *mockToolRegistry) List() []ports.ToolDefinition { return r.tools }
func (r *mockToolRegistry) Unregister(name string) error { return nil }
func (r *mockToolRegistry) WithoutSubagent() ports.ToolRegistry {
	filtered := make([]ports.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		if t.Name != "subagent" {
			filtered = append(filtered, t)
		}
	}
	return &mockToolRegistry{tools: filtered}
}

// Event capturer for testing

type eventCapturer struct {
	events []ports.AgentEvent
}

func (e *eventCapturer) OnEvent(event ports.AgentEvent) {
	e.events = append(e.events, event)
}

func TestToolFilteringEventImplementation(t *testing.T) {
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

	if event.EventType() != "tool_filtering" {
		t.Errorf("Expected event type 'tool_filtering', got '%s'", event.EventType())
	}
	if event.GetSessionID() != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", event.GetSessionID())
	}
	if event.GetTaskID() != "task-1" {
		t.Errorf("Expected task ID 'task-1', got '%s'", event.GetTaskID())
	}
	if event.GetParentTaskID() != "parent-task" {
		t.Errorf("Expected parent task ID 'parent-task', got '%s'", event.GetParentTaskID())
	}
	if event.GetAgentLevel() != ports.LevelCore {
		t.Errorf("Expected agent level LevelCore, got %v", event.GetAgentLevel())
	}
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

var _ ports.ToolRegistry = (*presets.FilteredToolRegistry)(nil)
