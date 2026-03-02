//go:build ignore
// Package evolution provides self-improving agent capabilities
package evolution

import "time"

// Config holds configuration for the evolution system
type Config struct {
	// Enabled controls whether evolution is active
	Enabled bool `json:"enabled" yaml:"enabled"`

	// AutoEvolve enables automatic evolution without manual approval
	AutoEvolve bool `json:"auto_evolve" yaml:"auto_evolve"`

	// MinSamplesBeforeEvolution minimum samples needed before evolution triggers
	MinSamplesBeforeEvolution int `json:"min_samples_before_evolution" yaml:"min_samples_before_evolution"`

	// EvolutionInterval how often to check for evolution opportunities
	EvolutionInterval time.Duration `json:"evolution_interval" yaml:"evolution_interval"`

	// MinPerformanceScore threshold for considering evolution necessary
	MinPerformanceScore float64 `json:"min_performance_score" yaml:"min_performance_score"`

	// MaxPromptVersions maximum number of prompt versions to keep
	MaxPromptVersions int `json:"max_prompt_versions" yaml:"max_prompt_versions"`

	// LearningRetentionDays how long to keep learning entries
	LearningRetentionDays int `json:"learning_retention_days" yaml:"learning_retention_days"`

	// Analyzer configuration
	Analyzer AnalyzerConfig `json:"analyzer" yaml:"analyzer"`

	// Optimizer configuration
	Optimizer OptimizerConfig `json:"optimizer" yaml:"optimizer"`

	// Memory configuration
	Memory MemoryConfig `json:"memory" yaml:"memory"`
}

// AnalyzerConfig configuration for performance analysis
type AnalyzerConfig struct {
	// SuccessRateThreshold minimum success rate before flagging issues
	SuccessRateThreshold float64 `json:"success_rate_threshold" yaml:"success_rate_threshold"`

	// MaxLatencyThresholdMs maximum acceptable latency in milliseconds
	MaxLatencyThresholdMs int `json:"max_latency_threshold_ms" yaml:"max_latency_threshold_ms"`

	// MinToolSuccessRate minimum tool execution success rate
	MinToolSuccessRate float64 `json:"min_tool_success_rate" yaml:"min_tool_success_rate"`
}

// OptimizerConfig configuration for prompt optimization
type OptimizerConfig struct {
	// MaxIterations maximum optimization iterations
	MaxIterations int `json:"max_iterations" yaml:"max_iterations"`

	// MutationRate how aggressively to mutate prompts (0-1)
	MutationRate float64 `json:"mutation_rate" yaml:"mutation_rate"`

	// PreserveCoreInstructions whether to preserve critical instructions
	PreserveCoreInstructions bool `json:"preserve_core_instructions" yaml:"preserve_core_instructions"`

	// ABLTestRatio ratio of traffic to send to variant prompts
	ABLTestRatio float64 `json:"abl_test_ratio" yaml:"abl_test_ratio"`
}

// MemoryConfig configuration for learning memory
type MemoryConfig struct {
	// MaxPatternCount maximum patterns to retain
	MaxPatternCount int `json:"max_pattern_count" yaml:"max_pattern_count"`

	// PatternMinFrequency minimum occurrences before pattern is retained
	PatternMinFrequency int `json:"pattern_min_frequency" yaml:"pattern_min_frequency"`

	// CompressionEnabled whether to compress old learnings
	CompressionEnabled bool `json:"compression_enabled" yaml:"compression_enabled"`
}

// DefaultConfig returns a default evolution configuration
func DefaultConfig() Config {
	return Config{
		Enabled:                   true,
		AutoEvolve:                false,
		MinSamplesBeforeEvolution: 10,
		EvolutionInterval:         time.Hour * 24,
		MinPerformanceScore:       0.7,
		MaxPromptVersions:         5,
		LearningRetentionDays:     90,
		Analyzer: AnalyzerConfig{
			SuccessRateThreshold:  0.8,
			MaxLatencyThresholdMs: 10000,
			MinToolSuccessRate:    0.75,
		},
		Optimizer: OptimizerConfig{
			MaxIterations:            3,
			MutationRate:             0.2,
			PreserveCoreInstructions: true,
			ABLTestRatio:             0.1,
		},
		Memory: MemoryConfig{
			MaxPatternCount:     1000,
			PatternMinFrequency: 3,
			CompressionEnabled:  true,
		},
	}
}

// Validate checks configuration validity
func (c Config) Validate() error {
	if c.MinSamplesBeforeEvolution < 1 {
		c.MinSamplesBeforeEvolution = 10
	}
	if c.EvolutionInterval < time.Minute {
		c.EvolutionInterval = time.Minute * 10
	}
	if c.MinPerformanceScore < 0 || c.MinPerformanceScore > 1 {
		c.MinPerformanceScore = 0.7
	}
	if c.MaxPromptVersions < 2 {
		c.MaxPromptVersions = 5
	}
	if c.Analyzer.SuccessRateThreshold <= 0 || c.Analyzer.SuccessRateThreshold > 1 {
		c.Analyzer.SuccessRateThreshold = 0.8
	}
	if c.Optimizer.MutationRate < 0 || c.Optimizer.MutationRate > 1 {
		c.Optimizer.MutationRate = 0.2
	}
	return nil
}
