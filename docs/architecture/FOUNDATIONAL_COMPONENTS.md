# Foundational Components Specification

**Version**: 1.0
**Date**: 2025-09-30
**Parent Document**: [REFACTORING_PROPOSAL.md](./REFACTORING_PROPOSAL.md)

---

## Overview

This document provides detailed specifications for all foundational components (infrastructure layer) in the refactored ALEX architecture. These components are reusable, pluggable, and implement interfaces defined in the domain layer.

---

## Table of Contents

1. [LLM Client](#1-llm-client)
2. [Tool System](#2-tool-system)
3. [MCP Protocol](#3-mcp-protocol)
4. [Context Manager](#4-context-manager)
5. [Session Store](#5-session-store)
6. [Message Channel](#6-message-channel)
7. [Function Call Parser](#7-function-call-parser)

---

## 1. LLM Client

**Package**: `internal/llm/`
**Interface Location**: `internal/agent/ports/llm.go`

### 1.1 Interface Definition

```go
package ports

import "context"

// LLMClient represents any LLM provider
type LLMClient interface {
    // Complete sends messages and returns a response
    Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

    // Stream sends messages and streams response chunks
    Stream(ctx context.Context, req CompletionRequest) (ResponseStream, error)

    // Model returns the model identifier
    Model() string
}

// CompletionRequest contains all parameters for LLM completion
type CompletionRequest struct {
    Messages     []Message          // Conversation history
    Tools        []ToolDefinition   // Available tools (function calling)
    Temperature  float64            // Sampling temperature (0.0-2.0)
    MaxTokens    int                // Maximum tokens to generate
    TopP         float64            // Nucleus sampling parameter
    StopSequences []string          // Stop generation at these sequences
    Stream       bool               // Whether to stream response
    Metadata     map[string]any     // Provider-specific options
}

// CompletionResponse is the LLM's response
type CompletionResponse struct {
    Content    string              // Text content
    ToolCalls  []ToolCall          // Requested tool calls
    StopReason string              // Why generation stopped (stop, length, tool_use)
    Usage      TokenUsage          // Token consumption
    Metadata   map[string]any      // Provider-specific data
}

// ResponseStream allows reading streamed chunks
type ResponseStream interface {
    // Next returns the next chunk, or io.EOF when done
    Next() (*StreamChunk, error)

    // Close terminates the stream
    Close() error
}

// StreamChunk represents one chunk of streamed response
type StreamChunk struct {
    Delta      string              // Text delta
    ToolCall   *ToolCall           // Tool call (if any)
    Done       bool                // Whether stream is complete
    Usage      *TokenUsage         // Token usage (final chunk only)
}

// TokenUsage tracks token consumption
type TokenUsage struct {
    PromptTokens     int
    CompletionTokens int
    TotalTokens      int
}
```

### 1.2 Factory Pattern

```go
package llm

import "sync"

// Factory creates and caches LLM clients
type Factory struct {
    cache map[string]LLMClient
    mu    sync.RWMutex
}

func NewFactory() *Factory {
    return &Factory{
        cache: make(map[string]LLMClient),
    }
}

// GetClient returns or creates a client for the specified provider and model
func (f *Factory) GetClient(provider, model string, config Config) (ports.LLMClient, error) {
    cacheKey := fmt.Sprintf("%s:%s", provider, model)

    // Check cache first
    f.mu.RLock()
    if client, ok := f.cache[cacheKey]; ok {
        f.mu.RUnlock()
        return client, nil
    }
    f.mu.RUnlock()

    // Create new client
    var client ports.LLMClient
    var err error

    switch provider {
    case "openai":
        client, err = openai.NewClient(model, config)
    case "deepseek":
        client, err = deepseek.NewClient(model, config)
    case "ollama":
        client, err = ollama.NewClient(model, config)
    default:
        return nil, fmt.Errorf("unknown provider: %s", provider)
    }

    if err != nil {
        return nil, err
    }

    // Cache for reuse
    f.mu.Lock()
    f.cache[cacheKey] = client
    f.mu.Unlock()

    return client, nil
}

// Config holds provider configuration
type Config struct {
    APIKey    string
    BaseURL   string
    Timeout   time.Duration
    MaxRetries int
    Headers   map[string]string
}
```

### 1.3 Provider Implementations

#### OpenAI/OpenRouter Client

**Package**: `internal/llm/openai/`

```go
package openai

import (
    "context"
    "github.com/sashabaranov/go-openai"
)

type client struct {
    underlying *openai.Client
    model      string
}

func NewClient(model string, config Config) (ports.LLMClient, error) {
    cfg := openai.DefaultConfig(config.APIKey)
    if config.BaseURL != "" {
        cfg.BaseURL = config.BaseURL
    }

    return &client{
        underlying: openai.NewClientWithConfig(cfg),
        model:      model,
    }, nil
}

func (c *client) Complete(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
    // Convert ports.CompletionRequest to openai.ChatCompletionRequest
    oaiReq := c.convertRequest(req)

    resp, err := c.underlying.CreateChatCompletion(ctx, oaiReq)
    if err != nil {
        return nil, err
    }

    // Convert openai.ChatCompletionResponse to ports.CompletionResponse
    return c.convertResponse(resp), nil
}

func (c *client) Stream(ctx context.Context, req ports.CompletionRequest) (ports.ResponseStream, error) {
    oaiReq := c.convertRequest(req)
    oaiReq.Stream = true

    stream, err := c.underlying.CreateChatCompletionStream(ctx, oaiReq)
    if err != nil {
        return nil, err
    }

    return &responseStream{stream: stream}, nil
}

func (c *client) Model() string {
    return c.model
}
```

#### DeepSeek Client

**Package**: `internal/llm/deepseek/`

Similar structure to OpenAI client, uses DeepSeek API endpoints.

#### Ollama Client

**Package**: `internal/llm/ollama/`

Connects to local Ollama server for running models locally.

### 1.4 Testing

```go
// internal/llm/factory_test.go
func TestFactory_GetClient(t *testing.T) {
    factory := NewFactory()

    client, err := factory.GetClient("openai", "gpt-4", Config{APIKey: "test"})
    assert.NoError(t, err)
    assert.NotNil(t, client)

    // Should return cached client
    client2, err := factory.GetClient("openai", "gpt-4", Config{APIKey: "test"})
    assert.NoError(t, err)
    assert.Same(t, client, client2)
}
```

---

## 2. Tool System

**Package**: `internal/tools/`
**Interface Location**: `internal/agent/ports/tools.go`

### 2.1 Interface Definition

```go
package ports

// ToolExecutor executes a single tool call
type ToolExecutor interface {
    // Execute runs the tool with given arguments
    Execute(ctx context.Context, call ToolCall) (*ToolResult, error)

    // Definition returns the tool's schema for LLM
    Definition() ToolDefinition

    // Metadata returns tool metadata
    Metadata() ToolMetadata
}

// ToolRegistry manages available tools
type ToolRegistry interface {
    // Register adds a tool to the registry
    Register(tool ToolExecutor) error

    // Get retrieves a tool by name
    Get(name string) (ToolExecutor, error)

    // List returns all available tools
    List() []ToolDefinition

    // Unregister removes a tool
    Unregister(name string) error
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
    ID        string            // Unique call identifier
    Name      string            // Tool name
    Arguments map[string]any    // Tool arguments
}

// ToolResult is the execution result
type ToolResult struct {
    CallID   string            // Matches ToolCall.ID
    Content  string            // Text result
    Error    error             // Execution error (if any)
    Metadata map[string]any    // Additional result data
}

// ToolDefinition describes a tool for the LLM
type ToolDefinition struct {
    Name        string
    Description string
    Parameters  ParameterSchema
}

// ToolMetadata contains tool information
type ToolMetadata struct {
    Name        string
    Version     string
    Category    string
    Tags        []string
    Dangerous   bool  // Requires confirmation
}

// ParameterSchema defines tool parameters (JSON Schema format)
type ParameterSchema struct {
    Type       string                 `json:"type"`
    Properties map[string]Property    `json:"properties"`
    Required   []string               `json:"required,omitempty"`
}

type Property struct {
    Type        string `json:"type"`
    Description string `json:"description"`
    Enum        []any  `json:"enum,omitempty"`
}
```

### 2.2 Registry Implementation

```go
package tools

import "sync"

// registry implements ToolRegistry with three-tier caching
type registry struct {
    static   map[string]ports.ToolExecutor  // Built-in tools
    dynamic  map[string]ports.ToolExecutor  // Runtime registered
    mcp      map[string]ports.ToolExecutor  // MCP protocol tools

    mu sync.RWMutex
}

func NewRegistry() ports.ToolRegistry {
    r := &registry{
        static:  make(map[string]ports.ToolExecutor),
        dynamic: make(map[string]ports.ToolExecutor),
        mcp:     make(map[string]ports.ToolExecutor),
    }

    // Register built-in tools
    r.registerBuiltins()

    return r
}

func (r *registry) Register(tool ports.ToolExecutor) error {
    r.mu.Lock()
    defer r.mu.Unlock()

    name := tool.Metadata().Name
    if _, exists := r.static[name]; exists {
        return fmt.Errorf("tool already exists: %s", name)
    }

    r.dynamic[name] = tool
    return nil
}

func (r *registry) Get(name string) (ports.ToolExecutor, error) {
    r.mu.RLock()
    defer r.mu.RUnlock()

    // Check static first (most common)
    if tool, ok := r.static[name]; ok {
        return tool, nil
    }

    // Check dynamic
    if tool, ok := r.dynamic[name]; ok {
        return tool, nil
    }

    // Check MCP
    if tool, ok := r.mcp[name]; ok {
        return tool, nil
    }

    return nil, fmt.Errorf("tool not found: %s", name)
}

func (r *registry) List() []ports.ToolDefinition {
    r.mu.RLock()
    defer r.mu.RUnlock()

    var defs []ports.ToolDefinition

    for _, tool := range r.static {
        defs = append(defs, tool.Definition())
    }
    for _, tool := range r.dynamic {
        defs = append(defs, tool.Definition())
    }
    for _, tool := range r.mcp {
        defs = append(defs, tool.Definition())
    }

    return defs
}

func (r *registry) registerBuiltins() {
    // Register all built-in tools
    r.static["file_read"] = builtin.NewFileRead()
    r.static["file_update"] = builtin.NewFileUpdate()
    r.static["bash"] = builtin.NewBash()
    r.static["grep"] = builtin.NewGrep()
    r.static["todo_read"] = builtin.NewTodoRead()
    r.static["todo_update"] = builtin.NewTodoUpdate()
    // ... more builtin tools
}
```

### 2.3 Example Built-in Tool

```go
// internal/tools/builtin/file_read.go
package builtin

type fileRead struct{}

func NewFileRead() ports.ToolExecutor {
    return &fileRead{}
}

func (t *fileRead) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
    // Extract path argument
    path, ok := call.Arguments["path"].(string)
    if !ok {
        return &ports.ToolResult{
            CallID: call.ID,
            Error:  fmt.Errorf("missing or invalid 'path' argument"),
        }, nil
    }

    // Validate path
    if err := validatePath(path); err != nil {
        return &ports.ToolResult{
            CallID: call.ID,
            Error:  err,
        }, nil
    }

    // Read file
    content, err := os.ReadFile(path)
    if err != nil {
        return &ports.ToolResult{
            CallID: call.ID,
            Error:  fmt.Errorf("failed to read file: %w", err),
        }, nil
    }

    return &ports.ToolResult{
        CallID:  call.ID,
        Content: string(content),
    }, nil
}

func (t *fileRead) Definition() ports.ToolDefinition {
    return ports.ToolDefinition{
        Name:        "file_read",
        Description: "Read the contents of a file",
        Parameters: ports.ParameterSchema{
            Type: "object",
            Properties: map[string]ports.Property{
                "path": {
                    Type:        "string",
                    Description: "Path to the file to read",
                },
            },
            Required: []string{"path"},
        },
    }
}

func (t *fileRead) Metadata() ports.ToolMetadata {
    return ports.ToolMetadata{
        Name:      "file_read",
        Version:   "1.0.0",
        Category:  "file_operations",
        Tags:      []string{"file", "read", "io"},
        Dangerous: false,
    }
}
```

---

## 3. MCP Protocol

**Package**: `internal/mcp/`
**Interface Location**: `internal/agent/ports/mcp.go`

### 3.1 Interface Definition

```go
package ports

// MCPServer represents a connection to an MCP server
type MCPServer interface {
    // Tools returns available tools from this server
    Tools() ([]ToolDefinition, error)

    // CallTool executes a tool on the server
    CallTool(ctx context.Context, name string, args map[string]any) (any, error)

    // Resources returns available resources
    Resources() ([]ResourceDefinition, error)

    // ReadResource fetches a resource
    ReadResource(ctx context.Context, uri string) (string, error)

    // Close terminates the connection
    Close() error

    // Health checks if server is alive
    Health() error
}

// MCPManager manages MCP server lifecycle
type MCPManager interface {
    // StartServer launches a new MCP server
    StartServer(config ServerConfig) (MCPServer, error)

    // GetServer retrieves an existing server
    GetServer(name string) (MCPServer, error)

    // StopServer terminates a server
    StopServer(name string) error

    // ListServers returns all running servers
    ListServers() []string
}
```

### 3.2 Manager Implementation

```go
package mcp

type manager struct {
    servers   map[string]ports.MCPServer
    transport TransportFactory
    mu        sync.RWMutex
}

func NewManager() ports.MCPManager {
    return &manager{
        servers:   make(map[string]ports.MCPServer),
        transport: newTransportFactory(),
    }
}

func (m *manager) StartServer(config ServerConfig) (ports.MCPServer, error) {
    m.mu.Lock()
    defer m.mu.Unlock()

    if _, exists := m.servers[config.Name]; exists {
        return nil, fmt.Errorf("server already running: %s", config.Name)
    }

    // Create transport based on config
    var transport Transport
    var err error

    switch config.Type {
    case "stdio":
        transport, err = m.transport.CreateSTDIO(config.Command, config.Args)
    case "sse":
        transport, err = m.transport.CreateSSE(config.URL)
    default:
        return nil, fmt.Errorf("unknown transport type: %s", config.Type)
    }

    if err != nil {
        return nil, err
    }

    // Create server with protocol handler
    server := &mcpServer{
        name:      config.Name,
        transport: transport,
        protocol:  protocol.NewHandler(transport),
    }

    // Initialize server
    if err := server.initialize(); err != nil {
        transport.Close()
        return nil, err
    }

    m.servers[config.Name] = server
    return server, nil
}
```

### 3.3 Protocol Layer

```go
// internal/mcp/protocol/handler.go
package protocol

// Handler implements JSON-RPC 2.0 protocol
type Handler struct {
    transport Transport
    nextID    uint64
    pending   map[uint64]chan *Response
    mu        sync.Mutex
}

func NewHandler(transport Transport) *Handler {
    h := &Handler{
        transport: transport,
        pending:   make(map[uint64]chan *Response),
    }

    // Start reading responses
    go h.readLoop()

    return h
}

func (h *Handler) Call(method string, params any) (*Response, error) {
    id := atomic.AddUint64(&h.nextID, 1)

    req := &Request{
        JSONRPC: "2.0",
        ID:      id,
        Method:  method,
        Params:  params,
    }

    // Register response channel
    respChan := make(chan *Response, 1)
    h.mu.Lock()
    h.pending[id] = respChan
    h.mu.Unlock()

    // Send request
    if err := h.transport.Send(req); err != nil {
        h.mu.Lock()
        delete(h.pending, id)
        h.mu.Unlock()
        return nil, err
    }

    // Wait for response
    resp := <-respChan
    return resp, nil
}
```

### 3.4 Transport Layer

```go
// internal/mcp/transport/stdio.go
package transport

type stdioTransport struct {
    cmd    *exec.Cmd
    stdin  io.WriteCloser
    stdout io.ReadCloser
    stderr io.ReadCloser
}

func NewSTDIO(command string, args []string) (Transport, error) {
    cmd := exec.Command(command, args...)

    stdin, err := cmd.StdinPipe()
    if err != nil {
        return nil, err
    }

    stdout, err := cmd.StdoutPipe()
    if err != nil {
        return nil, err
    }

    stderr, err := cmd.StderrPipe()
    if err != nil {
        return nil, err
    }

    if err := cmd.Start(); err != nil {
        return nil, err
    }

    return &stdioTransport{
        cmd:    cmd,
        stdin:  stdin,
        stdout: stdout,
        stderr: stderr,
    }, nil
}

func (t *stdioTransport) Send(msg any) error {
    data, err := json.Marshal(msg)
    if err != nil {
        return err
    }

    _, err = t.stdin.Write(append(data, '\n'))
    return err
}

func (t *stdioTransport) Receive() (any, error) {
    scanner := bufio.NewScanner(t.stdout)
    if !scanner.Scan() {
        return nil, io.EOF
    }

    var msg map[string]any
    if err := json.Unmarshal(scanner.Bytes(), &msg); err != nil {
        return nil, err
    }

    return msg, nil
}
```

---

## 4. Context Manager

**Package**: `internal/context/`
**Interface Location**: `internal/agent/ports/context.go`

### 4.1 Interface Definition

```go
package ports

// ContextManager handles token limits and compression
type ContextManager interface {
    // EstimateTokens estimates token count for messages
    EstimateTokens(messages []Message) int

    // Compress reduces message size when limit approached
    Compress(messages []Message, targetTokens int) ([]Message, error)

    // ShouldCompress checks if compression needed
    ShouldCompress(messages []Message, limit int) bool
}
```

### 4.2 Implementation

```go
package context

type manager struct {
    tokenCounter TokenCounter
    compressor   MessageCompressor
    threshold    float64  // Compress at 80% of limit
}

func NewManager() ports.ContextManager {
    return &manager{
        tokenCounter: newTikTokenCounter(),
        compressor:   newSmartCompressor(),
        threshold:    0.8,
    }
}

func (m *manager) EstimateTokens(messages []ports.Message) int {
    return m.tokenCounter.Count(messages)
}

func (m *manager) ShouldCompress(messages []ports.Message, limit int) bool {
    tokenCount := m.EstimateTokens(messages)
    return float64(tokenCount) > float64(limit)*m.threshold
}

func (m *manager) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
    currentTokens := m.EstimateTokens(messages)
    if currentTokens <= targetTokens {
        return messages, nil
    }

    // Strategy: Keep system prompts + recent messages, summarize middle
    return m.compressor.Compress(messages, targetTokens)
}
```

### 4.3 Compression Strategies

```go
// internal/context/compressor.go
package context

type smartCompressor struct {
    summarizer LLMSummarizer
}

func (c *smartCompressor) Compress(messages []ports.Message, targetTokens int) ([]ports.Message, error) {
    // Step 1: Separate system prompts, recent messages, and middle messages
    systemMessages := c.filterSystemMessages(messages)
    recentMessages := c.getRecentMessages(messages, 10)
    middleMessages := c.getMiddleMessages(messages, len(systemMessages), len(messages)-10)

    // Step 2: Always keep system prompts and recent messages
    result := append([]ports.Message{}, systemMessages...)

    // Step 3: Summarize middle messages if they exist
    if len(middleMessages) > 0 {
        summary, err := c.summarizer.Summarize(middleMessages)
        if err != nil {
            // Fallback: just trim oldest
            return c.trimOldest(messages, targetTokens), nil
        }

        result = append(result, ports.Message{
            Role:    "system",
            Content: fmt.Sprintf("[Conversation summary: %s]", summary),
        })
    }

    // Step 4: Add recent messages
    result = append(result, recentMessages...)

    return result, nil
}
```

---

## 5. Session Store

**Package**: `internal/session/`
**Interface Location**: `internal/agent/ports/session.go`

### 5.1 Interface Definition

```go
package ports

// SessionStore persists agent sessions
type SessionStore interface {
    // Create creates a new session
    Create(ctx context.Context) (*Session, error)

    // Get retrieves a session by ID
    Get(ctx context.Context, id string) (*Session, error)

    // Save persists session state
    Save(ctx context.Context, session *Session) error

    // List returns all session IDs
    List(ctx context.Context) ([]string, error)

    // Delete removes a session
    Delete(ctx context.Context, id string) error
}

// Session represents an agent session
type Session struct {
    ID        string
    Messages  []Message
    Todos     []Todo
    Metadata  map[string]string
    CreatedAt time.Time
    UpdatedAt time.Time
}
```

### 5.2 File Store Implementation

```go
// internal/session/filestore/store.go
package filestore

type store struct {
    baseDir string
}

func New(baseDir string) ports.SessionStore {
    // Expand home directory if needed
    if strings.HasPrefix(baseDir, "~/") {
        home, _ := os.UserHomeDir()
        baseDir = filepath.Join(home, baseDir[2:])
    }

    // Create directory if not exists
    os.MkdirAll(baseDir, 0755)

    return &store{baseDir: baseDir}
}

func (s *store) Create(ctx context.Context) (*ports.Session, error) {
    session := &ports.Session{
        ID:        generateID(),
        Messages:  []ports.Message{},
        Todos:     []ports.Todo{},
        Metadata:  make(map[string]string),
        CreatedAt: time.Now(),
        UpdatedAt: time.Now(),
    }

    if err := s.Save(ctx, session); err != nil {
        return nil, err
    }

    return session, nil
}

func (s *store) Get(ctx context.Context, id string) (*ports.Session, error) {
    path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", id))

    data, err := os.ReadFile(path)
    if err != nil {
        return nil, fmt.Errorf("session not found: %s", id)
    }

    var session ports.Session
    if err := json.Unmarshal(data, &session); err != nil {
        return nil, fmt.Errorf("invalid session data: %w", err)
    }

    return &session, nil
}

func (s *store) Save(ctx context.Context, session *ports.Session) error {
    session.UpdatedAt = time.Now()

    data, err := json.MarshalIndent(session, "", "  ")
    if err != nil {
        return err
    }

    path := filepath.Join(s.baseDir, fmt.Sprintf("%s.json", session.ID))
    return os.WriteFile(path, data, 0644)
}
```

### 5.3 Memory Store (Testing)

```go
// internal/session/memstore/store.go
package memstore

type store struct {
    sessions map[string]*ports.Session
    mu       sync.RWMutex
}

func New() ports.SessionStore {
    return &store{
        sessions: make(map[string]*ports.Session),
    }
}

// Implementation is similar but uses in-memory map instead of files
```

---

## 6. Message Channel (REMOVED)

**Status**: Removed in cleanup (2025-10-03)

**Rationale**: The MessageQueue abstraction was never used in the production codebase. ALEX uses a direct synchronous execution model where tasks are processed immediately through `AgentCoordinator.ExecuteTask()`. There is no need for async message queuing.

**Architecture Decision**: Tasks are processed directly through the coordinator:
- CLI calls `ExecuteTask()` synchronously with streaming output via EventListener
- Server uses SSE (Server-Sent Events) for streaming with `EventBroadcaster`
- No message queue needed - YAGNI principle applied

**Migration Notes**: If async task processing is needed in the future, consider:
1. Worker pool pattern at the server layer (not domain)
2. Channel-based task distribution for parallel processing
3. Message broker integration (RabbitMQ, Redis) for distributed scenarios

For historical reference, see git history before 2025-10-03.

---

## 7. Function Call Parser

**Package**: `internal/parser/`
**Interface Location**: `internal/agent/ports/parser.go`

### 7.1 Interface Definition

```go
package ports

// FunctionCallParser extracts tool calls from LLM responses
type FunctionCallParser interface {
    // Parse extracts tool calls from content
    Parse(content string) ([]ToolCall, error)

    // Validate checks if tool calls are valid
    Validate(call ToolCall, definition ToolDefinition) error
}
```

### 7.2 Implementation

```go
package parser

type parser struct {
    validators map[string]Validator
}

func New() ports.FunctionCallParser {
    return &parser{
        validators: defaultValidators(),
    }
}

func (p *parser) Parse(content string) ([]ports.ToolCall, error) {
    var calls []ports.ToolCall

    // Try multiple formats:

    // 1. Native function calling (already in ToolCalls field)
    // (handled by LLM client, not here)

    // 2. XML-style tool calls
    xmlCalls := p.parseXMLStyle(content)
    calls = append(calls, xmlCalls...)

    // 3. JSON tool calls
    jsonCalls := p.parseJSONStyle(content)
    calls = append(calls, jsonCalls...)

    return calls, nil
}

func (p *parser) parseXMLStyle(content string) []ports.ToolCall {
    // Match <tool_call>{"name": "...", "args": {...}}</tool_call>
    re := regexp.MustCompile(`<tool_call>(.*?)</tool_call>`)
    matches := re.FindAllStringSubmatch(content, -1)

    var calls []ports.ToolCall
    for _, match := range matches {
        var call struct {
            Name string         `json:"name"`
            Args map[string]any `json:"args"`
        }

        if err := json.Unmarshal([]byte(match[1]), &call); err != nil {
            continue
        }

        calls = append(calls, ports.ToolCall{
            ID:        generateID(),
            Name:      call.Name,
            Arguments: call.Args,
        })
    }

    return calls
}

func (p *parser) Validate(call ports.ToolCall, definition ports.ToolDefinition) error {
    // Check required parameters
    for _, required := range definition.Parameters.Required {
        if _, ok := call.Arguments[required]; !ok {
            return fmt.Errorf("missing required parameter: %s", required)
        }
    }

    // Validate parameter types
    for name, value := range call.Arguments {
        prop, ok := definition.Parameters.Properties[name]
        if !ok {
            return fmt.Errorf("unknown parameter: %s", name)
        }

        if err := p.validateType(value, prop); err != nil {
            return fmt.Errorf("invalid parameter %s: %w", name, err)
        }
    }

    return nil
}
```

---

## Summary

These foundational components provide:

- ✅ **Clean interfaces** defined in domain layer (`internal/agent/ports/`)
- ✅ **Pluggable implementations** in infrastructure layer
- ✅ **Easy testing** with mock implementations
- ✅ **Provider independence** (swap LLM providers, storage backends, etc.)
- ✅ **Reusability** across different business logic
- ✅ **Clear boundaries** between components

Each component can be developed, tested, and optimized independently without affecting others.

---

**Next Document**: [BUSINESS_LOGIC.md](./BUSINESS_LOGIC.md) - Domain and application layer specifications