package channel

// ChannelMessage represents an inbound message from a channel.
// Mirrors Eli's message structure.
type ChannelMessage struct {
	SessionID string         `json:"session_id"`
	Channel   string         `json:"channel"`
	Content   string         `json:"content"`
	ChatID    string         `json:"chat_id,omitempty"`
	Kind      string         `json:"kind,omitempty"` // "text", "command", "event"
	Context   map[string]any `json:"context,omitempty"`
	Media     []MediaItem    `json:"media,omitempty"`
	UserID    string         `json:"user_id,omitempty"`
	Timestamp int64          `json:"timestamp,omitempty"`
}
