package toolregistry

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
)

type validationStubTool struct {
	def  ports.ToolDefinition
	meta ports.ToolMetadata
}

func (t *validationStubTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	return &ports.ToolResult{CallID: call.ID, Content: "executed"}, nil
}
func (t *validationStubTool) Definition() ports.ToolDefinition { return t.def }
func (t *validationStubTool) Metadata() ports.ToolMetadata     { return t.meta }

var _ tools.ToolExecutor = (*validationStubTool)(nil)

func newValidationTestTool() *validationStubTool {
	return &validationStubTool{
		def: ports.ToolDefinition{
			Name: "test_tool",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"name":    {Type: "string"},
					"count":   {Type: "number"},
					"verbose": {Type: "boolean"},
					"items":   {Type: "array"},
				},
				Required: []string{"name"},
			},
		},
		meta: ports.ToolMetadata{Name: "test_tool"},
	}
}

func TestValidatingExecutor_ValidArgs(t *testing.T) {
	tool := newValidationTestTool()
	v := &validatingExecutor{delegate: tool}

	result, err := v.Execute(context.Background(), ports.ToolCall{
		ID:   "c1",
		Name: "test_tool",
		Arguments: map[string]any{
			"name":    "hello",
			"count":   float64(42),
			"verbose": true,
			"items":   []any{"a", "b"},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success, got error: %v", result.Error)
	}
	if result.Content != "executed" {
		t.Fatalf("expected 'executed', got %q", result.Content)
	}
}

func TestValidatingExecutor_MissingRequired(t *testing.T) {
	tool := newValidationTestTool()
	v := &validatingExecutor{delegate: tool}

	result, err := v.Execute(context.Background(), ports.ToolCall{
		ID:        "c2",
		Name:      "test_tool",
		Arguments: map[string]any{"count": float64(1)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected validation error for missing required field")
	}
	if result.Content == "executed" {
		t.Fatal("delegate should not have been called")
	}
}

func TestValidatingExecutor_WrongType(t *testing.T) {
	tool := newValidationTestTool()
	v := &validatingExecutor{delegate: tool}

	result, err := v.Execute(context.Background(), ports.ToolCall{
		ID:   "c3",
		Name: "test_tool",
		Arguments: map[string]any{
			"name":  123, // should be string
			"count": float64(1),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected validation error for wrong type")
	}
}

func TestValidatingExecutor_ExtraFieldsAllowed(t *testing.T) {
	tool := newValidationTestTool()
	v := &validatingExecutor{delegate: tool}

	result, err := v.Execute(context.Background(), ports.ToolCall{
		ID:   "c4",
		Name: "test_tool",
		Arguments: map[string]any{
			"name":     "hello",
			"unknown":  "extra",
			"another":  42,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected success with extra fields, got error: %v", result.Error)
	}
}

func TestValidatingExecutor_NoSchema(t *testing.T) {
	tool := &validationStubTool{
		def:  ports.ToolDefinition{Name: "bare_tool"},
		meta: ports.ToolMetadata{Name: "bare_tool"},
	}
	v := &validatingExecutor{delegate: tool}

	result, err := v.Execute(context.Background(), ports.ToolCall{
		ID:        "c5",
		Name:      "bare_tool",
		Arguments: map[string]any{"anything": "goes"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected pass-through for tool without schema, got error: %v", result.Error)
	}
}

func TestValidatingExecutor_IntegerAcceptsFloat64(t *testing.T) {
	tool := &validationStubTool{
		def: ports.ToolDefinition{
			Name: "int_tool",
			Parameters: ports.ParameterSchema{
				Type: "object",
				Properties: map[string]ports.Property{
					"page": {Type: "integer"},
				},
				Required: []string{"page"},
			},
		},
		meta: ports.ToolMetadata{Name: "int_tool"},
	}
	v := &validatingExecutor{delegate: tool}

	result, err := v.Execute(context.Background(), ports.ToolCall{
		ID:        "c6",
		Name:      "int_tool",
		Arguments: map[string]any{"page": float64(5)},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected float64 accepted as integer, got error: %v", result.Error)
	}
}
