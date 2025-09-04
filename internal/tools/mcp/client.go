package mcp

import (
	"context"
	"encoding/json"
	"fmt"
	"reflect"
	"sync"
	"time"

	"alex/internal/tools/mcp/protocol"
)

// parseJSONResponse efficiently parses a JSON-RPC response result using reflection
// This avoids the inefficient double JSON marshal/unmarshal pattern
func parseJSONResponse(result interface{}, target interface{}) error {
	if result == nil {
		return fmt.Errorf("response result is nil")
	}
	
	// Get the target value and ensure it's a pointer
	targetValue := reflect.ValueOf(target)
	if targetValue.Kind() != reflect.Ptr {
		return fmt.Errorf("target must be a pointer")
	}
	
	targetElem := targetValue.Elem()
	if !targetElem.CanSet() {
		return fmt.Errorf("target cannot be set")
	}
	
	// Try direct type assertion first for common cases
	resultValue := reflect.ValueOf(result)
	
	// If result is already the correct type, assign directly
	if resultValue.Type().AssignableTo(targetElem.Type()) {
		targetElem.Set(resultValue)
		return nil
	}
	
	// If result is a map[string]interface{}, try JSON unmarshaling as fallback
	// This handles the common JSON-RPC case where result is a generic map
	if resultMap, ok := result.(map[string]interface{}); ok {
		resultBytes, err := json.Marshal(resultMap)
		if err != nil {
			return fmt.Errorf("failed to marshal response result: %w", err)
		}
		if err := json.Unmarshal(resultBytes, target); err != nil {
			return fmt.Errorf("failed to parse response: %w", err)
		}
		return nil
	}
	
	// For other types, try JSON marshaling as fallback (original behavior)
	resultBytes, err := json.Marshal(result)
	if err != nil {
		return fmt.Errorf("failed to marshal response result: %w", err)
	}
	if err := json.Unmarshal(resultBytes, target); err != nil {
		return fmt.Errorf("failed to parse response: %w", err)
	}
	return nil
}

// Transport interface for MCP communication
type Transport interface {
	Connect(ctx context.Context) error
	Disconnect() error
	SendRequest(req *protocol.JSONRPCRequest) (*protocol.JSONRPCResponse, error)
	SendNotification(notification *protocol.JSONRPCNotification) error
	ReceiveMessages() <-chan []byte
	ReceiveErrors() <-chan error
	IsConnected() bool
	NextRequestID() int64
}

// Client represents an MCP client
type Client struct {
	transport     Transport
	serverInfo    *protocol.ServerInfo
	capabilities  *protocol.ServerCapabilities
	tools         []protocol.Tool
	resources     []protocol.Resource
	prompts       []protocol.Prompt
	mu            sync.RWMutex
	ctx           context.Context
	cancel        context.CancelFunc
	initialized   bool
	messageHandler func([]byte)
	errorHandler   func(error)
}

// ClientConfig represents configuration for an MCP client
type ClientConfig struct {
	ClientInfo   protocol.ClientInfo
	Capabilities protocol.ClientCapabilities
	Transport    Transport
	Timeout      time.Duration
}

// NewClient creates a new MCP client
func NewClient(config *ClientConfig) *Client {
	if config.Timeout == 0 {
		config.Timeout = 30 * time.Second
	}

	return &Client{
		transport: config.Transport,
	}
}

// Connect connects to the MCP server and initializes the session
func (c *Client) Connect(ctx context.Context, config *ClientConfig) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if c.initialized {
		return nil
	}

	c.ctx, c.cancel = context.WithCancel(ctx)

	// Connect transport
	if err := c.transport.Connect(c.ctx); err != nil {
		return fmt.Errorf("failed to connect transport: %w", err)
	}

	// Start message handling
	go c.handleMessages()
	go c.handleErrors()

	// Initialize the session
	if err := c.initialize(config); err != nil {
		_ = c.transport.Disconnect()
		return fmt.Errorf("failed to initialize session: %w", err)
	}

	// Set initialized flag after successful initialization
	c.initialized = true
	return nil
}

// Disconnect disconnects from the MCP server
func (c *Client) Disconnect() error {
	c.mu.Lock()
	defer c.mu.Unlock()

	if !c.initialized {
		return nil
	}

	if c.cancel != nil {
		c.cancel()
	}

	err := c.transport.Disconnect()
	c.initialized = false
	return err
}

// initialize performs the MCP initialization handshake
func (c *Client) initialize(config *ClientConfig) error {
	// Send initialize request
	request := &protocol.InitializeRequest{
		ProtocolVersion: "2024-11-05",
		Capabilities:    config.Capabilities,
		ClientInfo:      config.ClientInfo,
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodInitialize,
		request,
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return fmt.Errorf("failed to send initialize request: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("initialize failed: %s", resp.Error.Message)
	}

	// Parse initialize response
	var initResp protocol.InitializeResponse
	if err := parseJSONResponse(resp.Result, &initResp); err != nil {
		return fmt.Errorf("failed to parse initialize response: %w", err)
	}

	c.serverInfo = &initResp.ServerInfo
	c.capabilities = &initResp.Capabilities

	// Set initialized flag immediately after successful initialize response
	c.initialized = true

	// Load initial data
	if err := c.loadInitialData(); err != nil {
		// Don't fail completely if initial data loading fails
		fmt.Printf("[WARN] MCP: Failed to load initial data from %s: %v\n", c.serverInfo.Name, err)
	}

	return nil
}

// loadInitialData loads tools, resources, and prompts from the server
func (c *Client) loadInitialData() error {
	// Load tools
	if c.capabilities.Tools != nil {
		tools, err := c.ListTools()
		if err != nil {
			return fmt.Errorf("failed to load tools: %w", err)
		}
		c.tools = tools
	}

	// Load resources
	if c.capabilities.Resources != nil {
		resources, err := c.ListResources()
		if err != nil {
			return fmt.Errorf("failed to load resources: %w", err)
		}
		c.resources = resources
	}

	// Load prompts
	if c.capabilities.Prompts != nil {
		prompts, err := c.ListPrompts()
		if err != nil {
			return fmt.Errorf("failed to load prompts: %w", err)
		}
		c.prompts = prompts
	}

	return nil
}

// handleMessages handles incoming messages from the server
func (c *Client) handleMessages() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case message := <-c.transport.ReceiveMessages():
			if c.messageHandler != nil {
				c.messageHandler(message)
			}
		}
	}
}

// handleErrors handles errors from the transport
func (c *Client) handleErrors() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case err := <-c.transport.ReceiveErrors():
			if c.errorHandler != nil {
				c.errorHandler(err)
			}
		}
	}
}

// SetMessageHandler sets a custom message handler
func (c *Client) SetMessageHandler(handler func([]byte)) {
	c.messageHandler = handler
}

// SetErrorHandler sets a custom error handler
func (c *Client) SetErrorHandler(handler func(error)) {
	c.errorHandler = handler
}

// Ping sends a ping request to the server
func (c *Client) Ping() error {
	if !c.initialized {
		return fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodPing,
		&protocol.PingRequest{},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return fmt.Errorf("failed to send ping: %w", err)
	}

	if resp.Error != nil {
		return fmt.Errorf("ping failed: %s", resp.Error.Message)
	}

	return nil
}

// ListTools returns available tools from the server
func (c *Client) ListTools() ([]protocol.Tool, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodToolsList,
		&protocol.ToolsListRequest{},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list tools: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list tools failed: %s", resp.Error.Message)
	}

	var toolsResp protocol.ToolsListResponse
	if err := parseJSONResponse(resp.Result, &toolsResp); err != nil {
		return nil, fmt.Errorf("failed to parse tools response: %w", err)
	}

	return toolsResp.Tools, nil
}

// CallTool calls a tool on the server
func (c *Client) CallTool(name string, arguments map[string]interface{}) (*protocol.ToolsCallResponse, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodToolsCall,
		&protocol.ToolsCallRequest{
			Name:      name,
			Arguments: arguments,
		},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to call tool: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("tool call failed: %s", resp.Error.Message)
	}

	var toolResp protocol.ToolsCallResponse
	if err := parseJSONResponse(resp.Result, &toolResp); err != nil {
		return nil, fmt.Errorf("failed to parse tool response: %w", err)
	}

	return &toolResp, nil
}

// ListResources returns available resources from the server
func (c *Client) ListResources() ([]protocol.Resource, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodResourcesList,
		&protocol.ResourcesListRequest{},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list resources: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list resources failed: %s", resp.Error.Message)
	}

	var resourcesResp protocol.ResourcesListResponse
	if err := parseJSONResponse(resp.Result, &resourcesResp); err != nil {
		return nil, fmt.Errorf("failed to parse resources response: %w", err)
	}

	return resourcesResp.Resources, nil
}

// ReadResource reads a resource from the server
func (c *Client) ReadResource(uri string) (*protocol.ResourcesReadResponse, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodResourcesRead,
		&protocol.ResourcesReadRequest{
			URI: uri,
		},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to read resource: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("read resource failed: %s", resp.Error.Message)
	}

	var resourceResp protocol.ResourcesReadResponse
	if err := parseJSONResponse(resp.Result, &resourceResp); err != nil {
		return nil, fmt.Errorf("failed to parse resource response: %w", err)
	}

	return &resourceResp, nil
}

// ListPrompts returns available prompts from the server
func (c *Client) ListPrompts() ([]protocol.Prompt, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodPromptsList,
		&protocol.PromptsListRequest{},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to list prompts: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("list prompts failed: %s", resp.Error.Message)
	}

	var promptsResp protocol.PromptsListResponse
	if err := parseJSONResponse(resp.Result, &promptsResp); err != nil {
		return nil, fmt.Errorf("failed to parse prompts response: %w", err)
	}

	return promptsResp.Prompts, nil
}

// GetPrompt gets a prompt from the server
func (c *Client) GetPrompt(name string, arguments map[string]interface{}) (*protocol.PromptsGetResponse, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	req := protocol.NewRequest(
		c.transport.NextRequestID(),
		protocol.MethodPromptsGet,
		&protocol.PromptsGetRequest{
			Name:      name,
			Arguments: arguments,
		},
	)

	resp, err := c.transport.SendRequest(req)
	if err != nil {
		return nil, fmt.Errorf("failed to get prompt: %w", err)
	}

	if resp.Error != nil {
		return nil, fmt.Errorf("get prompt failed: %s", resp.Error.Message)
	}

	var promptResp protocol.PromptsGetResponse
	if err := parseJSONResponse(resp.Result, &promptResp); err != nil {
		return nil, fmt.Errorf("failed to parse prompt response: %w", err)
	}

	return &promptResp, nil
}

// GetServerInfo returns server information
func (c *Client) GetServerInfo() *protocol.ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// GetCapabilities returns server capabilities
func (c *Client) GetCapabilities() *protocol.ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities
}

// GetTools returns cached tools
func (c *Client) GetTools() []protocol.Tool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]protocol.Tool, len(c.tools))
	copy(result, c.tools)
	return result
}

// GetResources returns cached resources
func (c *Client) GetResources() []protocol.Resource {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]protocol.Resource, len(c.resources))
	copy(result, c.resources)
	return result
}

// GetPrompts returns cached prompts
func (c *Client) GetPrompts() []protocol.Prompt {
	c.mu.RLock()
	defer c.mu.RUnlock()
	result := make([]protocol.Prompt, len(c.prompts))
	copy(result, c.prompts)
	return result
}

// IsConnected returns connection status
func (c *Client) IsConnected() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized && c.transport.IsConnected()
}

// RefreshTools refreshes the cached tools from the server
func (c *Client) RefreshTools() error {
	tools, err := c.ListTools()
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.tools = tools
	c.mu.Unlock()

	return nil
}

// RefreshResources refreshes the cached resources from the server
func (c *Client) RefreshResources() error {
	resources, err := c.ListResources()
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.resources = resources
	c.mu.Unlock()

	return nil
}

// RefreshPrompts refreshes the cached prompts from the server
func (c *Client) RefreshPrompts() error {
	prompts, err := c.ListPrompts()
	if err != nil {
		return err
	}

	c.mu.Lock()
	c.prompts = prompts
	c.mu.Unlock()

	return nil
}