package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/tools/builtin"
	"alex/internal/tools/mcp/protocol"
)

// MCPTool implements the Alex Tool interface for MCP tools
type MCPTool struct {
	client      *Client
	toolName    string
	description string
	schema      map[string]interface{}
}

// NewMCPTool creates a new MCP tool adapter
func NewMCPTool(client *Client, tool protocol.Tool) *MCPTool {
	// Parse the input schema
	var schema map[string]interface{}
	if tool.InputSchema != nil {
		_ = json.Unmarshal(tool.InputSchema, &schema)
	}

	return &MCPTool{
		client:      client,
		toolName:    tool.Name,
		description: tool.Description,
		schema:      schema,
	}
}

// Name returns the tool name
func (t *MCPTool) Name() string {
	return fmt.Sprintf("mcp_%s", t.toolName)
}

// Description returns the tool description
func (t *MCPTool) Description() string {
	if t.description != "" {
		return fmt.Sprintf("[MCP] %s", t.description)
	}
	return fmt.Sprintf("[MCP] %s", t.toolName)
}

// Parameters returns the tool parameters schema
func (t *MCPTool) Parameters() map[string]interface{} {
	if t.schema == nil {
		return map[string]interface{}{
			"type":       "object",
			"properties": map[string]interface{}{},
		}
	}
	return t.schema
}

// Execute executes the MCP tool
func (t *MCPTool) Execute(ctx context.Context, input map[string]interface{}) (*builtin.ToolResult, error) {
	if !t.client.IsConnected() {
		return nil, fmt.Errorf("MCP client not connected")
	}

	// Call the tool on the MCP server
	response, err := t.client.CallTool(t.toolName, input)
	if err != nil {
		return nil, fmt.Errorf("failed to call MCP tool %s: %w", t.toolName, err)
	}

	if response.IsError {
		return nil, fmt.Errorf("MCP tool %s returned error: %s", t.toolName, formatContent(response.Content))
	}

	return &builtin.ToolResult{
		Content: formatContent(response.Content),
		Data: map[string]interface{}{
			"mcp_tool": t.toolName,
			"server":   t.client.GetServerInfo(),
			"content":  formatContent(response.Content),
		},
	}, nil
}

// Validate validates the tool input
func (t *MCPTool) Validate(input map[string]interface{}) error {
	// Basic validation - could be enhanced with JSON schema validation
	if t.schema == nil {
		return nil
	}

	// Check for required properties
	if required, ok := t.schema["required"].([]interface{}); ok {
		for _, reqField := range required {
			if reqFieldStr, ok := reqField.(string); ok {
				if _, exists := input[reqFieldStr]; !exists {
					return fmt.Errorf("required field %s is missing", reqFieldStr)
				}
			}
		}
	}

	return nil
}

// formatContent formats MCP content for display
func formatContent(content []protocol.Content) string {
	if len(content) == 0 {
		return ""
	}

	var result strings.Builder
	for i, c := range content {
		if i > 0 {
			result.WriteString("\n")
		}

		switch c.Type {
		case "text":
			result.WriteString(c.Text)
		case "image":
			result.WriteString(fmt.Sprintf("[Image: %s]", c.MimeType))
		case "resource":
			result.WriteString(fmt.Sprintf("[Resource: %s]", c.Text))
		default:
			result.WriteString(c.Text)
		}
	}

	return result.String()
}

// MCPToolRegistry manages MCP tools within Alex's tool system
type MCPToolRegistry struct {
	clients map[string]*Client
	tools   map[string]*MCPTool
}

// NewMCPToolRegistry creates a new MCP tool registry
func NewMCPToolRegistry() *MCPToolRegistry {
	return &MCPToolRegistry{
		clients: make(map[string]*Client),
		tools:   make(map[string]*MCPTool),
	}
}

// RegisterClient registers an MCP client
func (r *MCPToolRegistry) RegisterClient(id string, client *Client) error {
	if !client.IsConnected() {
		return fmt.Errorf("MCP client %s is not connected", id)
	}

	r.clients[id] = client

	// Register all tools from this client
	tools := client.GetTools()

	for _, tool := range tools {
		mcpTool := NewMCPTool(client, tool)
		toolName := mcpTool.Name()
		r.tools[toolName] = mcpTool
	}

	if len(tools) > 0 {
		fmt.Printf("[INFO] MCP: Registered %d tools from server %s\n", len(tools), id)
	}

	return nil
}

// UnregisterClient unregisters an MCP client and its tools
func (r *MCPToolRegistry) UnregisterClient(id string) {
	client, exists := r.clients[id]
	if !exists {
		return
	}

	// Remove all tools from this client
	tools := client.GetTools()
	for _, tool := range tools {
		toolName := fmt.Sprintf("mcp_%s", tool.Name)
		delete(r.tools, toolName)
	}

	delete(r.clients, id)
}

// GetTool returns an MCP tool by name
func (r *MCPToolRegistry) GetTool(name string) (*MCPTool, bool) {
	tool, exists := r.tools[name]
	return tool, exists
}

// ListTools returns all MCP tools
func (r *MCPToolRegistry) ListTools() []*MCPTool {
	result := make([]*MCPTool, 0, len(r.tools))
	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// RefreshTools refreshes all tools from all clients
func (r *MCPToolRegistry) RefreshTools() error {
	for id, client := range r.clients {
		if err := client.RefreshTools(); err != nil {
			return fmt.Errorf("failed to refresh tools for client %s: %w", id, err)
		}

		// Re-register tools for this client
		tools := client.GetTools()
		for _, tool := range tools {
			mcpTool := NewMCPTool(client, tool)
			toolName := mcpTool.Name()
			r.tools[toolName] = mcpTool
		}
	}

	return nil
}

// IntegrateWithBuiltinTools integrates MCP tools with Alex's builtin tool list
func (r *MCPToolRegistry) IntegrateWithBuiltinTools(existingTools []builtin.Tool) []builtin.Tool {
	// Add all MCP tools to the builtin tool list
	result := make([]builtin.Tool, len(existingTools))
	copy(result, existingTools)

	for _, tool := range r.tools {
		result = append(result, tool)
	}
	return result
}

// MCPResource wraps MCP resources for Alex's consumption
type MCPResource struct {
	client   *Client
	resource protocol.Resource
}

// NewMCPResource creates a new MCP resource wrapper
func NewMCPResource(client *Client, resource protocol.Resource) *MCPResource {
	return &MCPResource{
		client:   client,
		resource: resource,
	}
}

// URI returns the resource URI
func (r *MCPResource) URI() string {
	return r.resource.URI
}

// Name returns the resource name
func (r *MCPResource) Name() string {
	return r.resource.Name
}

// Description returns the resource description
func (r *MCPResource) Description() string {
	return r.resource.Description
}

// MimeType returns the resource MIME type
func (r *MCPResource) MimeType() string {
	return r.resource.MimeType
}

// Read reads the resource content
func (r *MCPResource) Read(ctx context.Context) (string, error) {
	if !r.client.IsConnected() {
		return "", fmt.Errorf("MCP client not connected")
	}

	response, err := r.client.ReadResource(r.resource.URI)
	if err != nil {
		return "", fmt.Errorf("failed to read MCP resource %s: %w", r.resource.URI, err)
	}

	return formatContent(response.Contents), nil
}

// MCPPrompt wraps MCP prompts for Alex's consumption
type MCPPrompt struct {
	client *Client
	prompt protocol.Prompt
}

// NewMCPPrompt creates a new MCP prompt wrapper
func NewMCPPrompt(client *Client, prompt protocol.Prompt) *MCPPrompt {
	return &MCPPrompt{
		client: client,
		prompt: prompt,
	}
}

// Name returns the prompt name
func (p *MCPPrompt) Name() string {
	return p.prompt.Name
}

// Description returns the prompt description
func (p *MCPPrompt) Description() string {
	return p.prompt.Description
}

// Arguments returns the prompt arguments
func (p *MCPPrompt) Arguments() []protocol.PromptArgument {
	return p.prompt.Arguments
}

// Get gets the prompt with arguments
func (p *MCPPrompt) Get(ctx context.Context, arguments map[string]interface{}) ([]protocol.Message, error) {
	if !p.client.IsConnected() {
		return nil, fmt.Errorf("MCP client not connected")
	}

	response, err := p.client.GetPrompt(p.prompt.Name, arguments)
	if err != nil {
		return nil, fmt.Errorf("failed to get MCP prompt %s: %w", p.prompt.Name, err)
	}

	return response.Messages, nil
}

// MCPResourceRegistry manages MCP resources
type MCPResourceRegistry struct {
	clients   map[string]*Client
	resources map[string]*MCPResource
}

// NewMCPResourceRegistry creates a new MCP resource registry
func NewMCPResourceRegistry() *MCPResourceRegistry {
	return &MCPResourceRegistry{
		clients:   make(map[string]*Client),
		resources: make(map[string]*MCPResource),
	}
}

// RegisterClient registers an MCP client and its resources
func (r *MCPResourceRegistry) RegisterClient(id string, client *Client) error {
	if !client.IsConnected() {
		return fmt.Errorf("MCP client %s is not connected", id)
	}

	r.clients[id] = client

	// Register all resources from this client
	resources := client.GetResources()
	for _, resource := range resources {
		mcpResource := NewMCPResource(client, resource)
		r.resources[resource.URI] = mcpResource
	}

	return nil
}

// GetResource returns an MCP resource by URI
func (r *MCPResourceRegistry) GetResource(uri string) (*MCPResource, bool) {
	resource, exists := r.resources[uri]
	return resource, exists
}

// ListResources returns all MCP resources
func (r *MCPResourceRegistry) ListResources() []*MCPResource {
	result := make([]*MCPResource, 0, len(r.resources))
	for _, resource := range r.resources {
		result = append(result, resource)
	}
	return result
}
