package wechat

import "time"

// Config captures WeChat gateway behavior.
type Config struct {
	Enabled                bool
	LoginMode              string
	HotLogin               bool
	HotLoginStoragePath    string
	SessionPrefix          string
	ReplyPrefix            string
	MentionOnly            bool
	ReplyWithMention       bool
	AllowGroups            bool
	AllowDirect            bool
	AllowedConversationIDs []string
	AgentPreset            string
	ToolPreset             string
	ReplyTimeout           time.Duration
	MemoryEnabled          bool
}
