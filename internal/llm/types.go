package llm

import (
	"context"
	"io"
	"time"
)

// ContextKeyType is a custom type for context keys to avoid collisions
type ContextKeyType string

// Message represents a chat message
type Message struct {
	Role       string     `json:"role"`
	Content    string     `json:"content,omitempty"`
	ToolCalls  []ToolCall `json:"tool_calls,omitempty"`
	ToolCallId string     `json:"tool_call_id,omitempty"`
	Name       string     `json:"name,omitempty"`

	// OpenAI reasoning fields (2025 Responses API)
	Reasoning        string `json:"reasoning,omitempty"`
	ReasoningSummary string `json:"reasoning_summary,omitempty"`
	Think            string `json:"think,omitempty"`

	// Metadata for tracking compression and other context
	Metadata map[string]interface{} `json:"metadata,omitempty"`

	// Compression tracking
	SourceMessages []Message `json:"source_messages,omitempty"`
	IsCompressed   bool      `json:"is_compressed,omitempty"`
}

// ChatRequest represents a request to the LLM
type ChatRequest struct {
	Messages    []Message              `json:"messages"`
	Model       string                 `json:"model,omitempty"`
	Temperature float64                `json:"temperature,omitempty"`
	MaxTokens   int                    `json:"max_tokens,omitempty"`
	Stream      bool                   `json:"stream,omitempty"`
	Provider    map[string]interface{} `json:"provider,omitempty"`
	// Tool calling support
	Tools      []Tool `json:"tools,omitempty"`
	ToolChoice string `json:"tool_choice,omitempty"`
	// Kimi API context caching support
	CacheControl interface{} `json:"cache_control,omitempty"`
	// Model type selection for multi-model configurations - not serialized to JSON
	ModelType ModelType `json:"-"`

	// Config for dynamic configuration resolution - not serialized to JSON
	Config *Config `json:"-"`
}

// ChatResponse represents a response from the LLM
// Supports both OpenAI and Gemini API response formats
type ChatResponse struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`

	// OpenAI format
	Usage Usage `json:"usage,omitempty"`

	// Gemini format - for compatibility
	UsageMetadata Usage `json:"usageMetadata,omitempty"`
}

// GetUsage returns the usage information, supporting both OpenAI and Gemini formats
func (r *ChatResponse) GetUsage() Usage {
	// Check if OpenAI format has any token data
	if r.Usage.GetTotalTokens() > 0 {
		return r.Usage
	}

	// Fall back to Gemini format
	if r.UsageMetadata.GetTotalTokens() > 0 {
		return r.UsageMetadata
	}

	// Return empty usage if neither format has data
	return Usage{}
}

// Choice represents a choice in the response
type Choice struct {
	Index        int     `json:"index"`
	Message      Message `json:"message,omitempty"`
	Delta        Message `json:"delta,omitempty"`
	FinishReason string  `json:"finish_reason,omitempty"`
}

// Usage represents token usage information
// Supports both OpenAI and Gemini API response formats
type Usage struct {
	// OpenAI format (snake_case)
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`

	// Gemini format (camelCase) - for compatibility
	PromptTokenCount     int `json:"promptTokenCount"`
	CandidatesTokenCount int `json:"candidatesTokenCount"`
	TotalTokenCount      int `json:"totalTokenCount"`
}

// GetPromptTokens returns the prompt tokens count, supporting both API formats
func (u *Usage) GetPromptTokens() int {
	if u.PromptTokens > 0 {
		return u.PromptTokens
	}
	return u.PromptTokenCount
}

// GetCompletionTokens returns the completion tokens count, supporting both API formats
func (u *Usage) GetCompletionTokens() int {
	if u.CompletionTokens > 0 {
		return u.CompletionTokens
	}
	return u.CandidatesTokenCount
}

// GetTotalTokens returns the total tokens count, supporting both API formats
func (u *Usage) GetTotalTokens() int {
	if u.TotalTokens > 0 {
		return u.TotalTokens
	}
	return u.TotalTokenCount
}

// StreamDelta represents a streaming response chunk
// Supports both OpenAI and other provider streaming formats
type StreamDelta struct {
	ID      string   `json:"id"`
	Object  string   `json:"object"`
	Created int64    `json:"created"`
	Model   string   `json:"model"`
	Choices []Choice `json:"choices"`

	// Usage information in streaming responses (when available)
	Usage Usage `json:"usage,omitempty"`

	// Additional provider-specific fields
	Provider string `json:"provider,omitempty"`
}

// GetUsage returns the usage information from streaming delta
func (d *StreamDelta) GetUsage() Usage {
	return d.Usage
}

// ModelType represents different model usage types
type ModelType string

const (
	BasicModel     ModelType = "basic"     // For general tasks, fast responses
	ReasoningModel ModelType = "reasoning" // For complex reasoning, tool calls, high-quality content
)

// ModelConfig represents configuration for a specific model
type ModelConfig struct {
	BaseURL     string  `json:"base_url"`
	Model       string  `json:"model"`
	APIKey      string  `json:"api_key"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// Config represents LLM client configuration with multi-model support
type Config struct {
	// Default single model config (backward compatibility)
	APIKey      string        `json:"api_key,omitempty"`
	BaseURL     string        `json:"base_url,omitempty"`
	Model       string        `json:"model,omitempty"`
	Temperature float64       `json:"temperature,omitempty"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Timeout     time.Duration `json:"timeout,omitempty"`

	// Multi-model configurations
	Models map[ModelType]*ModelConfig `json:"models,omitempty"`

	// Default model type to use when none specified
	DefaultModelType ModelType `json:"default_model_type,omitempty"`
}

// Client interface defines LLM client operations
type Client interface {
	// Chat sends a chat request and returns the response
	Chat(ctx context.Context, req *ChatRequest, sessionID string) (*ChatResponse, error)

	// ChatStream sends a chat request and returns a streaming response
	ChatStream(ctx context.Context, req *ChatRequest, sessionID string) (<-chan StreamDelta, error)

	// Close closes the client and cleans up resources
	Close() error
}

// StreamReader interface for reading streaming responses
type StreamReader interface {
	io.Reader
	io.Closer
}

// ToolCall represents an OpenAI-standard tool call
type ToolCall struct {
	ID       string   `json:"id"`
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// Function represents a function definition or call
type Function struct {
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Parameters  interface{} `json:"parameters,omitempty"`
	Arguments   string      `json:"arguments,omitempty"`
}

// Tool represents a tool definition
type Tool struct {
	Type     string   `json:"type"`
	Function Function `json:"function"`
}

// NewCompressedMessage creates a new compressed message with source history
func NewCompressedMessage(role, content string, sourceMessages []Message) *Message {
	return &Message{
		Role:           role,
		Content:        content,
		IsCompressed:   true,
		SourceMessages: sourceMessages,
		Metadata: map[string]interface{}{
			"compression_time": time.Now().Format(time.RFC3339),
			"source_count":     len(sourceMessages),
		},
	}
}

// HasSourceMessages checks if this message has compression history
func (m *Message) HasSourceMessages() bool {
	return m.IsCompressed && len(m.SourceMessages) > 0
}

// ExpandCompression expands a compressed message back to its source messages
func (m *Message) ExpandCompression() []Message {
	if !m.HasSourceMessages() {
		return []Message{*m}
	}
	return m.SourceMessages
}
