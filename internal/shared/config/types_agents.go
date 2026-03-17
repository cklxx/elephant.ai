package config

// ExternalAgentsConfig configures external agent executors.
// Retained for YAML config compatibility; agent execution has been removed.
type ExternalAgentsConfig struct {
	MaxParallelAgents int `json:"max_parallel_agents" yaml:"max_parallel_agents"`
}

// DefaultExternalAgentsConfig provides baseline defaults.
func DefaultExternalAgentsConfig() ExternalAgentsConfig {
	return ExternalAgentsConfig{
		MaxParallelAgents: 4,
	}
}
