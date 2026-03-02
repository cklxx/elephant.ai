package kernel

import (
	"fmt"
	"os"
	"sync"

	"gopkg.in/yaml.v3"
)

// FallbackConfig mirrors configs/llm_provider_fallback.yaml structure
type FallbackConfig struct {
	PrimaryProvider PrimaryProvider   `yaml:"primary_provider"`
	FallbackChain   []FallbackEntry   `yaml:"fallback_chain"`
	ErrorPatterns   []string          `yaml:"error_patterns_triggering_fallback"`
	AgentOverrides  map[string]AgentOverride `yaml:"agent_specific_overrides"`
	Monitoring      MonitoringConfig  `yaml:"monitoring"`
}

type PrimaryProvider struct {
	Name           string            `yaml:"name"`
	Model          string            `yaml:"model"`
	TimeoutMs      int               `yaml:"timeout_ms"`
	RetryAttempts  int               `yaml:"retry_attempts"`
	CircuitBreaker CircuitBreakerConfig `yaml:"circuit_breaker"`
}

type CircuitBreakerConfig struct {
	FailureThreshold  int `yaml:"failure_threshold"`
	RecoveryTimeoutMs int `yaml:"recovery_timeout_ms"`
}

type FallbackEntry struct {
	Provider  string `yaml:"provider"`
	Model     string `yaml:"model"`
	Priority  int    `yaml:"priority"`
	TimeoutMs int    `yaml:"timeout_ms"`
}

type AgentOverride struct {
	Preferred string `yaml:"preferred"`
	Fallback  string `yaml:"fallback"`
}

type MonitoringConfig struct {
	LogProviderSwitches      bool    `yaml:"log_provider_switches"`
	AlertOnFallbackRateThreshold float64 `yaml:"alert_on_fallback_rate_threshold"`
	MetricsCollection        bool    `yaml:"metrics_collection"`
}

var (
	fallbackConfig     *FallbackConfig
	fallbackConfigOnce sync.Once
	fallbackConfigErr  error
)

// LoadFallbackConfig loads the LLM provider fallback configuration.
// It caches the result after first successful load.
func LoadFallbackConfig() (*FallbackConfig, error) {
	fallbackConfigOnce.Do(func() {
		configPath := os.Getenv("LLM_FALLBACK_CONFIG_PATH")
		if configPath == "" {
			configPath = "configs/llm_provider_fallback.yaml"
		}
		
		data, err := os.ReadFile(configPath)
		if err != nil {
			fallbackConfigErr = fmt.Errorf("failed to read fallback config from %s: %w", configPath, err)
			return
		}
		
		var cfg FallbackConfig
		if err := yaml.Unmarshal(data, &cfg); err != nil {
			fallbackConfigErr = fmt.Errorf("failed to parse fallback config: %w", err)
			return
		}
		
		fallbackConfig = &cfg
	})
	
	return fallbackConfig, fallbackConfigErr
}

// GetFallbackForAgent returns the fallback chain for a specific agent.
// Falls back to default chain if no agent-specific override exists.
func (c *FallbackConfig) GetFallbackForAgent(agentID string) []FallbackEntry {
	if override, ok := c.AgentOverrides[agentID]; ok {
		// Build priority-ordered chain for this agent
		var result []FallbackEntry
		for _, entry := range c.FallbackChain {
			if entry.Provider == override.Preferred || entry.Provider == override.Fallback {
				result = append(result, entry)
			}
		}
		if len(result) > 0 {
			return result
		}
	}
	return c.FallbackChain
}

// ShouldTriggerFallback checks if an error matches patterns that should trigger fallback.
func (c *FallbackConfig) ShouldTriggerFallback(err error) bool {
	if err == nil {
		return false
	}
	errStr := err.Error()
	for _, pattern := range c.ErrorPatterns {
		if containsIgnoreCase(errStr, pattern) {
			return true
		}
	}
	return false
}

func containsIgnoreCase(s, substr string) bool {
	if len(substr) > len(s) {
		return false
	}
	// Simple case-insensitive contains
	for i := 0; i <= len(s)-len(substr); i++ {
		match := true
		for j := 0; j < len(substr); j++ {
			if toLower(s[i+j]) != toLower(substr[j]) {
				match = false
				break
			}
		}
		if match {
			return true
		}
	}
	return false
}

func toLower(c byte) byte {
	if c >= 'A' && c <= 'Z' {
		return c + ('a' - 'A')
	}
	return c
}

