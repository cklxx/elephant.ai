package toolregistry

import (
	"slices"
	"strings"
	"testing"

	"alex/internal/tools"
)

func TestNewRegistrySandboxRequiresManager(t *testing.T) {
	_, err := NewRegistry(Config{ExecutionMode: tools.ExecutionModeSandbox})
	if err == nil {
		t.Fatalf("expected error when sandbox manager missing in sandbox mode")
	}
}

func TestNewRegistryRejectsInvalidExecutionMode(t *testing.T) {
	_, err := NewRegistry(Config{ExecutionMode: tools.ExecutionMode(99)})
	if err == nil {
		t.Fatalf("expected error for invalid execution mode")
	}
}

func TestNewRegistryLocalModeSetsLocalTools(t *testing.T) {
	registry, err := NewRegistry(Config{ExecutionMode: tools.ExecutionModeLocal})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	tool, err := registry.Get("file_read")
	if err != nil {
		t.Fatalf("failed to get file_read: %v", err)
	}

	modeTool, ok := tool.(interface{ Mode() tools.ExecutionMode })
	if !ok {
		t.Fatalf("tool does not expose Mode accessor")
	}
	if mode := modeTool.Mode(); mode != tools.ExecutionModeLocal {
		t.Fatalf("expected local mode, got %v", mode)
	}

	if _, err := registry.Get("browser_info"); err == nil {
		t.Fatalf("browser_info should not be registered in local mode")
	}
}

func TestNewRegistrySandboxRegistersBrowserTools(t *testing.T) {
	manager := tools.NewSandboxManager("http://example.com")
	registry, err := NewRegistry(Config{ExecutionMode: tools.ExecutionModeSandbox, SandboxManager: manager})
	if err != nil {
		t.Fatalf("unexpected error creating registry: %v", err)
	}

	tool, err := registry.Get("file_read")
	if err != nil {
		t.Fatalf("failed to get file_read: %v", err)
	}
	modeTool, ok := tool.(interface{ Mode() tools.ExecutionMode })
	if !ok {
		t.Fatalf("tool does not expose Mode accessor")
	}
	if mode := modeTool.Mode(); mode != tools.ExecutionModeSandbox {
		t.Fatalf("expected sandbox mode, got %v", mode)
	}

	browserTool, err := registry.Get("browser_info")
	if err != nil {
		t.Fatalf("expected browser_info in sandbox registry: %v", err)
	}
	if _, ok := browserTool.(interface{ Mode() tools.ExecutionMode }); !ok {
		t.Fatalf("browser_info does not expose Mode accessor")
	}
}

func TestNewRegistryRegistersSeedreamVideoByDefault(t *testing.T) {
	registry, err := NewRegistry(Config{
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
