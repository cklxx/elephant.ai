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
}
