package mcp

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
)

// MockClient is a mock MCP client for testing
type MockClient struct {
	lastToolName string
	lastArgs     map[string]interface{}
	returnResult *ToolCallResult
	returnError  error
}

func (m *MockClient) CallTool(ctx context.Context, name string, args map[string]interface{}) (*ToolCallResult, error) {
	m.lastToolName = name
	m.lastArgs = args
	return m.returnResult, m.returnError
}

func TestToolAdapter_Execute_Success(t *testing.T) {
	mockClient := &MockClient{
		returnResult: &ToolCallResult{
			Content: []ContentBlock{
				{Type: "text", Text: "Tool executed successfully"},
			},
			IsError: false,
		},
	}

	schema := ToolSchema{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"param1": map[string]interface{}{
					"type":        "string",
					"description": "First parameter",
				},
			},
		},
	}

	adapter := NewToolAdapter("test-server", mockClient, schema)

	result, err := adapter.Execute(context.Background(), ports.ToolCall{
		ID:   "call-1",
		Name: "mcp__test-server__test_tool",
		Arguments: map[string]any{
			"param1": "value1",
		},
	})

	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if result.Error != nil {
		t.Errorf("Expected no result error, got %v", result.Error)
	}

	if result.Content != "Tool executed successfully" {
		t.Errorf("Expected content 'Tool executed successfully', got %s", result.Content)
	}

	if result.CallID != "call-1" {
		t.Errorf("Expected CallID 'call-1', got %s", result.CallID)
	}

	// Verify metadata
	if result.Metadata["mcp_server"] != "test-server" {
		t.Errorf("Expected metadata mcp_server='test-server', got %v", result.Metadata["mcp_server"])
	}
}

func TestToolAdapter_Execute_Error(t *testing.T) {
	mockClient := &MockClient{
		returnResult: &ToolCallResult{
			Content: []ContentBlock{
				{Type: "text", Text: "Error: invalid parameter"},
			},
			IsError: true,
		},
	}

	schema := ToolSchema{
		Name:        "test_tool",
		Description: "A test tool",
		InputSchema: map[string]interface{}{},
	}

	adapter := NewToolAdapter("test-server", mockClient, schema)

	result, err := adapter.Execute(context.Background(), ports.ToolCall{
		ID:        "call-1",
		Name:      "mcp__test-server__test_tool",
		Arguments: map[string]any{},
	})

	if err != nil {
		t.Fatalf("Expected no transport error, got %v", err)
	}

	if result.Error == nil {
		t.Error("Expected result to contain error")
	}
}

func TestToolAdapter_Definition(t *testing.T) {
	schema := ToolSchema{
		Name:        "read_file",
		Description: "Read a file",
		InputSchema: map[string]interface{}{
			"type": "object",
			"properties": map[string]interface{}{
				"path": map[string]interface{}{
					"type":        "string",
					"description": "File path",
				},
				"encoding": map[string]interface{}{
					"type":        "string",
					"description": "File encoding",
					"enum":        []interface{}{"utf-8", "ascii"},
				},
			},
			"required": []interface{}{"path"},
		},
	}

	adapter := NewToolAdapter("filesystem", nil, schema)
	def := adapter.Definition()

	// Check name is prefixed
	expectedName := "mcp__filesystem__read_file"
	if def.Name != expectedName {
		t.Errorf("Expected name %s, got %s", expectedName, def.Name)
	}

	// Check description is prefixed
	if def.Description != "[MCP:filesystem] Read a file" {
		t.Errorf("Expected prefixed description, got %s", def.Description)
	}

	// Check parameters were converted
	if def.Parameters.Type != "object" {
		t.Errorf("Expected type 'object', got %s", def.Parameters.Type)
	}

	pathProp, exists := def.Parameters.Properties["path"]
	if !exists {
		t.Fatal("Expected 'path' parameter to exist")
	}
	if pathProp.Type != "string" {
		t.Errorf("Expected path type 'string', got %s", pathProp.Type)
	}

	// Check required fields
	if len(def.Parameters.Required) != 1 || def.Parameters.Required[0] != "path" {
		t.Errorf("Expected required=['path'], got %v", def.Parameters.Required)
	}

	// Check enum was preserved
	encodingProp, exists := def.Parameters.Properties["encoding"]
	if !exists {
		t.Fatal("Expected 'encoding' parameter to exist")
	}
	if len(encodingProp.Enum) != 2 {
		t.Errorf("Expected 2 enum values, got %d", len(encodingProp.Enum))
	}
}

func TestToolAdapter_Metadata(t *testing.T) {
	schema := ToolSchema{
		Name:        "test_tool",
		Description: "Test tool",
		InputSchema: map[string]interface{}{},
	}

	adapter := NewToolAdapter("test-server", nil, schema)
	metadata := adapter.Metadata()

	if metadata.Name != "mcp__test-server__test_tool" {
		t.Errorf("Expected name 'mcp__test-server__test_tool', got %s", metadata.Name)
	}

	if metadata.Category != "mcp_tools" {
		t.Errorf("Expected category 'mcp_tools', got %s", metadata.Category)
	}

	// Check tags
	foundMCP := false
	foundServer := false
	for _, tag := range metadata.Tags {
		if tag == "mcp" {
			foundMCP = true
		}
		if tag == "test-server" {
			foundServer = true
		}
	}
	if !foundMCP {
		t.Error("Expected 'mcp' tag")
	}
	if !foundServer {
		t.Error("Expected 'test-server' tag")
	}
}

func TestToolAdapter_FormatContent(t *testing.T) {
	adapter := NewToolAdapter("test", nil, ToolSchema{})

	tests := []struct {
		name     string
		blocks   []ContentBlock
		expected string
	}{
		{
			name: "single text block",
			blocks: []ContentBlock{
				{Type: "text", Text: "Hello, world!"},
			},
			expected: "Hello, world!",
		},
		{
			name: "multiple text blocks",
			blocks: []ContentBlock{
				{Type: "text", Text: "First block"},
				{Type: "text", Text: "Second block"},
			},
			expected: "First block\n\nSecond block",
		},
		{
			name: "image block",
			blocks: []ContentBlock{
				{Type: "image", MimeType: "image/png", Data: "base64data"},
			},
			expected: "[Image: image/png]\n\nData: base64data",
		},
		{
			name: "resource block",
			blocks: []ContentBlock{
				{Type: "resource", Text: "file:///path/to/file"},
			},
			expected: "[Resource: file:///path/to/file]",
		},
		{
			name: "mixed blocks",
			blocks: []ContentBlock{
				{Type: "text", Text: "Some text"},
				{Type: "image", MimeType: "image/jpeg"},
				{Type: "resource", Text: "resource-uri"},
			},
			expected: "Some text\n\n[Image: image/jpeg]\n\n[Resource: resource-uri]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := adapter.formatContent(tt.blocks)
			if result != tt.expected {
				t.Errorf("Expected:\n%s\n\nGot:\n%s", tt.expected, result)
			}
		})
	}
}

func TestToolAdapter_ValidateArguments(t *testing.T) {
	schema := ToolSchema{
		Name: "test_tool",
		InputSchema: map[string]interface{}{
			"required": []interface{}{"param1", "param2"},
		},
	}

	adapter := NewToolAdapter("test", nil, schema)

	tests := []struct {
		name      string
		args      map[string]interface{}
		expectErr bool
	}{
		{
			name: "all required present",
			args: map[string]interface{}{
				"param1": "value1",
				"param2": "value2",
			},
			expectErr: false,
		},
		{
			name: "missing required param",
			args: map[string]interface{}{
				"param1": "value1",
			},
			expectErr: true,
		},
		{
			name:      "all missing",
			args:      map[string]interface{}{},
			expectErr: true,
		},
		{
			name: "extra params allowed",
			args: map[string]interface{}{
				"param1": "value1",
				"param2": "value2",
				"extra":  "value3",
			},
			expectErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := adapter.ValidateArguments(tt.args)
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no error, got %v", err)
			}
		})
	}
}
