package toolregistry

import (
	"context"
	"slices"
	"strings"
	"testing"

	agent "alex/internal/agent/ports/agent"
	llm "alex/internal/agent/ports/llm"
	storage "alex/internal/agent/ports/storage"
	tools "alex/internal/agent/ports/tools"
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
