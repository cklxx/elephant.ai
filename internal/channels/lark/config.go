package lark

import "time"

// Config captures Lark gateway behavior.
type Config struct {
	Enabled       bool
	AppID         string
	AppSecret     string
	BaseDomain    string
	SessionPrefix string
	SessionMode   string // "stable" (default) reuses chat session; "fresh" creates a new session per message.
	ReplyPrefix   string
	AllowGroups   bool
	AllowDirect   bool
	AgentPreset   string
	ToolPreset    string
	ReplyTimeout  time.Duration
	ReactEmoji    string // Emoji reaction sent immediately on message receipt (e.g. "SMILE"). Empty disables.
	MemoryEnabled       bool // Enable automatic memory save/recall per session.
	ShowToolProgress    bool // Show real-time tool progress in chat. Default false.
	AutoChatContext     bool // Automatically fetch recent chat messages as context. Default false.
	AutoChatContextSize int  // Number of recent messages to fetch for auto chat context. Default 20.
}
