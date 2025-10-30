package mcp

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/utils"
)

// MCPClient defines the interface for calling MCP tools
type MCPClient interface {
	CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolCallResult, error)
}

// ToolAdapter adapts an MCP tool to the ALEX ToolExecutor interface
type ToolAdapter struct {
	serverName string
	client     MCPClient
	toolSchema ToolSchema
	logger     *utils.Logger
}

// NewToolAdapter creates a new tool adapter
func NewToolAdapter(serverName string, client MCPClient, toolSchema ToolSchema) *ToolAdapter {
	return &ToolAdapter{
		serverName: serverName,
		client:     client,
		toolSchema: toolSchema,
		logger:     utils.NewComponentLogger(fmt.Sprintf("ToolAdapter[%s/%s]", serverName, toolSchema.Name)),
	}
}

// Execute implements ports.ToolExecutor.Execute
func (t *ToolAdapter) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.logger.Debug("Executing MCP tool: %s with args: %v", call.Name, call.Arguments)

	// Call the MCP tool
	result, err := t.client.CallTool(ctx, t.toolSchema.Name, call.Arguments)
	if err != nil {
		t.logger.Error("MCP tool call failed: %v", err)
		return &ports.ToolResult{
			CallID:       call.ID,
			Error:        fmt.Errorf("MCP tool call failed: %w", err),
			SessionID:    call.SessionID,
			TaskID:       call.TaskID,
			ParentTaskID: call.ParentTaskID,
		}, nil
	}

	// Handle error result from MCP
	if result.IsError {
		errMsg := t.formatContent(result.Content)
		t.logger.Warn("MCP tool returned error: %s", errMsg)
		return &ports.ToolResult{
			CallID:       call.ID,
			Error:        fmt.Errorf("MCP tool error: %s", errMsg),
			SessionID:    call.SessionID,
			TaskID:       call.TaskID,
			ParentTaskID: call.ParentTaskID,
		}, nil
	}

	// Format the content blocks into a single string
	content := t.formatContent(result.Content)
	t.logger.Debug("MCP tool succeeded: content_length=%d", len(content))

	return &ports.ToolResult{
		CallID:       call.ID,
		Content:      content,
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
		Metadata: map[string]any{
			"mcp_server": t.serverName,
			"tool_name":  t.toolSchema.Name,
		},
	}, nil
}

// Definition implements ports.ToolExecutor.Definition
func (t *ToolAdapter) Definition() ports.ToolDefinition {
	// Convert MCP input schema to ALEX parameter schema
	paramSchema := t.convertInputSchema()

	return ports.ToolDefinition{
		Name:        t.getPrefixedName(),
		Description: t.formatDescription(),
		Parameters:  paramSchema,
	}
}

// Metadata implements ports.ToolExecutor.Metadata
func (t *ToolAdapter) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     t.getPrefixedName(),
		Version:  "1.0.0",
		Category: "mcp_tools",
		Tags:     []string{"mcp", t.serverName},
	}
}

// getPrefixedName returns the tool name prefixed with server name
func (t *ToolAdapter) getPrefixedName() string {
	return fmt.Sprintf("mcp__%s__%s", t.serverName, t.toolSchema.Name)
}

// formatDescription adds server context to the tool description
func (t *ToolAdapter) formatDescription() string {
	return fmt.Sprintf("[MCP:%s] %s", t.serverName, t.toolSchema.Description)
}

// convertInputSchema converts MCP JSON schema to ALEX parameter schema
func (t *ToolAdapter) convertInputSchema() ports.ParameterSchema {
	schema := ports.ParameterSchema{
		Type:       "object",
		Properties: make(map[string]ports.Property),
		Required:   []string{},
	}

	// Extract properties from MCP input schema
	if inputSchema, ok := t.toolSchema.InputSchema["properties"].(map[string]interface{}); ok {
		for propName, propValue := range inputSchema {
			if propMap, ok := propValue.(map[string]interface{}); ok {
				prop := ports.Property{}

				// Extract type
				if typeVal, ok := propMap["type"].(string); ok {
					prop.Type = typeVal
				}

				// Extract description
				if descVal, ok := propMap["description"].(string); ok {
					prop.Description = descVal
				}

				// Extract enum if present
				if enumVal, ok := propMap["enum"].([]interface{}); ok {
					prop.Enum = enumVal
				}

				schema.Properties[propName] = prop
			}
		}
	}

	// Extract required fields
	if required, ok := t.toolSchema.InputSchema["required"].([]interface{}); ok {
		for _, req := range required {
			if reqStr, ok := req.(string); ok {
				schema.Required = append(schema.Required, reqStr)
			}
		}
	}

	return schema
}

// formatContent converts MCP content blocks to a single string
func (t *ToolAdapter) formatContent(blocks []ContentBlock) string {
	var parts []string

	for _, block := range blocks {
		switch block.Type {
		case "text":
			if block.Text != "" {
				parts = append(parts, block.Text)
			}
		case "image":
			// For images, include a description
			if block.MimeType != "" {
				parts = append(parts, fmt.Sprintf("[Image: %s]", block.MimeType))
			} else {
				parts = append(parts, "[Image]")
			}
			if block.Data != "" {
				// Include first few characters of data as preview
				preview := block.Data
				if len(preview) > 100 {
					preview = preview[:100] + "..."
				}
				parts = append(parts, fmt.Sprintf("Data: %s", preview))
			}
		case "resource":
			// For resources, include a reference
			parts = append(parts, fmt.Sprintf("[Resource: %s]", block.Text))
		default:
			// Unknown block type
			t.logger.Warn("Unknown content block type: %s", block.Type)
			parts = append(parts, fmt.Sprintf("[%s]", block.Type))
		}
	}

	return strings.Join(parts, "\n\n")
}

// ValidateArguments validates tool arguments against the schema
func (t *ToolAdapter) ValidateArguments(args map[string]interface{}) error {
	// Extract required fields from schema
	required := []string{}
	if reqInterface, ok := t.toolSchema.InputSchema["required"].([]interface{}); ok {
		for _, req := range reqInterface {
			if reqStr, ok := req.(string); ok {
				required = append(required, reqStr)
			}
		}
	}

	// Check all required fields are present
	for _, field := range required {
		if _, exists := args[field]; !exists {
			return fmt.Errorf("missing required argument: %s", field)
		}
	}

	return nil
}
