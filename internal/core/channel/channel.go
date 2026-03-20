package channel

import "context"

// Channel is the interface for message delivery channels (Lark, CLI, HTTP/SSE).
type Channel interface {
	// Name returns the channel identifier (e.g., "lark", "cli", "http").
	Name() string

	// Start initializes the channel.
	Start(ctx context.Context) error

	// Stop gracefully shuts down the channel.
	Stop(ctx context.Context) error

	// Send delivers an outbound message to a specific session.
	Send(ctx context.Context, sessionID string, msg Outbound) error

	// NeedsDebounce returns whether this channel requires message debouncing.
	NeedsDebounce() bool
}

// Outbound is a message ready for channel delivery.
type Outbound struct {
	Content  string         `json:"content"`
	Media    []MediaItem    `json:"media,omitempty"`
	Metadata map[string]any `json:"metadata,omitempty"`
	Kind     string         `json:"kind,omitempty"` // "text", "card", "image"
}

// MediaItem is a media attachment.
type MediaItem struct {
	Type string `json:"type"` // "image", "file", "audio"
	URL  string `json:"url,omitempty"`
	Data []byte `json:"data,omitempty"`
	Name string `json:"name,omitempty"`
}
