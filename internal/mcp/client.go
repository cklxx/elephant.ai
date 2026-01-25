package mcp

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"alex/internal/async"
	"alex/internal/logging"
)

// MCP Protocol version
const MCPProtocolVersion = "2024-11-05"

// Client implements MCP client over stdio transport
type Client struct {
	serverName   string
	process      *ProcessManager
	idGen        *RequestIDGenerator
	pendingCalls map[any]chan *Response
	mu           sync.RWMutex
	logger       logging.Logger
	initialized  bool
	serverInfo   *ServerInfo
	capabilities *ServerCapabilities
}

// ClientInfo represents the client information sent during initialize
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfo represents the server information received during initialize
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerCapabilities represents what the server supports
type ServerCapabilities struct {
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
}

// ToolsCapability indicates the server supports tools
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability indicates the server supports resources
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// PromptsCapability indicates the server supports prompts
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// InitializeResult is the result of the initialize handshake
type InitializeResult struct {
	ProtocolVersion string             `json:"protocolVersion"`
	ServerInfo      ServerInfo         `json:"serverInfo"`
	Capabilities    ServerCapabilities `json:"capabilities"`
}

// ToolSchema represents an MCP tool definition
type ToolSchema struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description"`
	InputSchema map[string]interface{} `json:"inputSchema"`
}

// ToolCallParams are the parameters for calling a tool
type ToolCallParams struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolCallResult is the result of calling a tool
type ToolCallResult struct {
	Content []ContentBlock `json:"content"`
	IsError bool           `json:"isError,omitempty"`
}

// ContentBlock represents a piece of content in the result
type ContentBlock struct {
	Type     string `json:"type"` // "text", "image", "resource"
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// NewClient creates a new MCP client
func NewClient(serverName string, process *ProcessManager) *Client {
	return &Client{
		serverName:   serverName,
		process:      process,
		idGen:        NewRequestIDGenerator(),
		pendingCalls: make(map[any]chan *Response),
		logger:       logging.NewComponentLogger(fmt.Sprintf("MCPClient[%s]", serverName)),
	}
}

// Start starts the client and initializes connection with the server
func (c *Client) Start(ctx context.Context) error {
	c.logger.Info("Starting MCP client for server: %s", c.serverName)

	// Start the server process
	if err := c.process.Start(ctx); err != nil {
		return fmt.Errorf("failed to start server process: %w", err)
	}

	// Start reading responses in background
	async.Go(c.logger, "mcp.client.readLoop", func() {
		c.readLoop()
	})

	// Perform initialize handshake
	if err := c.initialize(ctx); err != nil {
		_ = c.process.Stop(5 * time.Second) // Best effort cleanup
		return fmt.Errorf("initialize handshake failed: %w", err)
	}

	c.logger.Info("MCP client initialized successfully")
	return nil
}

// Stop stops the client and server process
func (c *Client) Stop() error {
	c.logger.Info("Stopping MCP client")
	return c.process.Stop(5 * time.Second)
}

// Initialize performs the MCP initialize handshake
func (c *Client) initialize(ctx context.Context) error {
	c.logger.Debug("Sending initialize request")

	params := map[string]any{
		"protocolVersion": MCPProtocolVersion,
		"clientInfo": ClientInfo{
			Name:    "alex",
			Version: "0.1.0",
		},
	}

	result, err := c.call(ctx, "initialize", params)
	if err != nil {
		return fmt.Errorf("initialize call failed: %w", err)
	}

	// Parse initialize result
	var initResult InitializeResult
	if err := unmarshalResult(result, &initResult); err != nil {
		return fmt.Errorf("failed to parse initialize result: %w", err)
	}

	// Validate protocol version
	if initResult.ProtocolVersion != MCPProtocolVersion {
		c.logger.Warn("Protocol version mismatch: client=%s, server=%s",
			MCPProtocolVersion, initResult.ProtocolVersion)
	}

	c.serverInfo = &initResult.ServerInfo
	c.capabilities = &initResult.Capabilities
	c.initialized = true

	c.logger.Info("Initialized with server: %s v%s", initResult.ServerInfo.Name, initResult.ServerInfo.Version)
	c.logger.Debug("Server capabilities: tools=%v, resources=%v, prompts=%v",
		initResult.Capabilities.Tools != nil,
		initResult.Capabilities.Resources != nil,
		initResult.Capabilities.Prompts != nil)

	// Send initialized notification
	if err := c.notify(ctx, "notifications/initialized", nil); err != nil {
		c.logger.Warn("Failed to send initialized notification: %v", err)
	}

	return nil
}

// ListTools retrieves all available tools from the server
func (c *Client) ListTools(ctx context.Context) ([]ToolSchema, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	c.logger.Debug("Listing tools from server")

	result, err := c.call(ctx, "tools/list", nil)
	if err != nil {
		return nil, fmt.Errorf("tools/list call failed: %w", err)
	}

	// Parse tools list result
	var response struct {
		Tools []ToolSchema `json:"tools"`
	}
	if err := unmarshalResult(result, &response); err != nil {
		return nil, fmt.Errorf("failed to parse tools list: %w", err)
	}

	c.logger.Info("Retrieved %d tools from server", len(response.Tools))
	return response.Tools, nil
}

// CallTool executes a tool on the MCP server
func (c *Client) CallTool(ctx context.Context, name string, arguments map[string]interface{}) (*ToolCallResult, error) {
	if !c.initialized {
		return nil, fmt.Errorf("client not initialized")
	}

	c.logger.Debug("Calling tool: %s", name)

	params := map[string]any{
		"name":      name,
		"arguments": arguments,
	}

	result, err := c.call(ctx, "tools/call", params)
	if err != nil {
		return nil, fmt.Errorf("tools/call failed: %w", err)
	}

	// Parse tool call result
	var toolResult ToolCallResult
	if err := unmarshalResult(result, &toolResult); err != nil {
		return nil, fmt.Errorf("failed to parse tool result: %w", err)
	}

	c.logger.Debug("Tool %s executed successfully, content blocks: %d", name, len(toolResult.Content))
	return &toolResult, nil
}

// call sends a JSON-RPC request and waits for the response
func (c *Client) call(ctx context.Context, method string, params map[string]any) (any, error) {
	// Generate request ID
	id := c.idGen.Next()

	// Create request
	req := NewRequest(id, method, params)

	// Marshal request to JSON
	data, err := Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	// Add newline delimiter for stdio transport
	data = append(data, '\n')

	// Create response channel
	respChan := make(chan *Response, 1)
	c.mu.Lock()
	c.pendingCalls[id] = respChan
	c.mu.Unlock()

	// Clean up on exit
	defer func() {
		c.mu.Lock()
		delete(c.pendingCalls, id)
		c.mu.Unlock()
	}()

	// Send request
	c.logger.Debug("Sending request: method=%s, id=%v", method, id)
	if err := c.process.Write(data); err != nil {
		return nil, fmt.Errorf("failed to write request: %w", err)
	}

	// Wait for response with timeout
	select {
	case resp := <-respChan:
		if resp.IsError() {
			return nil, resp.Error
		}
		return resp.Result, nil
	case <-ctx.Done():
		return nil, fmt.Errorf("request cancelled: %w", ctx.Err())
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("request timeout after 30s")
	}
}

// notify sends a JSON-RPC notification (no response expected)
func (c *Client) notify(ctx context.Context, method string, params map[string]any) error {
	notif := NewNotification(method, params)

	data, err := Marshal(notif)
	if err != nil {
		return fmt.Errorf("failed to marshal notification: %w", err)
	}

	// Add newline delimiter
	data = append(data, '\n')

	c.logger.Debug("Sending notification: method=%s", method)
	return c.process.Write(data)
}

// readLoop continuously reads responses from the server
func (c *Client) readLoop() {
	c.logger.Debug("Starting read loop")
	scanner := bufio.NewScanner(c.process.GetStdout())

	// Increase buffer size for large responses
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024) // 1MB max

	for scanner.Scan() {
		line := scanner.Bytes()
		c.logger.Debug("Received response: %d bytes", len(line))

		// Parse response
		resp, err := UnmarshalResponse(line)
		if err != nil {
			c.logger.Error("Failed to unmarshal response: %v", err)
			continue
		}

		// Route response to waiting caller
		c.mu.RLock()
		ch, ok := c.pendingCalls[resp.ID]
		c.mu.RUnlock()

		if ok {
			select {
			case ch <- resp:
				c.logger.Debug("Routed response to caller: id=%v", resp.ID)
			default:
				c.logger.Warn("Response channel full, dropping response: id=%v", resp.ID)
			}
		} else {
			c.logger.Warn("No pending call found for response: id=%v", resp.ID)
		}
	}

	if err := scanner.Err(); err != nil {
		c.logger.Error("Read loop error: %v", err)
	}
	c.logger.Debug("Read loop exited")
}

// IsInitialized checks if the client has completed initialization
func (c *Client) IsInitialized() bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.initialized
}

// GetServerInfo returns information about the connected server
func (c *Client) GetServerInfo() *ServerInfo {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.serverInfo
}

// GetCapabilities returns the server's capabilities
func (c *Client) GetCapabilities() *ServerCapabilities {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.capabilities
}

// unmarshalResult is a helper to unmarshal the result field
func unmarshalResult(result any, target any) error {
	// Convert result to JSON and back to parse into target struct
	// This handles the any type safely
	data, err := Marshal(result)
	if err != nil {
		return err
	}

	// Unmarshal into target
	return json.Unmarshal(data, target)
}
