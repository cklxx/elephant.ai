package toolregistry

import (
	"context"
	"slices"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/memory"
)

func newTestMemoryService() memory.Service {
	return memory.NewService(memory.NewInMemoryStore())
}

func TestNewRegistryRegistersBuiltins(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryService: newTestMemoryService()})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	if _, err := registry.Get("file_read"); err != nil {
		t.Fatalf("failed to get file_read: %v", err)
	}
}

func TestNewRegistryRegistersSeedreamVideoByDefault(t *testing.T) {
	registry, err := NewRegistry(Config{
		MemoryService:      newTestMemoryService(),
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
		MemoryService:      newTestMemoryService(),
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

type stubLLMFactory struct {
	provider string
	model    string
	client   ports.LLMClient
}

func (s *stubLLMFactory) GetClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	s.provider = provider
	s.model = model
	return s.client, nil
}

func (s *stubLLMFactory) GetIsolatedClient(provider, model string, config ports.LLMConfig) (ports.LLMClient, error) {
	return s.GetClient(provider, model, config)
}

func (*stubLLMFactory) DisableRetry() {}

type countingLLM struct {
	calls int
}

func (c *countingLLM) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
	c.calls++
	return &ports.CompletionResponse{
		Content: "<html><body>llm output</body></html>",
	}, nil
}

func (*countingLLM) Model() string { return "stub-model" }

func TestMiniAppHTMLUsesConfiguredLLM(t *testing.T) {
	llm := &countingLLM{}
	factory := &stubLLMFactory{client: llm}

	registry, err := NewRegistry(Config{
		MemoryService: newTestMemoryService(),
		LLMFactory:    factory,
		LLMProvider:   "openai",
		LLMModel:      "gpt-4o",
	})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	if factory.provider != "openai" || factory.model != "gpt-4o" {
		t.Fatalf("expected factory to be called with provider/model, got %q/%q", factory.provider, factory.model)
	}

	tool, err := registry.Get("miniapp_html")
	if err != nil {
		t.Fatalf("expected miniapp_html to be registered: %v", err)
	}

	if _, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "miniapp-1",
		Arguments: map[string]any{
			"prompt": "meme tapping game",
			"title":  "Meme Tap",
		},
	}); err != nil {
		t.Fatalf("expected tool execution to succeed: %v", err)
	}

	if llm.calls == 0 {
		t.Fatalf("expected configured LLM to be invoked")
	}
	if factory.provider != "openai" || factory.model != "gpt-4o" {
		t.Fatalf("expected factory to be called with provider/model, got %q/%q", factory.provider, factory.model)
	}
}

func TestToolDefinitionsArrayItems(t *testing.T) {
	registry, err := NewRegistry(Config{MemoryService: newTestMemoryService()})
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
		MemoryService:       newTestMemoryService(),
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

func (stubCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, listener ports.EventListener) (*ports.TaskResult, error) {
	return nil, nil
}

func (stubCoordinator) PrepareExecution(ctx context.Context, task string, sessionID string) (*ports.ExecutionEnvironment, error) {
	return nil, nil
}

func (stubCoordinator) SaveSessionAfterExecution(ctx context.Context, session *ports.Session, result *ports.TaskResult) error {
	return nil
}

func (stubCoordinator) ListSessions(ctx context.Context) ([]string, error) {
	return nil, nil
}

func (stubCoordinator) GetConfig() ports.AgentConfig {
	return ports.AgentConfig{}
}

func (stubCoordinator) GetLLMClient() (ports.LLMClient, error) {
	return nil, nil
}

func (stubCoordinator) GetToolRegistryWithoutSubagent() ports.ToolRegistry {
	return nil
}

func (stubCoordinator) GetParser() ports.FunctionCallParser {
	return nil
}

func (stubCoordinator) GetContextManager() ports.ContextManager {
	return nil
}

func (stubCoordinator) GetSystemPrompt() string {
	return ""
}
