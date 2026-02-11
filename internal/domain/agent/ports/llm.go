package ports

import "time"

// ThinkingConfig configures model-side reasoning output when supported.
type ThinkingConfig struct {
	Enabled      bool   `json:"enabled,omitempty"`
	Effort       string `json:"effort,omitempty"`
	Summary      string `json:"summary,omitempty"`
	BudgetTokens int    `json:"budget_tokens,omitempty"`
}

// CompletionRequest contains all parameters for LLM completion
type CompletionRequest struct {
	Messages      []Message        `json:"messages"`
	Tools         []ToolDefinition `json:"tools,omitempty"`
	Temperature   float64          `json:"temperature,omitempty"`
	MaxTokens     int              `json:"max_tokens,omitempty"`
	TopP          float64          `json:"top_p,omitempty"`
	StopSequences []string         `json:"stop,omitempty"`
	Thinking      ThinkingConfig   `json:"thinking,omitempty"`
	Metadata      map[string]any   `json:"metadata,omitempty"`
}

// CompletionResponse is the LLM's response
type CompletionResponse struct {
	Content    string         `json:"content"`
	Thinking   Thinking       `json:"thinking,omitempty"`
	ToolCalls  []ToolCall     `json:"tool_calls,omitempty"`
	StopReason string         `json:"stop_reason"`
	Usage      TokenUsage     `json:"usage"`
	Metadata   map[string]any `json:"metadata,omitempty"`
}

// Thinking captures model-generated reasoning content across providers.
type Thinking struct {
	Parts []ThinkingPart `json:"parts,omitempty"`
}

// ThinkingPart stores a single reasoning segment with optional metadata.
type ThinkingPart struct {
	Kind       string `json:"kind,omitempty"`
	Text       string `json:"text,omitempty"`
	Encrypted  string `json:"encrypted,omitempty"`
	Signature  string `json:"signature,omitempty"`
	ProviderID string `json:"provider_id,omitempty"`
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
	MessageSourceProactive      MessageSource = "proactive_context"
	MessageSourceCheckpoint     MessageSource = "checkpoint"
)

type Message struct {
	Role        string                `json:"role"`
	Content     string                `json:"content"`
	Thinking    Thinking              `json:"thinking,omitempty"`
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
	// Fingerprint is a stable hash of the attachment payload used to
	// disambiguate same-name replacements across iterations.
	Fingerprint string `json:"fingerprint,omitempty"`
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
