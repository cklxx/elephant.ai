package preparation

import (
	"context"
	"sort"
	"testing"
	"time"

	appconfig "alex/internal/agent/app/config"
	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/app/cost"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/agent/presets"
)

func TestSelectToolRegistryUsesConfiguredPresetForCoreAgent(t *testing.T) {
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "think"}, {Name: "todo_read"}, {Name: "todo_update"}, {Name: "subagent"}, {Name: "final"}, {Name: "file_read"}}},
		SessionStore:  &stubSessionStore{session: &storage.Session{ID: "core", Metadata: map[string]string{}}},
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "stub", MaxIterations: 1, ToolPreset: string(presets.ToolPresetFull)},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	filtered := service.selectToolRegistry(context.Background(), presets.ToolModeCLI, service.config.ToolPreset)

	names := sortedToolNames(filtered.List())
	expected := []string{"file_read", "final", "subagent", "think", "todo_read", "todo_update"}

	if len(names) != len(expected) {
		t.Fatalf("core agent should see %d tools from full preset, got %v", len(expected), names)
	}
	for i, want := range expected {
		if names[i] != want {
			t.Fatalf("unexpected tool order/content: got %v, want %v", names, expected)
		}
	}
}

func TestSelectToolRegistryDefaultsToFullWhenUnset(t *testing.T) {
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "think"}, {Name: "todo_read"}, {Name: "todo_update"}, {Name: "subagent"}, {Name: "final"}, {Name: "file_read"}, {Name: "bash"}}},
		SessionStore:  &stubSessionStore{session: &storage.Session{ID: "core", Metadata: map[string]string{}}},
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "stub", MaxIterations: 1},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	filtered := service.selectToolRegistry(context.Background(), presets.ToolModeCLI, service.config.ToolPreset)

	names := sortedToolNames(filtered.List())
	expected := []string{"bash", "file_read", "final", "subagent", "think", "todo_read", "todo_update"}

	if len(names) != len(expected) {
		t.Fatalf("core agent should default to full preset tools, got %v", names)
	}
	for i, want := range expected {
		if names[i] != want {
			t.Fatalf("unexpected tool order/content: got %v, want %v", names, expected)
		}
	}
}

func TestSelectToolRegistryUsesConfiguredPresetForSubagents(t *testing.T) {
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "think"}, {Name: "subagent"}, {Name: "final"}, {Name: "file_read"}, {Name: "bash"}}},
		SessionStore:  &stubSessionStore{session: &storage.Session{ID: "sub", Metadata: map[string]string{}}},
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "stub", MaxIterations: 1, ToolPreset: string(presets.ToolPresetReadOnly)},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	ctx := appcontext.MarkSubagentContext(context.Background())
	filtered := service.selectToolRegistry(ctx, presets.ToolModeCLI, service.config.ToolPreset)
	names := sortedToolNames(filtered.List())

	if containsString(names, "bash") {
		t.Fatalf("subagent preset should block bash: %v", names)
	}
	if !containsString(names, "file_read") {
		t.Fatalf("subagent preset should retain file_read: %v", names)
	}
}

func TestSelectToolRegistryDoesNotStripExecutionToolsForSubagentsWhenPresetUnset(t *testing.T) {
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "think"}, {Name: "subagent"}, {Name: "explore"}, {Name: "final"}, {Name: "file_read"}, {Name: "bash"}}},
		SessionStore:  &stubSessionStore{session: &storage.Session{ID: "sub", Metadata: map[string]string{}}},
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "stub", MaxIterations: 1},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	ctx := appcontext.MarkSubagentContext(context.Background())
	filtered := service.selectToolRegistry(ctx, presets.ToolModeCLI, service.config.ToolPreset)
	names := sortedToolNames(filtered.List())

	if containsString(names, "subagent") {
		t.Fatalf("subagents should not be able to call other subagents: %v", names)
	}
	if containsString(names, "explore") {
		t.Fatalf("subagents should not be able to call explore (delegates to subagent): %v", names)
	}
	if !containsString(names, "bash") || !containsString(names, "file_read") {
		t.Fatalf("subagents should retain execution tools when preset unset, got: %v", names)
	}
}

func TestSelectToolRegistryUsesArchitectPresetInWebMode(t *testing.T) {
	deps := ExecutionPreparationDeps{
		LLMFactory:    &fakeLLMFactory{client: fakeLLMClient{}},
		ToolRegistry:  &registryWithList{defs: []ports.ToolDefinition{{Name: "plan"}, {Name: "clarify"}, {Name: "web_search"}, {Name: "web_fetch"}, {Name: "request_user"}, {Name: "acp_executor"}, {Name: "file_read"}, {Name: "bash"}}},
		SessionStore:  &stubSessionStore{session: &storage.Session{ID: "core", Metadata: map[string]string{}}},
		ContextMgr:    stubContextManager{},
		Parser:        stubParser{},
		Config:        appconfig.Config{LLMProvider: "mock", LLMModel: "stub", MaxIterations: 1, ToolPreset: string(presets.ToolPresetArchitect)},
		Logger:        agent.NoopLogger{},
		Clock:         agent.ClockFunc(func() time.Time { return time.Unix(0, 0) }),
		CostDecorator: cost.NewCostTrackingDecorator(nil, agent.NoopLogger{}, agent.ClockFunc(time.Now)),
		EventEmitter:  agent.NoopEventListener{},
	}

	service := NewExecutionPreparationService(deps)
	filtered := service.selectToolRegistry(context.Background(), presets.ToolModeWeb, string(presets.ToolPresetArchitect))
	names := sortedToolNames(filtered.List())

	if containsString(names, "bash") || containsString(names, "file_read") {
		t.Fatalf("web architect preset should block local tools, got: %v", names)
	}
	for _, allowed := range []string{"plan", "clarify", "web_search", "web_fetch", "request_user", "acp_executor"} {
		if !containsString(names, allowed) {
			t.Fatalf("expected tool %s in web architect preset, got: %v", allowed, names)
		}
	}
}

func sortedToolNames(defs []ports.ToolDefinition) []string {
	names := make([]string, len(defs))
	for i, def := range defs {
		names[i] = def.Name
	}
	sort.Strings(names)
	return names
}

func containsString(list []string, target string) bool {
	for _, item := range list {
		if item == target {
			return true
		}
	}
	return false
}
