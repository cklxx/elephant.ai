package config

import (
	"time"

	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"
)

// StewardConfig controls the steward AI mode where a structured state document
// is maintained across turns and injected as a SYSTEM_REMINDER.
type StewardConfig struct {
	Enabled                   bool    `yaml:"enabled"`
	MaxStateChars             int     `yaml:"max_state_chars"`              // default 1400
	StateCompressionThreshold float64 `yaml:"state_compression_threshold"`  // default 0.70
	AggressiveTrimThreshold   float64 `yaml:"aggressive_trim_threshold"`    // default 0.85
	MaxTurnRetention          int     `yaml:"max_turn_retention"`           // default 6
}

// DefaultStewardConfig returns sensible defaults for steward mode.
func DefaultStewardConfig() StewardConfig {
	return StewardConfig{
		Enabled:                   false,
		MaxStateChars:             1400,
		StateCompressionThreshold: 0.70,
		AggressiveTrimThreshold:   0.85,
		MaxTurnRetention:          6,
	}
}

// Config captures runtime defaults for coordinator execution and preparation.
type Config struct {
	LLMProvider         string
	LLMModel            string
	LLMSmallProvider    string
	LLMSmallModel       string
	LLMVisionModel      string
	APIKey              string
	BaseURL             string
	MaxTokens           int
	MaxIterations       int
	ToolMaxConcurrent   int
	Temperature         float64
	TemperatureProvided bool
	TopP                float64
	StopSequences       []string
	AgentPreset         string // Agent persona preset (default, code-expert, etc.)
	ToolPreset          string // Tool access preset (full, read-only, safe, architect)
	ToolMode            string // Tool access mode (web or cli)
	EnvironmentSummary  string
	SessionStaleAfter   time.Duration
	Proactive           runtimeconfig.ProactiveConfig
	ToolPolicy          toolspolicy.ToolPolicyConfig
	Steward             StewardConfig
}
