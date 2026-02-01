package tools

import "time"

// ToolRetryConfig controls retry behavior for tool executions.
type ToolRetryConfig struct {
	MaxRetries     int           `yaml:"max_retries" json:"max_retries"`
	InitialBackoff time.Duration `yaml:"initial_backoff" json:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff" json:"max_backoff"`
	BackoffFactor  float64       `yaml:"backoff_factor" json:"backoff_factor"`
}

// ToolTimeoutConfig controls per-tool timeout overrides.
type ToolTimeoutConfig struct {
	Default time.Duration            `yaml:"default" json:"default"`
	PerTool map[string]time.Duration `yaml:"per_tool" json:"per_tool"`
}

// ToolPolicyConfig combines timeout and retry configuration for tool execution.
type ToolPolicyConfig struct {
	Timeout ToolTimeoutConfig `yaml:"timeout" json:"timeout"`
	Retry   ToolRetryConfig   `yaml:"retry" json:"retry"`
}

// DefaultToolPolicyConfig returns sensible defaults:
//   - Default timeout: 120s
//   - Max retries: 2 (caller should use RetryConfigFor to get 0 for dangerous tools)
//   - Initial backoff: 1s, max backoff: 30s, factor: 2.0
func DefaultToolPolicyConfig() ToolPolicyConfig {
	return ToolPolicyConfig{
		Timeout: ToolTimeoutConfig{
			Default: 120 * time.Second,
			PerTool: map[string]time.Duration{},
		},
		Retry: ToolRetryConfig{
			MaxRetries:     2,
			InitialBackoff: 1 * time.Second,
			MaxBackoff:     30 * time.Second,
			BackoffFactor:  2.0,
		},
	}
}

// ToolPolicy determines timeout and retry behavior per tool.
type ToolPolicy interface {
	// TimeoutFor returns the execution timeout for the named tool.
	TimeoutFor(toolName string) time.Duration

	// RetryConfigFor returns the retry configuration for the named tool.
	// When dangerous is true, retries are suppressed (MaxRetries = 0).
	RetryConfigFor(toolName string, dangerous bool) ToolRetryConfig
}

// configToolPolicy implements ToolPolicy backed by ToolPolicyConfig.
type configToolPolicy struct {
	cfg ToolPolicyConfig
}

// NewToolPolicy creates a ToolPolicy from the given configuration.
func NewToolPolicy(cfg ToolPolicyConfig) ToolPolicy {
	return &configToolPolicy{cfg: cfg}
}

// TimeoutFor returns the per-tool timeout if configured, otherwise the global default.
func (p *configToolPolicy) TimeoutFor(toolName string) time.Duration {
	if d, ok := p.cfg.Timeout.PerTool[toolName]; ok && d > 0 {
		return d
	}
	if p.cfg.Timeout.Default > 0 {
		return p.cfg.Timeout.Default
	}
	return 120 * time.Second
}

// RetryConfigFor returns the global retry config for safe tools.
// Dangerous tools always receive zero retries to avoid repeating side-effects.
func (p *configToolPolicy) RetryConfigFor(toolName string, dangerous bool) ToolRetryConfig {
	if dangerous {
		return ToolRetryConfig{
			MaxRetries:     0,
			InitialBackoff: p.cfg.Retry.InitialBackoff,
			MaxBackoff:     p.cfg.Retry.MaxBackoff,
			BackoffFactor:  p.cfg.Retry.BackoffFactor,
		}
	}
	return p.cfg.Retry
}
