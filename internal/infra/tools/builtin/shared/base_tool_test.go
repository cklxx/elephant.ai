package shared

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestNewBaseTool_BothNamesSet(t *testing.T) {
	bt := NewBaseTool(
		ports.ToolDefinition{Name: "my_tool"},
		ports.ToolMetadata{Name: "my_tool"},
	)
	if bt.Definition().Name != "my_tool" {
		t.Fatalf("expected def name 'my_tool', got %q", bt.Definition().Name)
	}
	if bt.Metadata().Name != "my_tool" {
		t.Fatalf("expected meta name 'my_tool', got %q", bt.Metadata().Name)
	}
}

func TestNewBaseTool_DefOnly(t *testing.T) {
	bt := NewBaseTool(
		ports.ToolDefinition{Name: "from_def"},
		ports.ToolMetadata{Category: "test"},
	)
	if bt.Definition().Name != "from_def" {
		t.Fatalf("expected def name 'from_def', got %q", bt.Definition().Name)
	}
	if bt.Metadata().Name != "from_def" {
		t.Fatalf("expected meta name auto-synced to 'from_def', got %q", bt.Metadata().Name)
	}
}

func TestNewBaseTool_MetaOnly(t *testing.T) {
	bt := NewBaseTool(
		ports.ToolDefinition{Description: "test"},
		ports.ToolMetadata{Name: "from_meta"},
	)
	if bt.Definition().Name != "from_meta" {
		t.Fatalf("expected def name auto-synced to 'from_meta', got %q", bt.Definition().Name)
	}
	if bt.Metadata().Name != "from_meta" {
		t.Fatalf("expected meta name 'from_meta', got %q", bt.Metadata().Name)
	}
}

func TestNewBaseTool_BothEmpty(t *testing.T) {
	bt := NewBaseTool(ports.ToolDefinition{}, ports.ToolMetadata{})
	if bt.Definition().Name != "" {
		t.Fatalf("expected empty def name, got %q", bt.Definition().Name)
	}
	if bt.Metadata().Name != "" {
		t.Fatalf("expected empty meta name, got %q", bt.Metadata().Name)
	}
}
