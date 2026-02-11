package toolregistry

import (
	"context"
	"strings"
	"testing"

	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	llm "alex/internal/domain/agent/ports/llm"
	storage "alex/internal/domain/agent/ports/storage"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/memory"
	toolspolicy "alex/internal/infra/tools"
	"alex/internal/infra/tools/builtin/shared"
	runtimeconfig "alex/internal/shared/config"
)

func newTestMemoryEngine(t *testing.T) memory.Engine {
	t.Helper()
	engine := memory.NewMarkdownEngine(t.TempDir())
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	return engine
}

func TestNewRegistryRegistersBuiltins(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	if _, err := registry.Get("read_file"); err != nil {
		t.Fatalf("failed to get read_file: %v", err)
	}
}

func TestNewRegistryRegistersLarkLocalTools(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryEngine: newTestMemoryEngine(t),
		Toolset:      ToolsetLarkLocal,
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	for _, want := range []string{"read_file", "write_file", "replace_in_file", "shell_exec", "execute_code", "browser_action", "channel"} {
		if _, err := registry.Get(want); err != nil {
			t.Fatalf("expected %s to be registered for lark-local toolset: %v", want, err)
		}
	}
}

func TestNewRegistryRegistersExpectedToolCount(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}
	defs := registry.List()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}
	// 15 core tools: read_file, write_file, replace_in_file, shell_exec,
	// execute_code, browser_action, channel, web_search, skills,
	// plan, clarify, request_user, memory_search, memory_get, context_checkpoint
	if len(defs) != 15 {
		t.Fatalf("expected 15 tools, got %d: %v", len(defs), names)
	}
}

func TestToolDefinitionsArrayItems(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	defs := registry.List()
	for _, def := range defs {
		for name, prop := range def.Parameters.Properties {
			if prop.Type != "array" {
				continue
			}
			if prop.Items == nil {
				t.Fatalf("tool %s property %s missing items schema", def.Name, name)
			}
		}
	}
}

func TestToolDefinitionsArrayItemsIncludesSubagentTools(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	registry.RegisterSubAgent(stubCoordinator{})

	defs := registry.List()
	for _, def := range defs {
		for name, prop := range def.Parameters.Properties {
			if prop.Type != "array" {
				continue
			}
			if prop.Items == nil {
				t.Fatalf("tool %s property %s missing items schema", def.Name, name)
			}
		}
	}
}

type stubCoordinator struct{}

func (stubCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	return nil, nil
}

func (stubCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*agent.ExecutionEnvironment, error) {
	return nil, nil
}

func (stubCoordinator) SaveSessionAfterExecution(ctx context.Context, _ *storage.Session, _ *agent.TaskResult) error {
	return nil
}

func (stubCoordinator) ListSessions(ctx context.Context, limit int, offset int) ([]string, error) {
	return nil, nil
}

func (stubCoordinator) GetConfig() agent.AgentConfig {
	return agent.AgentConfig{}
}

func (stubCoordinator) GetLLMClient() (llm.LLMClient, error) {
	return nil, nil
}

func (stubCoordinator) GetToolRegistryWithoutSubagent() tools.ToolRegistry {
	return nil
}

func (stubCoordinator) GetParser() tools.FunctionCallParser {
	return nil
}

func (stubCoordinator) GetContextManager() agent.ContextManager {
	return nil
}

func (stubCoordinator) GetSystemPrompt() string {
	return ""
}

func TestGetReturnsPreWrappedTools(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	tool, err := registry.Get("read_file")
	if err != nil {
		t.Fatalf("failed to get read_file: %v", err)
	}

	// The tool should already be wrapped with idAwareExecutor
	if _, ok := tool.(*idAwareExecutor); !ok {
		t.Fatalf("expected tool to be *idAwareExecutor, got %T", tool)
	}

	// Calling Get twice should return the same pre-wrapped instance
	tool2, err := registry.Get("read_file")
	if err != nil {
		t.Fatalf("failed to get read_file second time: %v", err)
	}
	if tool != tool2 {
		t.Fatalf("expected Get to return the same pre-wrapped instance")
	}
}

func TestWrapToolDoesNotMutateInput(t *testing.T) {
	inner := &stubExecutor{name: "test_tool"}
	wrapped := &idAwareExecutor{delegate: inner}

	// Capture original delegate
	originalDelegate := wrapped.delegate

	policy := toolspolicy.NewToolPolicy(toolspolicy.DefaultToolPolicyConfig())
	breakers := newCircuitBreakerStore(normalizeCircuitBreakerConfig(CircuitBreakerConfig{}))
	result := wrapTool(wrapped, policy, breakers, nil)

	// The original idAwareExecutor should not be mutated
	if wrapped.delegate != originalDelegate {
		t.Fatalf("wrapTool mutated the input's delegate field")
	}

	// Result should be a new idAwareExecutor wrapping a retry executor.
	newWrapper, ok := result.(*idAwareExecutor)
	if !ok {
		t.Fatalf("expected *idAwareExecutor, got %T", result)
	}
	retry, ok := newWrapper.delegate.(*retryExecutor)
	if !ok {
		t.Fatalf("expected delegate to be *retryExecutor, got %T", newWrapper.delegate)
	}
	if _, ok := retry.delegate.(*toolspolicy.ApprovalExecutor); !ok {
		t.Fatalf("expected retry delegate to be *ApprovalExecutor, got %T", retry.delegate)
	}
}

func TestWrapToolUnwrapsExistingDegradationLayer(t *testing.T) {
	base := &stubExecutor{name: "grep"}
	degCfg := DefaultDegradationConfig()
	degCfg.FallbackMap["grep"] = []string{"ripgrep"}
	degraded := NewDegradationExecutor(base, makeLookup(nil), degCfg)

	policy := toolspolicy.NewToolPolicy(toolspolicy.DefaultToolPolicyConfig())
	breakers := newCircuitBreakerStore(normalizeCircuitBreakerConfig(CircuitBreakerConfig{}))
	result := wrapTool(degraded, policy, breakers, nil)

	if got := unwrapTool(result); got != base {
		t.Fatalf("expected wrapTool to unwrap degradation layer before re-wrapping")
	}
}

func TestRegistry_DefaultDegradationHasEmptyFallbackMap(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	// With no fallback mappings, no tool should be wrapped with degradation.
	fileRead, err := registry.Get("read_file")
	if err != nil {
		t.Fatalf("failed to get read_file: %v", err)
	}
	if _, ok := fileRead.(*degradationExecutor); ok {
		t.Fatalf("expected read_file not to be wrapped by degradationExecutor")
	}
}

func TestListCachingWithDirtyFlag(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	// First call builds the cache
	defs1 := registry.List()
	if len(defs1) == 0 {
		t.Fatalf("expected non-empty definitions list")
	}

	// Second call should return cached result (same slice)
	defs2 := registry.List()
	if len(defs1) != len(defs2) {
		t.Fatalf("expected same length from cached List()")
	}

	// Registering a new tool should invalidate cache
	if err := registry.Register(&stubExecutor{name: "custom_test_tool"}); err != nil {
		t.Fatalf("register tool: %v", err)
	}
	defs3 := registry.List()
	if len(defs3) != len(defs1)+1 {
		t.Fatalf("expected one more definition after Register, got %d vs %d", len(defs3), len(defs1)+1)
	}

	// Unregistering should invalidate cache
	if err := registry.Unregister("custom_test_tool"); err != nil {
		t.Fatalf("unregister tool: %v", err)
	}
	defs4 := registry.List()
	if len(defs4) != len(defs1) {
		t.Fatalf("expected original count after Unregister, got %d vs %d", len(defs4), len(defs1))
	}
}

type stubExecutor struct {
	name        string
	dangerous   bool
	safetyLevel int
}

func (s *stubExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	return &ports.ToolResult{CallID: call.ID}, nil
}

func (s *stubExecutor) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: s.name, Description: "test tool"}
}

func (s *stubExecutor) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: s.name, Dangerous: s.dangerous, SafetyLevel: s.safetyLevel}
}

type captureApprover struct {
	request *tools.ApprovalRequest
}

func (c *captureApprover) RequestApproval(_ context.Context, req *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
	c.request = req
	return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
}

func TestNewRegistryRegistersOnlyCoreTools(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	defs := registry.List()
	names := make(map[string]bool, len(defs))
	for _, def := range defs {
		names[def.Name] = true
	}

	// Core tools MUST be present
	for _, want := range []string{
		"read_file", "write_file", "replace_in_file", "shell_exec",
		"execute_code", "browser_action",
		"plan", "clarify", "request_user",
		"memory_search", "memory_get",
		"web_search", "skills", "channel",
	} {
		if !names[want] {
			t.Errorf("expected tool %s to be registered", want)
		}
	}

	// Deprecated tools MUST NOT be present
	for _, dropped := range []string{
		"grep", "ripgrep", "find",
		"todo_read", "todo_update", "apps", "music_play",
		"artifacts_write", "artifacts_list", "artifacts_delete",
		"a2ui_emit", "artifact_manifest", "pptx_from_images",
		"acp_executor", "config_manage",
		"html_edit", "web_fetch", "douyin_hot",
		"text_to_image", "image_to_image", "video_generate",
		"diagram_render",
		"okr_read", "okr_write",
		"set_timer", "list_timers", "cancel_timer",
		"scheduler_create_job", "scheduler_list_jobs", "scheduler_delete_job",
		"browser_info", "browser_screenshot", "browser_dom",
		"list_dir", "search_file", "write_attachment",
		"lark_send_message", "lark_chat_history", "lark_upload_file",
		"lark_calendar_create", "lark_calendar_query",
		"lark_calendar_update", "lark_calendar_delete",
		"lark_task_manage",
	} {
		if names[dropped] {
			t.Errorf("deprecated tool %s should NOT be registered", dropped)
		}
	}
}

func TestNewRegistryQuickstartDisablesCredentialDependentTools(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryEngine: newTestMemoryEngine(t),
		Profile:      runtimeconfig.RuntimeProfileQuickstart,
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	if _, err := registry.Get("read_file"); err != nil {
		t.Fatalf("expected core tool read_file to remain registered: %v", err)
	}
	if _, err := registry.Get("web_search"); err == nil {
		t.Fatalf("expected web_search to be disabled in quickstart without Tavily key")
	}
}

func TestNewRegistryStandardKeepsOptionalToolsRegistered(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryEngine: newTestMemoryEngine(t),
		Profile:      runtimeconfig.RuntimeProfileStandard,
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	if _, err := registry.Get("web_search"); err != nil {
		t.Fatalf("expected web_search to remain registered in standard profile: %v", err)
	}
}

func TestApprovalExecutor_EnrichesSafetyContext(t *testing.T) {
	executor := toolspolicy.NewApprovalExecutor(&stubExecutor{
		name:        "file_delete",
		dangerous:   true,
		safetyLevel: ports.SafetyLevelIrreversible,
	})
	approver := &captureApprover{}
	ctx := shared.WithApprover(context.Background(), approver)

	_, err := executor.Execute(ctx, ports.ToolCall{
		ID:   "call-1",
		Name: "file_delete",
		Arguments: map[string]any{
			"path": "/tmp/demo.txt",
			"mode": "force",
		},
	})
	if err != nil {
		t.Fatalf("execute: %v", err)
	}
	if approver.request == nil {
		t.Fatal("expected approval request to be captured")
	}
	if approver.request.SafetyLevel != ports.SafetyLevelIrreversible {
		t.Fatalf("expected safety level L4, got %d", approver.request.SafetyLevel)
	}
	if approver.request.RollbackSteps == "" {
		t.Fatal("expected rollback steps for high-impact operations")
	}
	if approver.request.AlternativePlan == "" {
		t.Fatal("expected alternative plan for irreversible operations")
	}
	if !strings.Contains(approver.request.Summary, "L4") {
		t.Fatalf("expected summary to include safety level, got %q", approver.request.Summary)
	}
	if !strings.Contains(approver.request.Summary, "args=") {
		t.Fatalf("expected summary to include argument keys, got %q", approver.request.Summary)
	}
}
