package kernel

// KernelConfig holds the configuration for a kernel engine instance.
type KernelConfig struct {
	Enabled        bool              `yaml:"enabled"`
	KernelID       string            `yaml:"kernel_id"`
	Schedule       string            `yaml:"schedule"`
	StateDir       string            `yaml:"state_dir"`
	SeedState      string            `yaml:"seed_state"`
	TimeoutSeconds int               `yaml:"timeout_seconds"`
	LeaseSeconds   int               `yaml:"lease_seconds"`
	MaxConcurrent  int               `yaml:"max_concurrent"`
	Channel        string            `yaml:"channel"`
	UserID         string            `yaml:"user_id"`
	ChatID         string            `yaml:"chat_id"`
	Agents         []AgentConfig     `yaml:"agents"`
}

// AgentConfig defines a single agent that the kernel dispatches.
type AgentConfig struct {
	AgentID  string            `yaml:"agent_id"`
	Prompt   string            `yaml:"prompt"`    // may contain {STATE} placeholder
	Priority int               `yaml:"priority"`
	Enabled  bool              `yaml:"enabled"`
	Metadata map[string]string `yaml:"metadata,omitempty"`
}
