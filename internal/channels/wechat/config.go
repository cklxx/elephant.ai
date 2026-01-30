package wechat

import "alex/internal/channels"

// Config captures WeChat gateway behavior.
type Config struct {
	channels.BaseConfig    `yaml:",inline"`
	Enabled                bool
	LoginMode              string
	HotLogin               bool
	HotLoginStoragePath    string
	MentionOnly            bool
	ReplyWithMention       bool
	AllowedConversationIDs []string
}
