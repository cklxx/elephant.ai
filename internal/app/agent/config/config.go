package config

import (
	"strings"
	"time"

	toolspolicy "alex/internal/infra/tools"
	runtimeconfig "alex/internal/shared/config"
)

// Config captures runtime defaults for coordinator execution and preparation.
type Config struct {
	LLMProvider         string
	LLMModel            string
	LLMSmallProvider    string
	LLMSmallModel       string
	LLMVisionModel      string
	APIKey              string
	BaseURL             string
	LLMProfile          runtimeconfig.LLMProfile
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
}

// DefaultLLMProfile returns the resolved default profile used by the runtime.
// It prefers the explicit LLMProfile field and falls back to legacy fields.
func (c Config) DefaultLLMProfile() runtimeconfig.LLMProfile {
	profile := c.LLMProfile
	if strings.TrimSpace(profile.Provider) == "" {
		profile.Provider = strings.TrimSpace(c.LLMProvider)
	}
	if strings.TrimSpace(profile.Model) == "" {
		profile.Model = strings.TrimSpace(c.LLMModel)
	}
	if strings.TrimSpace(profile.APIKey) == "" {
		profile.APIKey = strings.TrimSpace(c.APIKey)
	}
	if strings.TrimSpace(profile.BaseURL) == "" {
		profile.BaseURL = strings.TrimSpace(c.BaseURL)
	}
	return profile
}

// SmallLLMProfile returns the profile for small-model pre-analysis if configured.
func (c Config) SmallLLMProfile() (runtimeconfig.LLMProfile, bool) {
	model := strings.TrimSpace(c.LLMSmallModel)
	if model == "" {
		return runtimeconfig.LLMProfile{}, false
	}
	profile := c.DefaultLLMProfile()
	provider := strings.TrimSpace(c.LLMSmallProvider)
	if provider != "" {
		profile.Provider = provider
	}
	profile.Model = model
	if strings.TrimSpace(profile.Provider) == "" {
		return runtimeconfig.LLMProfile{}, false
	}
	return profile, true
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
	if strings.TrimSpace(profile.Provider) == "" {
		return runtimeconfig.LLMProfile{}, false
	}
	return profile, true
}
