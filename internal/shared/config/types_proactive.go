package config

// ProactiveConfig captures proactive behavior defaults.
type ProactiveConfig struct {
	Enabled   bool               `json:"enabled" yaml:"enabled"`
	Prompt    PromptConfig       `json:"prompt" yaml:"prompt"`
	Memory    MemoryConfig       `json:"memory" yaml:"memory"`
	Skills    SkillsConfig       `json:"skills" yaml:"skills"`
	OKR       OKRProactiveConfig `json:"okr" yaml:"okr"`
	Scheduler SchedulerConfig    `json:"scheduler" yaml:"scheduler"`
	Timer     TimerConfig        `json:"timer" yaml:"timer"`
	Attention AttentionConfig    `json:"attention" yaml:"attention"`
}

// PromptConfig controls system-prompt assembly behavior.
type PromptConfig struct {
	Mode              string   `json:"mode" yaml:"mode"` // full | minimal | none
	Timezone          string   `json:"timezone" yaml:"timezone"`
	BootstrapMaxChars int      `json:"bootstrap_max_chars" yaml:"bootstrap_max_chars"`
	BootstrapFiles    []string `json:"bootstrap_files" yaml:"bootstrap_files"`
	ReplyTagsEnabled  bool     `json:"reply_tags_enabled" yaml:"reply_tags_enabled"`
}

// OKRProactiveConfig configures OKR goal management behavior.
type OKRProactiveConfig struct {
	Enabled    bool   `json:"enabled" yaml:"enabled"`
	GoalsRoot  string `json:"goals_root" yaml:"goals_root"`   // default: ~/.alex/goals
	AutoInject bool   `json:"auto_inject" yaml:"auto_inject"` // inject OKR context into tasks
}

// MemoryConfig controls loading persistent Markdown memory.
type MemoryConfig struct {
	Enabled          bool              `json:"enabled" yaml:"enabled"`
	Index            MemoryIndexConfig `json:"index" yaml:"index"`
	ArchiveAfterDays int               `json:"archive_after_days" yaml:"archive_after_days"` // move daily entries older than N days to archive/ (default 30, 0 disables)
	CleanupInterval  string            `json:"cleanup_interval" yaml:"cleanup_interval"`     // how often to run cleanup (default "24h", Go duration)
}

// MemoryIndexConfig controls local vector indexing for Markdown memory.
type MemoryIndexConfig struct {
	Enabled            bool    `json:"enabled" yaml:"enabled"`
	DBPath             string  `json:"db_path" yaml:"db_path"`
	ChunkTokens        int     `json:"chunk_tokens" yaml:"chunk_tokens"`
	ChunkOverlap       int     `json:"chunk_overlap" yaml:"chunk_overlap"`
	MinScore           float64 `json:"min_score" yaml:"min_score"`
	FusionWeightVector float64 `json:"fusion_weight_vector" yaml:"fusion_weight_vector"`
	FusionWeightBM25   float64 `json:"fusion_weight_bm25" yaml:"fusion_weight_bm25"`
	EmbedderModel      string  `json:"embedder_model" yaml:"embedder_model"`
}

// SkillsConfig controls skill activation and feedback.
type SkillsConfig struct {
	AutoActivation           SkillsAutoActivationConfig `json:"auto_activation" yaml:"auto_activation"`
	Feedback                 SkillsFeedbackConfig       `json:"feedback" yaml:"feedback"`
	CacheTTLSeconds          int                        `json:"cache_ttl_seconds" yaml:"cache_ttl_seconds"`
	MetaOrchestratorEnabled  bool                       `json:"meta_orchestrator_enabled" yaml:"meta_orchestrator_enabled"`
	SoulAutoEvolutionEnabled bool                       `json:"soul_auto_evolution_enabled" yaml:"soul_auto_evolution_enabled"`
	ProactiveLevel           string                     `json:"proactive_level" yaml:"proactive_level"`
	PolicyPath               string                     `json:"policy_path" yaml:"policy_path"`
}

type SkillsAutoActivationConfig struct {
	Enabled             bool    `json:"enabled" yaml:"enabled"`
	MaxActivated        int     `json:"max_activated" yaml:"max_activated"`
	TokenBudget         int     `json:"token_budget" yaml:"token_budget"`
	ConfidenceThreshold float64 `json:"confidence_threshold" yaml:"confidence_threshold"`
}

type SkillsFeedbackConfig struct {
	Enabled   bool   `json:"enabled" yaml:"enabled"`
	StorePath string `json:"store_path" yaml:"store_path"`
}

// DefaultProactiveConfig returns the baseline proactive defaults.
func DefaultProactiveConfig() ProactiveConfig {
	return ProactiveConfig{
		Enabled: true,
		Prompt: PromptConfig{
			Mode:              "full",
			BootstrapMaxChars: 20000,
			BootstrapFiles: []string{
				"AGENTS.md",
				"SOUL.md",
				"TOOLS.md",
				"IDENTITY.md",
				"USER.md",
				"HEARTBEAT.md",
				"BOOTSTRAP.md",
			},
		},
		Memory: MemoryConfig{
			Enabled:          true,
			ArchiveAfterDays: 30,
			CleanupInterval:  "24h",
			Index: MemoryIndexConfig{
				Enabled:            true,
				DBPath:             "~/.alex/memory/index.sqlite",
				ChunkTokens:        400,
				ChunkOverlap:       80,
				MinScore:           0.35,
				FusionWeightVector: 0.7,
				FusionWeightBM25:   0.3,
				EmbedderModel:      "nomic-embed-text",
			},
		},
		Skills: SkillsConfig{
			AutoActivation: SkillsAutoActivationConfig{
				Enabled:             true,
				MaxActivated:        3,
				TokenBudget:         4000,
				ConfidenceThreshold: 0.6,
			},
			Feedback: SkillsFeedbackConfig{
				Enabled: false,
			},
			CacheTTLSeconds:          300,
			MetaOrchestratorEnabled:  true,
			SoulAutoEvolutionEnabled: true,
			ProactiveLevel:           "medium",
			PolicyPath:               "configs/skills/meta-orchestrator.yaml",
		},
		OKR: OKRProactiveConfig{
			Enabled:    true,
			AutoInject: true,
		},
		Scheduler: SchedulerConfig{
			Enabled:                          false,
			TriggerTimeoutSeconds:            900,
			ConcurrencyPolicy:                "skip",
			LeaderLockEnabled:                true,
			LeaderLockName:                   "proactive_scheduler",
			LeaderLockAcquireIntervalSeconds: 15,
			CooldownSeconds:                  0,
			MaxConcurrent:                    1,
			RecoveryMaxRetries:               0,
			RecoveryBackoffSeconds:           60,
			CalendarReminder: CalendarReminderConfig{
				Enabled:          false,
				Schedule:         "*/15 * * * *",
				LookAheadMinutes: 120,
			},
			Heartbeat: HeartbeatConfig{
				Enabled:          false,
				Schedule:         "*/30 * * * *",
				Task:             "Read HEARTBEAT.md if it exists. Follow it strictly. If nothing needs attention, reply HEARTBEAT_OK.",
				QuietHours:       [2]int{23, 8},
				WindowLookbackHr: 8,
			},
			MilestoneCheckin: MilestoneCheckinConfig{
				Enabled:          false,
				Schedule:         "0 */1 * * *",
				LookbackSeconds:  3600,
				IncludeActive:    true,
				IncludeCompleted: true,
			},
			WeeklyPulse: WeeklyPulseConfig{
				Enabled:  false,
				Schedule: "0 9 * * 1", // Monday 9am
			},
			BlockerRadar: BlockerRadarConfig{
				Enabled:               false,
				Schedule:              "*/10 * * * *",
				StaleThresholdSeconds: 1800,
				InputWaitSeconds:      900,
			},
			PrepBrief: PrepBriefConfig{
				Enabled:         false,
				Schedule:        "30 8 * * 1-5", // weekdays 8:30am
				LookbackSeconds: 604800,          // 7 days
			},
		},
		Timer: TimerConfig{
			Enabled:            true,
			StorePath:          "~/.alex/timers",
			MaxTimers:          100,
			TaskTimeoutSeconds: 900,
			HeartbeatEnabled:   false,
			HeartbeatMinutes:   30,
		},
		Attention: AttentionConfig{
			MaxDailyNotifications: 5,
			MinIntervalSeconds:    1800,
			QuietHours:            [2]int{22, 8},
			PriorityThreshold:     0.6,
		},
	}
}
