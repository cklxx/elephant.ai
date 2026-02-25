package agent

// SkillAutoActivationConfig controls automatic skill activation.
type SkillAutoActivationConfig struct {
	Enabled             bool
	MaxActivated        int
	TokenBudget         int
	ConfidenceThreshold float64
	CacheTTLSeconds     int
	FallbackToIndex     bool
}

// SkillFeedbackConfig controls skill feedback persistence.
type SkillFeedbackConfig struct {
	Enabled   bool
	StorePath string
}

// SkillsConfig groups skill-related runtime settings.
type SkillsConfig struct {
	AutoActivation SkillAutoActivationConfig
	Feedback       SkillFeedbackConfig
	// MetaOrchestratorEnabled toggles meta-level orchestration summary and filtering.
	MetaOrchestratorEnabled bool
	// SoulAutoEvolutionEnabled allows self_evolve_soul capability skills to auto-activate.
	SoulAutoEvolutionEnabled bool
	// ProactiveLevel controls orchestration aggressiveness: low|medium|high.
	ProactiveLevel string
	// PolicyPath points to the orchestration policy YAML file.
	PolicyPath string
}
