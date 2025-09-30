package ports

import "context"

// LLMClient represents any LLM provider
type LLMClient interface {
	// Complete sends messages and returns a response (non-streaming)
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Model returns the model identifier
	Model() string
}

// CompletionRequest contains all parameters for LLM completion
type CompletionRequest struct {
	Messages      []Message        `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Temperature   float64          `json:"temperature,omitempty"`
	MaxTokens     int              `json:"max_tokens,omitempty"`
	TopP          float64          `json:"top_p,omitempty"`
	StopSequences []string         `json:"stop,omitempty"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}

// CompletionResponse is the LLM's response
type CompletionResponse struct {
	Content    string         `json:"content"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	StopReason string         `json:"stop_reason"`
	Usage      TokenUsage     `json:"usage"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// TokenUsage tracks token consumption
type TokenUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// Message represents a conversation message
type Message struct {
	Role        string         `json:"role"`
	Content     string         `json:"content"`
	ToolCalls   []ToolCall     `json:"tool_calls,omitempty"`
	ToolResults []ToolResult   `json:"tool_results,omitempty"`
	Metadata    map[string]any `json:"metadata,omitempty"`
}
