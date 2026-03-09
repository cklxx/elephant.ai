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
	ProcessingReactEmoji          string // Emoji reaction while task is running. Removed on completion. Default "OnIt".
	InjectionAckReactEmoji        string // Emoji reaction for injected user messages while a task is running. Default THINKING.
	ShowToolProgress              bool   // Show real-time tool progress in chat. Default false.
	SlowProgressSummaryEnabled    *bool  // Emit periodic progress summaries when foreground task exceeds delay. Default true.
	SlowProgressSummaryDelay      time.Duration
	ShowPlanClarifyMessages       bool  // Send plan/clarify tool outputs as chat messages. Default false.
	ToolFailureAbortThreshold     int   // Abort foreground run after N consecutive tool failures. Default 6.
	AutoChatContextSize           int   // Number of recent messages to fetch for auto chat context. Default 20.
	BackgroundProgressEnabled     *bool // Push background task progress updates. Default true.
	RephraseEnabled               *bool // LLM rephrase of background task results for readability. Default true.
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
	PersistenceMode                 string        // "file" or "memory". Default "file".
	PersistenceDir                  string        // Base dir used by file persistence.
	PersistenceRetention            time.Duration // Expiration horizon for terminal task cleanup.
	PersistenceMaxTasksPerChat      int           // Max retained tasks per chat in persistence store.
	MaxConcurrentTasks              int           // Max concurrent tasks per chat. Default 3.
	TeamCompletionSummaryEnabled    *bool         // Send summary when all background tasks finish. Default true.
	TeamCompletionSummaryLLMTimeout time.Duration // LLM timeout for team summary generation. Default 10s.
	DefaultPlanMode                 PlanMode      // Global default plan mode strategy. Default "auto".
	DeliveryMode                    string        // Terminal delivery strategy: direct|shadow|outbox.
	DeliveryWorker                  DeliveryWorkerConfig
	// AIChatBotIDs is a list of bot IDs that participate in coordinated multi-bot chats.
	// When multiple bots from this list are mentioned in a group message, they will
	// take turns responding instead of all responding simultaneously.
	AIChatBotIDs []string
	// CCHooksAutoConfig enables automatic Claude Code hooks configuration
	// (direct file write to .claude/settings.local.json) after /notice bind.
	CCHooksAutoConfig *CCHooksAutoConfig
}

const (
	defaultDeliveryWorkerPollInterval = 500 * time.Millisecond
	defaultDeliveryWorkerBatchSize    = 50
	defaultDeliveryWorkerMaxAttempts  = 8
	defaultDeliveryWorkerBaseBackoff  = 500 * time.Millisecond
	defaultDeliveryWorkerMaxBackoff   = 60 * time.Second
	defaultDeliveryWorkerJitterRatio  = 0.2
)

// DeliveryWorkerConfig controls async outbox processing.
type DeliveryWorkerConfig struct {
	Enabled      bool
	PollInterval time.Duration
	BatchSize    int
	MaxAttempts  int
	BaseBackoff  time.Duration
	MaxBackoff   time.Duration
	JitterRatio  float64
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
