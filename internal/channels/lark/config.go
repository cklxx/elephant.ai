package lark

import "time"

// Config captures Lark gateway behavior.
type Config struct {
	Enabled       bool
	AppID         string
	AppSecret     string
	BaseDomain    string
	SessionPrefix string
	ReplyPrefix   string
	AllowGroups   bool
	AllowDirect   bool
	AgentPreset   string
	ToolPreset    string
	ReplyTimeout  time.Duration
	ReactEmoji    string // Emoji reaction sent immediately on message receipt (e.g. "SMILE"). Empty disables.
}
