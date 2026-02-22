package llm

import (
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/json"
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

func TestNormalizeToolSchemaAddsItemsForArray(t *testing.T) {
	schema := ports.ParameterSchema{
		Type: "object",
		Properties: map[string]ports.Property{
			"name": {Type: "string", Description: "A name"},
			"tags": {Type: "array", Description: "A list of tags"},
		},
	}
	normalized := normalizeToolSchema(schema)

	tagsProp := normalized.Properties["tags"]
	if tagsProp.Items == nil {
		t.Fatal("Expected Items to be set for bare array property")
	}
	if tagsProp.Items.Type != "string" {
		t.Errorf("Expected default items type 'string', got %s", tagsProp.Items.Type)
	}

	// String properties should be unchanged
	nameProp := normalized.Properties["name"]
	if nameProp.Items != nil {
		t.Error("String property should not have Items set")
	}
}

func TestNormalizeToolSchemaPreservesExistingItems(t *testing.T) {
	items := &ports.Property{Type: "integer"}
	schema := ports.ParameterSchema{
		Type: "object",
		Properties: map[string]ports.Property{
			"numbers": {Type: "array", Items: items},
		},
	}
	normalized := normalizeToolSchema(schema)

	numProp := normalized.Properties["numbers"]
	if numProp.Items == nil {
		t.Fatal("Expected Items to remain set")
	}
	if numProp.Items.Type != "integer" {
		t.Errorf("Expected items type 'integer', got %s", numProp.Items.Type)
	}
}

func TestConvertCodexToolsArrayItemsInJSON(t *testing.T) {
	tools := []ports.ToolDefinition{{
		Name: "browser_click",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"selector": {Type: "string", Description: "CSS selector"},
				"modifiers": {
					Type:        "array",
					Description: "Keyboard modifiers",
					Items:       &ports.Property{Type: "string"},
				},
			},
		},
	}}
	converted := convertCodexTools(tools)
	if len(converted) != 1 {
		t.Fatalf("expected 1 tool, got %d", len(converted))
	}

	// Marshal/unmarshal to verify JSON round-trip includes items.
	data, err := jsonx.Marshal(converted)
	if err != nil {
		t.Fatalf("failed to marshal: %v", err)
	}
	payload := string(data)
	if !strings.Contains(payload, `"items"`) {
		t.Fatalf("Expected 'items' in serialized output — Codex rejects array schemas without it. Got: %s", payload)
	}
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
