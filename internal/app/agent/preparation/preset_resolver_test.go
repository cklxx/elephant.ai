package preparation

import (
	"context"
	"fmt"
	"testing"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/shared/agent/presets"
	id "alex/internal/shared/utils/id"
)

func TestPresetResolver_ResolveToolRegistry_DefaultBehavior(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "bash"}},
	}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, presets.ToolModeCLI, "")
	if registry == nil {
		t.Fatal("expected registry when defaulting tool preset")
	}

	tools := registry.List()
	if len(tools) != len(baseRegistry.tools) {
		t.Fatalf("expected default preset to retain all tools, got %d", len(tools))
	}
}

func TestPresetResolver_ResolveToolRegistry_WithConfigPreset(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}},
	}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, presets.ToolModeCLI, "read-only")
	if registry == baseRegistry {
		t.Fatal("expected filtered registry, got base registry")
	}

	tools := registry.List()
	if len(tools) != len(baseRegistry.tools) {
		t.Fatalf("expected all tools to remain available, got %d", len(tools))
	}
}

func TestPresetResolver_ResolveToolRegistry_WithContextPreset(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}, {Name: "web_search"}},
	}

	ctx := context.WithValue(context.Background(), appcontext.PresetContextKey{}, appcontext.PresetConfig{ToolPreset: "read-only"})

	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, presets.ToolModeCLI, "safe")
	tools := registry.List()
	hasWebSearch := false
	hasFileRead := false
	hasFileWrite := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			hasWebSearch = true
		}
		if tool.Name == "file_read" {
			hasFileRead = true
		}
		if tool.Name == "file_write" {
			hasFileWrite = true
		}
	}
	if !hasWebSearch {
		t.Fatal("expected web_search in read-only preset")
	}
	if !hasFileRead {
		t.Fatal("expected file_read to be retained in read-only preset")
	}
	if !hasFileWrite {
		t.Fatal("expected file_write to remain available in read-only preset")
	}
}

func TestPresetResolver_ResolveToolRegistry_InvalidPreset(t *testing.T) {
	logger := &testLogger{}
	resolver := NewPresetResolver(logger)
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "bash"}},
	}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, presets.ToolModeCLI, "invalid-preset")
	if registry == nil {
		t.Fatal("expected registry when preset is invalid")
	}
	tools := registry.List()
	if len(tools) != len(baseRegistry.tools) {
		t.Fatalf("expected fallback to full preset, got %d tools", len(tools))
	}
}

func TestPresetResolver_AllValidToolPresets(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}, {Name: "web_search"}},
	}

	validPresets := []string{"full", "read-only", "safe", "architect"}
	for _, preset := range validPresets {
		t.Run(preset, func(t *testing.T) {
			registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, presets.ToolModeCLI, preset)
			if registry == nil {
				t.Fatalf("expected non-nil registry for preset %s", preset)
			}
			tools := registry.List()
			if len(tools) != len(baseRegistry.tools) {
				t.Fatalf("expected all tools retained for preset %s, got %d", preset, len(tools))
			}
		})
	}
}

func TestPresetResolver_WebModeUsesPresetWhenProvided(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "web_search"}, {Name: "plan"}, {Name: "bash"}},
	}

	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, presets.ToolModeWeb, "architect")
	tools := registry.List()
	if len(tools) != len(baseRegistry.tools) {
		t.Fatalf("expected web architect to retain all tools, got %d", len(tools))
	}
}

func TestPresetResolver_ContextPriorityOverConfigForTools(t *testing.T) {
	resolver := NewPresetResolver(&testLogger{})
	ctx := context.WithValue(context.Background(), appcontext.PresetContextKey{}, appcontext.PresetConfig{ToolPreset: "safe"})
	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}, {Name: "web_search"}},
	}

	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, presets.ToolModeCLI, "read-only")
	tools := registry.List()
	hasWeb := false
	hasFileWrite := false
	for _, tool := range tools {
		if tool.Name == "web_search" {
			hasWeb = true
		}
		if tool.Name == "file_write" {
			hasFileWrite = true
		}
	}
	if !hasWeb {
		t.Fatal("expected web_search retained via context preset")
	}
	if !hasFileWrite {
		t.Fatal("expected file_write retained via context preset")
	}
	hasBash := false
	for _, tool := range tools {
		if tool.Name == "bash" {
			hasBash = true
			break
		}
	}
	if !hasBash {
		t.Fatal("expected bash retained via context preset")
	}
}

func TestPresetResolver_EmitsWorkflowDiagnosticToolFilteringEvent(t *testing.T) {
	logger := &testLogger{}
	mockClk := newMockClock(time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC))
	eventCapturer := &eventCapturer{}

	resolver := NewPresetResolverWithDeps(PresetResolverDeps{
		Logger:       logger,
		Clock:        mockClk,
		EventEmitter: eventCapturer,
	})

	baseRegistry := &mockToolRegistry{
		tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "file_write"}, {Name: "bash"}, {Name: "web_search"}},
	}

	ctx := context.WithValue(context.Background(), id.SessionContextKey{}, "test-session-123")
	registry := resolver.ResolveToolRegistry(ctx, baseRegistry, presets.ToolModeCLI, "read-only")

	if len(eventCapturer.events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(eventCapturer.events))
	}
	event := eventCapturer.events[0]
	filterEvent, ok := event.(*domain.WorkflowDiagnosticToolFilteringEvent)
	if !ok {
		t.Fatalf("expected WorkflowDiagnosticToolFilteringEvent, got %T", event)
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

func TestPresetResolver_DefaultPresetStillEmitsEvent(t *testing.T) {
	eventCapturer := &eventCapturer{}
	resolver := NewPresetResolverWithDeps(PresetResolverDeps{
		Logger:       &testLogger{},
		EventEmitter: eventCapturer,
	})

	baseRegistry := &mockToolRegistry{tools: []ports.ToolDefinition{{Name: "file_read"}, {Name: "bash"}}}
	registry := resolver.ResolveToolRegistry(context.Background(), baseRegistry, presets.ToolModeCLI, "")
	if len(eventCapturer.events) != 1 {
		t.Fatalf("expected an event for default preset application, got %d", len(eventCapturer.events))
	}
	filterEvent, ok := eventCapturer.events[0].(*domain.WorkflowDiagnosticToolFilteringEvent)
	if !ok {
		t.Fatalf("expected WorkflowDiagnosticToolFilteringEvent, got %T", eventCapturer.events[0])
	}
	if filterEvent.PresetName != "Full Access" {
		t.Fatalf("expected Full Access preset name, got %s", filterEvent.PresetName)
	}
	if registry == nil {
		t.Fatal("expected registry when resolving default preset")
	}
}

// Mock tool registry and helpers

type mockToolRegistry struct {
	tools []ports.ToolDefinition
}

func (r *mockToolRegistry) Register(tool tools.ToolExecutor) error { return nil }
func (r *mockToolRegistry) Get(name string) (tools.ToolExecutor, error) {
	for _, t := range r.tools {
		if t.Name == name {
			return nil, nil
		}
	}
	return nil, fmt.Errorf("tool not found: %s", name)
}
func (r *mockToolRegistry) List() []ports.ToolDefinition { return r.tools }
func (r *mockToolRegistry) Unregister(name string) error { return nil }
func (r *mockToolRegistry) WithoutSubagent() tools.ToolRegistry {
	filtered := make([]ports.ToolDefinition, 0, len(r.tools))
	for _, t := range r.tools {
		if t.Name != "subagent" && t.Name != "acp_executor" && t.Name != "explore" {
			filtered = append(filtered, t)
		}
	}
	return &mockToolRegistry{tools: filtered}
}

// Event capturer for testing

type eventCapturer struct {
	events []agent.AgentEvent
}

func (e *eventCapturer) OnEvent(event agent.AgentEvent) {
	e.events = append(e.events, event)
}

func TestWorkflowDiagnosticToolFilteringEventImplementation(t *testing.T) {
	event := domain.NewWorkflowDiagnosticToolFilteringEvent(
		agent.LevelCore,
		"test-session",
		"task-1",
		"parent-task",
		"Read-Only Access",
		10,
		5,
		[]string{"file_read", "grep", "find", "ripgrep", "list_files"},
		time.Date(2025, 1, 15, 10, 0, 0, 0, time.UTC),
	)

	if event.EventType() != "workflow.diagnostic.tool_filtering" {
		t.Errorf("Expected event type 'workflow.diagnostic.tool_filtering', got '%s'", event.EventType())
	}
	if event.GetSessionID() != "test-session" {
		t.Errorf("Expected session ID 'test-session', got '%s'", event.GetSessionID())
	}
	if event.GetRunID() != "task-1" {
		t.Errorf("Expected run ID 'task-1', got '%s'", event.GetRunID())
	}
	if event.GetParentRunID() != "parent-task" {
		t.Errorf("Expected parent run ID 'parent-task', got '%s'", event.GetParentRunID())
	}
	if event.GetAgentLevel() != agent.LevelCore {
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

var _ tools.ToolRegistry = (*presets.FilteredToolRegistry)(nil)
