package lark

import (
	"time"

	"alex/internal/channels"
)

// Config captures Lark gateway behavior.
type Config struct {
	channels.BaseConfig           `yaml:",inline"`
	Enabled                       bool
	AppID                         string
	AppSecret                     string
	BaseDomain                    string
	WorkspaceDir                  string
	CardsEnabled                  bool
	CardsPlanReview               bool
	CardsResults                  bool
	CardsErrors                   bool
	CardCallbackVerificationToken string
	CardCallbackEncryptKey        string
	AutoUploadFiles               bool
	AutoUploadMaxBytes            int
	AutoUploadAllowExt            []string
	Browser                       BrowserConfig
	ReactEmoji                    string // Random emoji pool for start/end reactions (comma/space separated).
	ShowToolProgress              bool   // Show real-time tool progress in chat. Default false.
	ShowPlanClarifyMessages       bool   // Send plan/clarify tool outputs as chat messages. Default false.
	AutoChatContextSize           int    // Number of recent messages to fetch for auto chat context. Default 20.
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
}

// BrowserConfig captures local browser settings for Lark tool execution.
type BrowserConfig struct {
	CDPURL      string
	ChromePath  string
	Headless    bool
	UserDataDir string
	Timeout     time.Duration
}
