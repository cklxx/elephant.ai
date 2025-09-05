package protocol

import "encoding/json"

// MCP Protocol Messages

// InitializeRequest represents the initialize request
type InitializeRequest struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ClientCapabilities     `json:"capabilities"`
	ClientInfo      ClientInfo             `json:"clientInfo"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

// InitializeResponse represents the initialize response
type InitializeResponse struct {
	ProtocolVersion string                 `json:"protocolVersion"`
	Capabilities    ServerCapabilities     `json:"capabilities"`
	ServerInfo      ServerInfo             `json:"serverInfo"`
	Meta            map[string]interface{} `json:"meta,omitempty"`
}

// ClientCapabilities represents client capabilities
type ClientCapabilities struct {
	Sampling  *SamplingCapability  `json:"sampling,omitempty"`
	Roots     *RootsCapability     `json:"roots,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
}

// ServerCapabilities represents server capabilities
type ServerCapabilities struct {
	Prompts   *PromptsCapability   `json:"prompts,omitempty"`
	Resources *ResourcesCapability `json:"resources,omitempty"`
	Tools     *ToolsCapability     `json:"tools,omitempty"`
	Logging   *LoggingCapability   `json:"logging,omitempty"`
}

// SamplingCapability represents sampling capability
type SamplingCapability struct{}

// RootsCapability represents roots capability
type RootsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// LoggingCapability represents logging capability
type LoggingCapability struct{}

// PromptsCapability represents prompts capability
type PromptsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ResourcesCapability represents resources capability
type ResourcesCapability struct {
	Subscribe   bool `json:"subscribe,omitempty"`
	ListChanged bool `json:"listChanged,omitempty"`
}

// ToolsCapability represents tools capability
type ToolsCapability struct {
	ListChanged bool `json:"listChanged,omitempty"`
}

// ClientInfo represents client information
type ClientInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// ServerInfo represents server information
type ServerInfo struct {
	Name    string `json:"name"`
	Version string `json:"version"`
}

// PingRequest represents a ping request
type PingRequest struct{}

// PingResponse represents a ping response
type PingResponse struct{}

// Tool represents a tool definition
type Tool struct {
	Name        string          `json:"name"`
	Description string          `json:"description,omitempty"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// ToolsListRequest represents a tools list request
type ToolsListRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ToolsListResponse represents a tools list response
type ToolsListResponse struct {
	Tools      []Tool `json:"tools"`
	NextCursor string `json:"nextCursor,omitempty"`
}

// ToolsCallRequest represents a tools call request
type ToolsCallRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// ToolsCallResponse represents a tools call response
type ToolsCallResponse struct {
	Content []Content `json:"content"`
	IsError bool      `json:"isError,omitempty"`
}

// Content represents content in various formats
type Content struct {
	Type     string `json:"type"`
	Text     string `json:"text,omitempty"`
	Data     string `json:"data,omitempty"`
	MimeType string `json:"mimeType,omitempty"`
}

// Resource represents a resource definition
type Resource struct {
	URI         string `json:"uri"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	MimeType    string `json:"mimeType,omitempty"`
}

// ResourcesListRequest represents a resources list request
type ResourcesListRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// ResourcesListResponse represents a resources list response
type ResourcesListResponse struct {
	Resources  []Resource `json:"resources"`
	NextCursor string     `json:"nextCursor,omitempty"`
}

// ResourcesReadRequest represents a resources read request
type ResourcesReadRequest struct {
	URI string `json:"uri"`
}

// ResourcesReadResponse represents a resources read response
type ResourcesReadResponse struct {
	Contents []Content `json:"contents"`
}

// Prompt represents a prompt definition
type Prompt struct {
	Name        string                 `json:"name"`
	Description string                 `json:"description,omitempty"`
	Arguments   []PromptArgument       `json:"arguments,omitempty"`
	Meta        map[string]interface{} `json:"meta,omitempty"`
}

// PromptArgument represents a prompt argument
type PromptArgument struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
	Required    bool   `json:"required,omitempty"`
}

// PromptsListRequest represents a prompts list request
type PromptsListRequest struct {
	Cursor string `json:"cursor,omitempty"`
}

// PromptsListResponse represents a prompts list response
type PromptsListResponse struct {
	Prompts    []Prompt `json:"prompts"`
	NextCursor string   `json:"nextCursor,omitempty"`
}

// PromptsGetRequest represents a prompts get request
type PromptsGetRequest struct {
	Name      string                 `json:"name"`
	Arguments map[string]interface{} `json:"arguments,omitempty"`
}

// PromptsGetResponse represents a prompts get response
type PromptsGetResponse struct {
	Description string    `json:"description,omitempty"`
	Messages    []Message `json:"messages"`
}

// Message represents a message in the conversation
type Message struct {
	Role    string    `json:"role"`
	Content []Content `json:"content"`
}

// LoggingMessage represents a logging message
type LoggingMessage struct {
	Level  string                 `json:"level"`
	Data   interface{}            `json:"data"`
	Logger string                 `json:"logger,omitempty"`
	Meta   map[string]interface{} `json:"meta,omitempty"`
}

// RootsListRequest represents a roots list request
type RootsListRequest struct{}

// RootsListResponse represents a roots list response
type RootsListResponse struct {
	Roots []Root `json:"roots"`
}

// Root represents a root directory
type Root struct {
	URI  string `json:"uri"`
	Name string `json:"name,omitempty"`
}

// SamplingCreateMessageRequest represents a sampling create message request
type SamplingCreateMessageRequest struct {
	Messages         []Message              `json:"messages"`
	ModelPreferences *ModelPreferences      `json:"modelPreferences,omitempty"`
	SystemPrompt     string                 `json:"systemPrompt,omitempty"`
	IncludeContext   string                 `json:"includeContext,omitempty"`
	Temperature      float64                `json:"temperature,omitempty"`
	MaxTokens        int                    `json:"maxTokens,omitempty"`
	StopSequences    []string               `json:"stopSequences,omitempty"`
	Meta             map[string]interface{} `json:"meta,omitempty"`
}

// SamplingCreateMessageResponse represents a sampling create message response
type SamplingCreateMessageResponse struct {
	Role       string                 `json:"role"`
	Content    []Content              `json:"content"`
	Model      string                 `json:"model"`
	StopReason string                 `json:"stopReason,omitempty"`
	Meta       map[string]interface{} `json:"meta,omitempty"`
}

// ModelPreferences represents model preferences
type ModelPreferences struct {
	CostPriority         float64 `json:"costPriority,omitempty"`
	SpeedPriority        float64 `json:"speedPriority,omitempty"`
	IntelligencePriority float64 `json:"intelligencePriority,omitempty"`
}

// Progress represents progress information
type Progress struct {
	Progress int    `json:"progress"`
	Total    int    `json:"total,omitempty"`
	Token    string `json:"token,omitempty"`
}

// ProgressNotification represents a progress notification
type ProgressNotification struct {
	Progress int    `json:"progress"`
	Total    int    `json:"total,omitempty"`
	Token    string `json:"token,omitempty"`
}

// CancelledNotification represents a cancelled notification
type CancelledNotification struct {
	RequestID string `json:"requestId"`
	Reason    string `json:"reason,omitempty"`
}

// Common MCP method names
const (
	MethodInitialize            = "initialize"
	MethodPing                  = "ping"
	MethodToolsList             = "tools/list"
	MethodToolsCall             = "tools/call"
	MethodResourcesList         = "resources/list"
	MethodResourcesRead         = "resources/read"
	MethodResourcesUpdated      = "notifications/resources/updated"
	MethodResourcesListChanged  = "notifications/resources/list_changed"
	MethodPromptsList           = "prompts/list"
	MethodPromptsGet            = "prompts/get"
	MethodPromptsListChanged    = "notifications/prompts/list_changed"
	MethodRootsList             = "roots/list"
	MethodRootsListChanged      = "notifications/roots/list_changed"
	MethodSamplingCreateMessage = "sampling/createMessage"
	MethodLoggingMessage        = "notifications/message"
	MethodProgress              = "notifications/progress"
	MethodCancelled             = "notifications/cancelled"
)
