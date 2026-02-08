package toolregistry

import (
	"context"
	"runtime"
	"slices"
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

func TestRegistryListExcludesLegacyCompatibilityTools(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	defs := registry.List()
	names := make([]string, 0, len(defs))
	for _, def := range defs {
		names = append(names, def.Name)
	}

	for _, legacy := range []string{"file_read", "file_write", "file_edit", "list_files", "bash", "code_execute"} {
		if slices.Contains(names, legacy) {
			t.Fatalf("legacy tool %s should not be listed", legacy)
		}
	}
	for _, canonical := range []string{"read_file", "write_file", "replace_in_file", "list_dir", "shell_exec", "execute_code"} {
		if !slices.Contains(names, canonical) {
			t.Fatalf("canonical tool %s must be listed", canonical)
		}
	}
}

func TestRegistryLegacyAliasesRemainResolvable(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	for aliasName, canonicalName := range map[string]string{
		"file_read":    "read_file",
		"file_write":   "write_file",
		"file_edit":    "replace_in_file",
		"list_files":   "list_dir",
		"bash":         "shell_exec",
		"code_execute": "execute_code",
	} {
		tool, err := registry.Get(aliasName)
		if err != nil {
			t.Fatalf("expected alias %s to resolve: %v", aliasName, err)
		}
		if tool.Metadata().Name != aliasName {
			t.Fatalf("expected alias metadata name %s, got %s", aliasName, tool.Metadata().Name)
		}
		def := tool.Definition()
		if def.Name != aliasName {
			t.Fatalf("expected alias definition name %s, got %s", aliasName, def.Name)
		}
		if !strings.Contains(def.Description, canonicalName) {
			t.Fatalf("expected alias %s description to mention %s", aliasName, canonicalName)
		}
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

	if _, err := registry.Get("read_file"); err != nil {
		t.Fatalf("failed to get read_file: %v", err)
	}
	if _, err := registry.Get("browser_action"); err != nil {
		t.Fatalf("failed to get browser_action: %v", err)
	}
	if runtime.GOOS == "darwin" {
		if _, err := registry.Get("peekaboo_exec"); err != nil {
			t.Fatalf("expected peekaboo_exec to be registered on darwin: %v", err)
		}
	} else {
		if _, err := registry.Get("peekaboo_exec"); err == nil {
			t.Fatalf("expected peekaboo_exec to be absent on non-darwin platforms")
		}
	}
	if _, err := registry.Get("write_attachment"); err != nil {
		t.Fatalf("expected write_attachment to be registered for lark-local toolset: %v", err)
	}
}

func TestNewRegistryRegistersSeedreamVideoByDefault(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryEngine:       newTestMemoryEngine(t),
		ArkAPIKey:          "test",
		SeedreamVideoModel: "",
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}
	if _, err := registry.Get("video_generate"); err != nil {
		t.Fatalf("expected video_generate to be registered by default: %v", err)
	}
}

func TestSeedreamVideoToolMetadataAndDefinition(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryEngine:       newTestMemoryEngine(t),
		ArkAPIKey:          "test",
		SeedreamVideoModel: " custom-video-model ",
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	tool, err := registry.Get("video_generate")
	if err != nil {
		t.Fatalf("expected video_generate to be registered: %v", err)
	}

	metadata := tool.Metadata()
	if metadata.Name != "video_generate" {
		t.Fatalf("unexpected metadata name: %s", metadata.Name)
	}
	if metadata.Category != "design" {
		t.Fatalf("expected design category, got %s", metadata.Category)
	}
	if !slices.Contains(metadata.Tags, "video") {
		t.Fatalf("expected metadata tags to include video: %v", metadata.Tags)
	}

	def := tool.Definition()
	if def.Name != "video_generate" {
		t.Fatalf("unexpected definition name: %s", def.Name)
	}
	if !strings.Contains(def.Description, "Seedance") {
		t.Fatalf("expected definition description to reference Seedance, got %q", def.Description)
	}
	if !slices.Contains(def.Parameters.Required, "duration_seconds") {
		t.Fatalf("expected duration_seconds to be required: %v", def.Parameters.Required)
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

func TestToolDefinitionsArrayItemsIncludesOptionalTools(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryEngine:        newTestMemoryEngine(t),
		SeedreamVisionModel: "seedream-vision",
	})
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
	if _, ok := retry.delegate.(*approvalExecutor); !ok {
		t.Fatalf("expected retry delegate to be *approvalExecutor, got %T", retry.delegate)
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

func TestRegistry_DefaultDegradationWrapsMappedToolsOnly(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryEngine: newTestMemoryEngine(t)})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	grepTool, err := registry.Get("grep")
	if err != nil {
		t.Fatalf("failed to get grep: %v", err)
	}
	if _, ok := grepTool.(*degradationExecutor); !ok {
		t.Fatalf("expected grep to be wrapped by degradationExecutor, got %T", grepTool)
	}

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

func TestApprovalExecutor_EnrichesSafetyContext(t *testing.T) {
	executor := &approvalExecutor{
		delegate: &stubExecutor{
			name:        "file_delete",
			dangerous:   true,
			safetyLevel: ports.SafetyLevelIrreversible,
		},
	}
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
