package tools

import "time"

// ToolPolicy determines timeout and retry behavior per tool.
type ToolPolicy interface {
	// TimeoutFor returns the execution timeout for the named tool.
	TimeoutFor(toolName string) time.Duration

	// RetryConfigFor returns the retry configuration for the named tool.
	// When dangerous is true, retries are suppressed (MaxRetries = 0).
	RetryConfigFor(toolName string, dangerous bool) ToolRetryConfig

	// Resolve evaluates all rules against the full ToolCallContext and
	// returns the merged policy result.
	Resolve(ctx ToolCallContext) ResolvedPolicy
}

// ToolRetryConfig controls retry behavior for tool executions.
type ToolRetryConfig struct {
	MaxRetries     int           `yaml:"max_retries" json:"max_retries"`
	InitialBackoff time.Duration `yaml:"initial_backoff" json:"initial_backoff"`
	MaxBackoff     time.Duration `yaml:"max_backoff" json:"max_backoff"`
	BackoffFactor  float64       `yaml:"backoff_factor" json:"backoff_factor"`
}

// ToolCallContext carries runtime context about the current tool invocation
// so the policy engine can evaluate per-context selectors.
type ToolCallContext struct {
	ToolName    string
	Category    string
	Tags        []string
	Dangerous   bool
	Channel     string // cli, web, lark, wechat
	SafetyLevel int    // effective safety level (1-4; 0=unset)
}

// ResolvedPolicy is the final, flattened result of evaluating all policy
// rules for a specific tool call.
type ResolvedPolicy struct {
	Timeout         time.Duration
	Retry           ToolRetryConfig
	Enabled         bool   // false = tool call blocked by policy
	EnforcementMode string // enforce | warn_allow
	SafetyLevel     int    // effective safety level from context (1-4; 0=unset)
}
