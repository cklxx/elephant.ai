package lark

import (
	"time"

	"alex/internal/delivery/channels"
)

// Config captures Lark gateway behavior.
type Config struct {
	channels.BaseConfig           `yaml:",inline"`
	Enabled                       bool
	AppID                         string
	AppSecret                     string
	TenantCalendarID              string
	BaseDomain                    string
	WorkspaceDir                  string
	AutoUploadFiles               bool
	AutoUploadMaxBytes            int
	AutoUploadAllowExt            []string
	Browser                       BrowserConfig
	ReactEmoji                    string // Random emoji pool for start/end reactions (comma/space separated).
	InjectionAckReactEmoji        string // Emoji reaction for injected user messages while a task is running. Default THINKING.
	FinalAnswerReviewReactEmoji   string // Emoji reaction when final_answer_review triggers. Default GLANCE.
	ShowToolProgress              bool   // Show real-time tool progress in chat. Default false.
	ShowPlanClarifyMessages       bool   // Send plan/clarify tool outputs as chat messages. Default false.
	AutoChatContextSize           int    // Number of recent messages to fetch for auto chat context. Default 20.
	BackgroundProgressEnabled     *bool  // Push background task progress updates. Default true.
	BackgroundProgressInterval    time.Duration
	BackgroundProgressWindow      time.Duration
	PlanReviewEnabled             bool
	PlanReviewRequireConfirmation bool
	PlanReviewPendingTTL          time.Duration
	ActiveSlotTTL                 time.Duration // Expire idle in-memory session slots.
	ActiveSlotMaxEntries          int           // Hard cap for activeSlots map size.
	PendingInputRelayTTL          time.Duration // Expire stale external input relay requests.
	PendingInputRelayMaxChats     int           // Hard cap for pendingInputRelays map size.
	PendingInputRelayMaxPerChat   int           // Hard cap per chat pending relay queue.
	AIChatSessionTTL              time.Duration // Expire inactive AI chat coordination sessions.
	StateCleanupInterval          time.Duration // Sweeper interval for in-memory Lark runtime state.
	// Task management configuration.
	PersistenceMode            string        // "file" or "memory". Default "file".
	PersistenceDir             string        // Base dir used by file persistence.
	PersistenceRetention       time.Duration // Expiration horizon for terminal task cleanup.
	PersistenceMaxTasksPerChat int           // Max retained tasks per chat in persistence store.
	MaxConcurrentTasks         int           // Max concurrent tasks per chat. Default 3.
	DefaultPlanMode            PlanMode      // Global default plan mode strategy. Default "auto".
	// AIChatBotIDs is a list of bot IDs that participate in coordinated multi-bot chats.
	// When multiple bots from this list are mentioned in a group message, they will
	// take turns responding instead of all responding simultaneously.
	AIChatBotIDs []string
	// CCHooksAutoConfig enables automatic Claude Code hooks configuration
	// (direct file write to .claude/settings.local.json) after /notice bind.
	CCHooksAutoConfig *CCHooksAutoConfig
}

// CCHooksAutoConfig holds parameters for automatic Claude Code hooks setup.
type CCHooksAutoConfig struct {
	ServerURL string
	Token     string
}

// BrowserConfig captures local browser settings for Lark tool execution.
type BrowserConfig struct {
	CDPURL      string
	ChromePath  string
	Headless    bool
	UserDataDir string
	Timeout     time.Duration
}
