package swe_bench

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
)

// AlexAgentFactory implements the AgentFactory interface using real Alex agent
type AlexAgentFactory struct{}

// NewAlexAgentFactory creates a new agent factory for Alex ReactAgent
func NewAlexAgentFactory() *AlexAgentFactory {
	log.Println("[AGENT-FACTORY] Using Alex ReactAgent for SWE-Bench evaluation")
	return &AlexAgentFactory{}
}

// CreateAgent creates a real Alex agent instance
func (af *AlexAgentFactory) CreateAgent(ctx context.Context, config *BatchConfig) (Agent, error) {
	// Validate configuration first
	if err := af.ValidateConfig(config); err != nil {
		return nil, fmt.Errorf("invalid config: %w", err)
	}

	log.Printf("[AGENT-FACTORY] Creating AlexAgent with model: %s", config.Agent.Model.Name)
	agent, err := NewAlexAgent(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create AlexAgent: %w", err)
	}

	return agent, nil
}

// ValidateConfig validates the agent configuration
func (af *AlexAgentFactory) ValidateConfig(config *BatchConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	if config.Agent.Model.Name == "" {
		return fmt.Errorf("model name is required")
	}

	if config.Agent.Model.Temperature < 0 || config.Agent.Model.Temperature > 2 {
		return fmt.Errorf("temperature must be between 0 and 2")
	}

	if config.Agent.MaxTurns <= 0 {
		return fmt.Errorf("max turns must be positive")
	}

	if config.Agent.Timeout <= 0 {
		return fmt.Errorf("timeout must be positive")
	}

	// Check API key is available
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		apiKey = os.Getenv("ANTHROPIC_API_KEY")
	}
	if apiKey == "" {
		apiKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if apiKey == "" {
		log.Println("[AGENT-FACTORY] Warning: No API key found in environment variables")
		log.Println("[AGENT-FACTORY] Set OPENAI_API_KEY, ANTHROPIC_API_KEY, or DEEPSEEK_API_KEY")
	}

	// Validate model-specific settings for reasoning models
	modelLower := strings.ToLower(config.Agent.Model.Name)
	if strings.Contains(modelLower, "r1") || strings.Contains(modelLower, "reasoning") {
		// Reasoning models need more tokens and time
		if config.Agent.Model.MaxTokens < 4000 {
			log.Printf("[AGENT-FACTORY] Warning: Reasoning model %s may need more tokens (current: %d)",
				config.Agent.Model.Name, config.Agent.Model.MaxTokens)
		}
		if config.Agent.Timeout < 300 {
			log.Printf("[AGENT-FACTORY] Warning: Reasoning model %s may need more time (current: %ds)",
				config.Agent.Model.Name, config.Agent.Timeout)
		}
	}

	return nil
}
