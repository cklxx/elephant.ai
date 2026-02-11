package kernel

// KernelConfig holds only the fields the Engine reads at runtime.
// DI-only concerns (Enabled, StateDir, TimeoutSeconds, LeaseSeconds, ChatID,
// Agents) stay in KernelProactiveConfig and are consumed by the DI builder.
type KernelConfig struct {
	KernelID      string `yaml:"kernel_id"`
	Schedule      string `yaml:"schedule"`
	SeedState     string `yaml:"seed_state"`
	MaxConcurrent int    `yaml:"max_concurrent"`
	Channel       string `yaml:"channel"`
	UserID        string `yaml:"user_id"`
}

// AgentConfig defines a single agent that the kernel dispatches.
type AgentConfig struct {
	AgentID  string            `yaml:"agent_id"`
	Prompt   string            `yaml:"prompt"`    // may contain {STATE} placeholder
	Priority int               `yaml:"priority"`
	Enabled  bool              `yaml:"enabled"`
	Metadata map[string]string `yaml:"metadata,omitempty"`
}
