package ports

import (
	"context"
	"time"
)

// LLMClient represents any LLM provider
type LLMClient interface {
	// Complete sends messages and returns a response (non-streaming)
	Complete(ctx context.Context, req CompletionRequest) (*CompletionResponse, error)

	// Model returns the model identifier
	Model() string
}

// StreamingLLMClient extends LLMClient with native streaming support.
// Implementations can surface incremental content while still returning the
// aggregated completion response when the stream ends.
type StreamingLLMClient interface {
	LLMClient

	// StreamComplete behaves like Complete but delivers incremental deltas
	// via the provided callbacks before returning the final response.
	StreamComplete(ctx context.Context, req CompletionRequest, callbacks CompletionStreamCallbacks) (*CompletionResponse, error)
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

// ContentDelta represents a streamed assistant content fragment.
type ContentDelta struct {
	Delta string
	Final bool
}

// CompletionStreamCallbacks captures optional hooks invoked while streaming an
// LLM response. All callbacks are optional; nil functions are ignored.
type CompletionStreamCallbacks struct {
	OnContentDelta func(ContentDelta)
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
	MessageSourceUserHistory    MessageSource = "user_history"
	MessageSourceAssistantReply MessageSource = "assistant_reply"
	MessageSourceToolResult     MessageSource = "tool_result"
	MessageSourceDebug          MessageSource = "debug"
	MessageSourceEvaluation     MessageSource = "evaluation"
	MessageSourceImportant      MessageSource = "important_notice"
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

// ImportantNote captures high-signal, user-personalized snippets that should
// survive context compression and be reattached to the conversation window on
// demand.
type ImportantNote struct {
	ID        string    `json:"id"`
	Content   string    `json:"content"`
	Source    string    `json:"source,omitempty"`
	Tags      []string  `json:"tags,omitempty"`
	CreatedAt time.Time `json:"created_at"`
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
	// Kind distinguishes short-lived attachments from long-lived artifacts.
	Kind string `json:"kind,omitempty"`
	// Format is a normalized representation of the content format (pptx, html, etc.).
	Format string `json:"format,omitempty"`
	// PreviewProfile hints at how clients should render complex artifacts.
	PreviewProfile string `json:"preview_profile,omitempty"`
	// PreviewAssets captures derived screenshots/pages for document-style artifacts.
	PreviewAssets []AttachmentPreviewAsset `json:"preview_assets,omitempty"`
	// RetentionTTLSeconds allows callers to override default cleanup windows.
	RetentionTTLSeconds uint64 `json:"retention_ttl_seconds,omitempty"`
}

// AttachmentPreviewAsset describes a derived preview asset for an attachment.
type AttachmentPreviewAsset struct {
	AssetID     string `json:"asset_id,omitempty"`
	Label       string `json:"label,omitempty"`
	MimeType    string `json:"mime_type,omitempty"`
	CDNURL      string `json:"cdn_url,omitempty"`
	PreviewType string `json:"preview_type,omitempty"`
}
