package ports

import "context"

// LLMClient represents any LLM provider
type LLMClient interface {
	// Complete sends messages and returns a response (non-streaming)
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Model returns the model identifier
	Model() string
}

// LLMClientFactory creates LLM clients for different providers
// This interface abstracts the concrete llm.Factory implementation
type LLMClientFactory interface {
	// GetClient creates or retrieves a cached LLM client
	GetClient(provider, model string, config LLMConfig) (LLMClient, error)

	// GetIsolatedClient creates a new non-cached client for session isolation
	GetIsolatedClient(provider, model string, config LLMConfig) (LLMClient, error)

	// DisableRetry disables retry logic for all clients created by this factory
	DisableRetry()
}

// LLMConfig contains configuration for LLM client creation
type LLMConfig struct {
	APIKey     string
	BaseURL    string
	Timeout    int
	MaxRetries int
	Headers    map[string]string
}

// UsageTrackingClient extends LLMClient with usage tracking
type UsageTrackingClient interface {
	LLMClient
	// SetUsageCallback sets a callback to be invoked after each API call
	SetUsageCallback(callback func(usage TokenUsage, model string, provider string))
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
type MessageSource string

const (
	MessageSourceUnknown        MessageSource = ""
	MessageSourceSystemPrompt   MessageSource = "system_prompt"
	MessageSourceUserInput      MessageSource = "user_input"
	MessageSourceAssistantReply MessageSource = "assistant_reply"
	MessageSourceToolResult     MessageSource = "tool_result"
	MessageSourceDebug          MessageSource = "debug"
	MessageSourceEvaluation     MessageSource = "evaluation"
)

type Message struct {
	Role        string                `json:"role"`
	Content     string                `json:"content"`
	ToolCalls   []ToolCall            `json:"tool_calls,omitempty"`
	ToolResults []ToolResult          `json:"tool_results,omitempty"`
	ToolCallID  string                `json:"tool_call_id,omitempty"`
	Metadata    map[string]any        `json:"metadata,omitempty"`
	Attachments map[string]Attachment `json:"attachments,omitempty"`
	Source      MessageSource         `json:"source,omitempty"`
}

// Attachment represents a binary asset (image, audio, etc.) referenced within a
// message or tool result using a placeholder such as `[filename.ext]`.
type Attachment struct {
	// Name is the canonical filename (without surrounding brackets) used in
	// the placeholder, e.g. `diagram.png` for `[diagram.png]`.
	Name string `json:"name"`
	// MediaType is the MIME type (e.g. image/png).
	MediaType string `json:"media_type"`
	// Data is a base64-encoded payload. It is optional when URI is
	// populated (for CDN hosted assets).
	Data string `json:"data,omitempty"`
	// URI is an absolute or data URI that can be used by clients to render
	// the attachment without additional decoding.
	URI string `json:"uri,omitempty"`
	// Source identifies where the attachment originated (tool name,
	// `user_upload`, etc.).
	Source string `json:"source,omitempty"`
	// Description provides optional human readable context about the
	// attachment contents.
	Description string `json:"description,omitempty"`
}
