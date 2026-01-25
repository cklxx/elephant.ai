package llm

import (
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/jsonx"
)

func TestConvertToolsNormalizesSchema(t *testing.T) {
	tools := []ports.ToolDefinition{{
		Name:       "vision_analyze",
		Parameters: ports.ParameterSchema{Type: "object"},
	}}
	converted := convertTools(tools)
	schema := extractOpenAIToolSchema(t, converted)
	assertObjectSchema(t, schema)
}

func TestConvertCodexToolsNormalizesSchema(t *testing.T) {
	tools := []ports.ToolDefinition{{
		Name:       "vision_analyze",
		Parameters: ports.ParameterSchema{Type: "object"},
	}}
	converted := convertCodexTools(tools)
	schema := extractCodexToolSchema(t, converted)
	assertObjectSchema(t, schema)
}

func TestConvertAnthropicToolsNormalizesSchema(t *testing.T) {
	tools := []ports.ToolDefinition{{
		Name:       "vision_analyze",
		Parameters: ports.ParameterSchema{Type: "object"},
	}}
	converted := convertAnthropicTools(tools)
	schema := extractAnthropicToolSchema(t, converted)
	assertObjectSchema(t, schema)
}

func TestConvertAntigravityToolsNormalizesSchema(t *testing.T) {
	tools := []ports.ToolDefinition{{
		Name:       "vision_analyze",
		Parameters: ports.ParameterSchema{Type: "object"},
	}}
	converted := convertAntigravityTools(tools)
	if len(converted) != 1 {
		t.Fatalf("expected one tool wrapper, got %d", len(converted))
	}
	declarations, ok := converted[0]["functionDeclarations"].([]map[string]any)
	if !ok {
		t.Fatalf("expected functionDeclarations to be []map[string]any, got %T", converted[0]["functionDeclarations"])
	}
	if len(declarations) != 1 {
		t.Fatalf("expected one declaration, got %d", len(declarations))
	}
	schema, ok := declarations[0]["parametersJsonSchema"].(map[string]any)
	if !ok {
		t.Fatalf("expected parametersJsonSchema map, got %T", declarations[0]["parametersJsonSchema"])
	}
	assertObjectSchema(t, schema)
}

func extractOpenAIToolSchema(t *testing.T, converted []map[string]any) map[string]any {
	t.Helper()
	decoded := decodeToolPayload(t, converted)
	function, ok := decoded[0]["function"].(map[string]any)
	if !ok {
		t.Fatalf("expected function map, got %T", decoded[0]["function"])
	}
	schema, ok := function["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("expected parameters map, got %T", function["parameters"])
	}
	return schema
}

func extractCodexToolSchema(t *testing.T, converted []map[string]any) map[string]any {
	t.Helper()
	decoded := decodeToolPayload(t, converted)
	schema, ok := decoded[0]["parameters"].(map[string]any)
	if !ok {
		t.Fatalf("expected parameters map, got %T", decoded[0]["parameters"])
	}
	return schema
}

func extractAnthropicToolSchema(t *testing.T, converted []map[string]any) map[string]any {
	t.Helper()
	decoded := decodeToolPayload(t, converted)
	schema, ok := decoded[0]["input_schema"].(map[string]any)
	if !ok {
		t.Fatalf("expected input_schema map, got %T", decoded[0]["input_schema"])
	}
	return schema
}

func decodeToolPayload(t *testing.T, payload any) []map[string]any {
	t.Helper()
	data, err := jsonx.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal payload: %v", err)
	}
	var decoded []map[string]any
	if err := jsonx.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("failed to unmarshal payload: %v", err)
	}
	if len(decoded) == 0 {
		t.Fatalf("expected non-empty payload")
	}
	return decoded
}

func assertObjectSchema(t *testing.T, schema map[string]any) {
	t.Helper()
	if schema["type"] != "object" {
		t.Fatalf("expected type object, got %v", schema["type"])
	}
	if props, ok := schema["properties"].(map[string]any); !ok || props == nil {
		t.Fatalf("expected properties map, got %T", schema["properties"])
	}
}
