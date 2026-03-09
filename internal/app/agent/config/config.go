package config

import (
	"alex/internal/shared/utils"
	"strings"
	"time"

	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"
)

// Config captures runtime defaults for coordinator execution and preparation.
type Config struct {
	LLMProvider         string
	LLMModel            string
	LLMVisionModel      string
	APIKey              string
	BaseURL             string
	LLMProfile          runtimeconfig.LLMProfile
	MaxTokens           int
	MaxIterations       int
	ToolMaxConcurrent   int
	MaxBackgroundTasks  int
	Temperature         float64
	TemperatureProvided bool
	TopP                float64
	StopSequences       []string
	AgentPreset         string // Agent persona preset (default, code-expert, etc.)
	ToolPreset          string // Tool access preset (full, read-only, safe, architect)
	ToolMode            string // Tool access mode (web or cli)
	EnvironmentSummary         string
	EnvironmentSummaryProvider func() string // lazy; resolved on first use, overrides EnvironmentSummary
	SessionStaleAfter          time.Duration
	Proactive           runtimeconfig.ProactiveConfig
	ToolPolicy          toolspolicy.ToolPolicyConfig
}

// ResolveEnvironmentSummary returns the environment summary, preferring the
// lazy provider when set, falling back to the static string.
func (c Config) ResolveEnvironmentSummary() string {
	if c.EnvironmentSummaryProvider != nil {
		return c.EnvironmentSummaryProvider()
	}
	return c.EnvironmentSummary
}

// DefaultLLMProfile returns the resolved default profile used by the runtime.
// It prefers the explicit LLMProfile field and falls back to legacy fields.
func (c Config) DefaultLLMProfile() runtimeconfig.LLMProfile {
	profile := c.LLMProfile
	if utils.IsBlank(profile.Provider) {
		profile.Provider = strings.TrimSpace(c.LLMProvider)
	}
	if utils.IsBlank(profile.Model) {
		profile.Model = strings.TrimSpace(c.LLMModel)
	}
	if utils.IsBlank(profile.APIKey) {
		profile.APIKey = strings.TrimSpace(c.APIKey)
	}
	if utils.IsBlank(profile.BaseURL) {
		profile.BaseURL = strings.TrimSpace(c.BaseURL)
	}
	return profile
}

// VisionLLMProfile returns the profile for vision tasks when a dedicated
// vision model is configured.
func (c Config) VisionLLMProfile() (runtimeconfig.LLMProfile, bool) {
	model := strings.TrimSpace(c.LLMVisionModel)
	if model == "" {
		return runtimeconfig.LLMProfile{}, false
	}
	profile := c.DefaultLLMProfile()
	profile.Model = model
	if utils.IsBlank(profile.Provider) {
		return runtimeconfig.LLMProfile{}, false
	}
	return profile, true
}
