package toolregistry

import (
	"context"
	"runtime"
	"slices"
	"strings"
	"testing"

	ports "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/memory"
	toolspolicy "alex/internal/tools"
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

	if _, err := registry.Get("file_read"); err != nil {
		t.Fatalf("failed to get file_read: %v", err)
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
	if _, err := registry.Get("write_attachment"); err == nil {
		t.Fatalf("expected write_attachment to be absent for lark-local toolset")
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

	tool, err := registry.Get("file_read")
	if err != nil {
		t.Fatalf("failed to get file_read: %v", err)
	}

	// The tool should already be wrapped with idAwareExecutor
	if _, ok := tool.(*idAwareExecutor); !ok {
		t.Fatalf("expected tool to be *idAwareExecutor, got %T", tool)
	}

	// Calling Get twice should return the same pre-wrapped instance
	tool2, err := registry.Get("file_read")
	if err != nil {
		t.Fatalf("failed to get file_read second time: %v", err)
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
	name string
}

func (s *stubExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	return &ports.ToolResult{CallID: call.ID}, nil
}

func (s *stubExecutor) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: s.name, Description: "test tool"}
}

func (s *stubExecutor) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: s.name}
}
