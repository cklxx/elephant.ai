//go:build ignore
package evolution

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"
)

// PromptOptimizer analyzes successful patterns and generates improved prompts
type PromptOptimizer struct {
	llmClient      LLMClient
	minSamples     int
	maxPromptChars int
}

// NewPromptOptimizer creates a new prompt optimizer
func NewPromptOptimizer(llm LLMClient) *PromptOptimizer {
	return &PromptOptimizer{
		llmClient:      llm,
		minSamples:     3,
		maxPromptChars: 8000,
	}
}

// OptimizationResult contains the optimized prompt and metadata
type OptimizationResult struct {
	OriginalPrompt string
	OptimizedPrompt string
	Changes        []PromptChange
	Rationale      string
	Confidence     float64
}

// PromptChange describes a single modification
type PromptChange struct {
	Type        string // "add", "remove", "modify"
	Section     string
	Description string
}

// OptimizePrompt generates an improved prompt based on learning history
func (po *PromptOptimizer) OptimizePrompt(ctx context.Context, currentPrompt string, history []LearningRecord) (*OptimizationResult, error) {
	if len(history) < po.minSamples {
		return nil, fmt.Errorf("insufficient learning history: need at least %d samples, got %d", po.minSamples, len(history))
	}

	// Build context from learning history
	var successfulPatterns []string
	var failedPatterns []string
	
	for _, record := range history {
		if record.Outcome.Success {
			if record.PatternSummary != "" {
				successfulPatterns = append(successfulPatterns, record.PatternSummary)
			}
		} else {
			if record.PatternSummary != "" {
				failedPatterns = append(failedPatterns, record.PatternSummary)
			}
		}
	}

	// Create optimization prompt
	optimizationPrompt := po.buildOptimizationPrompt(currentPrompt, successfulPatterns, failedPatterns)
	
	// Call LLM for optimization
	response, err := po.llmClient.Complete(ctx, optimizationPrompt)
	if err != nil {
		return nil, fmt.Errorf("llm optimization failed: %w", err)
	}

	// Parse optimization result
	result, err := po.parseOptimizationResponse(currentPrompt, response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse optimization: %w", err)
	}

	return result, nil
}

// buildOptimizationPrompt constructs the prompt for LLM-based optimization
func (po *PromptOptimizer) buildOptimizationPrompt(currentPrompt string, successfulPatterns, failedPatterns []string) string {
	var b strings.Builder
	
	b.WriteString("You are a prompt optimization expert. Analyze the current prompt and learning history to create an improved version.\n\n")
	b.WriteString("## Current Prompt\n```\n")
	if len(currentPrompt) > po.maxPromptChars {
		b.WriteString(currentPrompt[:po.maxPromptChars])
		b.WriteString("\n... (truncated)")
	} else {
		b.WriteString(currentPrompt)
	}
	b.WriteString("\n```\n\n")

	if len(successfulPatterns) > 0 {
		b.WriteString("## Successful Patterns (DO MORE OF THESE)\n")
		for i, pattern := range successfulPatterns {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, pattern))
		}
		b.WriteString("\n")
	}

	if len(failedPatterns) > 0 {
		b.WriteString("## Failed Patterns (AVOID THESE)\n")
		for i, pattern := range failedPatterns {
			b.WriteString(fmt.Sprintf("%d. %s\n", i+1, pattern))
		}
		b.WriteString("\n")
	}

	b.WriteString("## Instructions\n")
	b.WriteString("1. Analyze the patterns to identify what works and what doesn't\n")
	b.WriteString("2. Create an improved prompt that amplifies successful patterns\n")
	b.WriteString("3. Remove or modify elements that lead to failures\n")
	b.WriteString("4. Keep the prompt clear, specific, and actionable\n")
	b.WriteString("5. Preserve the original intent while improving effectiveness\n\n")

	b.WriteString("## Output Format\n")
	b.WriteString("Return a JSON object with this structure:\n")
	b.WriteString("```json\n")
	b.WriteString("{\n")
	b.WriteString("  \"optimized_prompt\": \"the improved prompt text\",\n")
	b.WriteString("  \"changes\": [\n")
	b.WriteString("    {\"type\": \"add|remove|modify\", \"section\": \"section name\", \"description\": \"what changed\"}\n")
	b.WriteString("  ],\n")
	b.WriteString("  \"rationale\": \"explanation of changes\",\n")
	b.WriteString("  \"confidence\": 0.85\n")
	b.WriteString("}\n")
	b.WriteString("```\n")

	return b.String()
}

// parseOptimizationResponse parses the LLM response into OptimizationResult
func (po *PromptOptimizer) parseOptimizationResponse(originalPrompt, response string) (*OptimizationResult, error) {
	// Extract JSON from response
	jsonStr := po.extractJSON(response)
	
	var raw struct {
		OptimizedPrompt string `json:"optimized_prompt"`
		Changes         []struct {
			Type        string `json:"type"`
			Section     string `json:"section"`
			Description string `json:"description"`
		} `json:"changes"`
		Rationale  string  `json:"rationale"`
		Confidence float64 `json:"confidence"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	// Convert changes
	changes := make([]PromptChange, len(raw.Changes))
	for i, c := range raw.Changes {
		changes[i] = PromptChange{
			Type:        c.Type,
			Section:     c.Section,
			Description: c.Description,
		}
	}

	// Ensure confidence is within bounds
	confidence := raw.Confidence
	if confidence < 0 {
		confidence = 0
	}
	if confidence > 1 {
		confidence = 1
	}

	return &OptimizationResult{
		OriginalPrompt:  originalPrompt,
		OptimizedPrompt: raw.OptimizedPrompt,
		Changes:         changes,
		Rationale:       raw.Rationale,
		Confidence:      confidence,
	}, nil
}

// extractJSON extracts JSON from a text response
func (po *PromptOptimizer) extractJSON(text string) string {
	// Try to find JSON block
	startIdx := strings.Index(text, "```json")
	if startIdx != -1 {
		startIdx += len("```json")
		endIdx := strings.Index(text[startIdx:], "```")
		if endIdx != -1 {
			return strings.TrimSpace(text[startIdx : startIdx+endIdx])
		}
	}

	// Try to find any JSON object
	startIdx = strings.Index(text, "{")
	if startIdx != -1 {
		// Find matching closing brace
		depth := 0
		for i := startIdx; i < len(text); i++ {
			switch text[i] {
			case '{':
				depth++
			case '}':
				depth--
				if depth == 0 {
					return strings.TrimSpace(text[startIdx : i+1])
				}
			}
		}
	}

	return text
}

// StrategyOptimizer suggests improvements to agent strategies
type StrategyOptimizer struct {
	llmClient LLMClient
}

// NewStrategyOptimizer creates a new strategy optimizer
func NewStrategyOptimizer(llm LLMClient) *StrategyOptimizer {
	return &StrategyOptimizer{llmClient: llm}
}

// StrategyRecommendation contains suggested strategy changes
type StrategyRecommendation struct {
	CurrentStrategy string
	SuggestedStrategy string
	Improvements    []StrategyImprovement
	ExpectedImpact  string
}

// StrategyImprovement describes a specific improvement
type StrategyImprovement struct {
	Area        string
	Change      string
	ExpectedBenefit string
}

// AnalyzeStrategy analyzes current strategy and suggests improvements
func (so *StrategyOptimizer) AnalyzeStrategy(ctx context.Context, currentStrategy string, metrics *PerformanceMetrics) (*StrategyRecommendation, error) {
	prompt := so.buildStrategyPrompt(currentStrategy, metrics)
	
	response, err := so.llmClient.Complete(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("strategy analysis failed: %w", err)
	}

	return so.parseStrategyResponse(currentStrategy, response)
}

// buildStrategyPrompt constructs the prompt for strategy analysis
func (so *StrategyOptimizer) buildStrategyPrompt(currentStrategy string, metrics *PerformanceMetrics) string {
	return fmt.Sprintf(`Analyze the current agent strategy and suggest improvements based on performance metrics.

## Current Strategy
%s

## Performance Metrics
- Success Rate: %.2f%%
- Avg Iterations: %.2f
- Avg Duration: %v
- Tool Success Rate: %.2f%%
- User Satisfaction: %.2f/5

## Task
1. Identify weaknesses in the current strategy
2. Suggest specific improvements
3. Predict the expected impact of changes

Return your analysis as JSON:
{
  "suggested_strategy": "description of improved strategy",
  "improvements": [
    {"area": "area name", "change": "what to change", "expected_benefit": "why this helps"}
  ],
  "expected_impact": "summary of expected improvements"
}`, currentStrategy, metrics.SuccessRate*100, metrics.AverageIterations,
		metrics.AverageDuration, metrics.ToolSuccessRate*100, metrics.UserSatisfaction)
}

// parseStrategyResponse parses the strategy analysis response
func (so *StrategyOptimizer) parseStrategyResponse(currentStrategy, response string) (*StrategyRecommendation, error) {
	jsonStr := (&PromptOptimizer{}).extractJSON(response)
	
	var raw struct {
		SuggestedStrategy string `json:"suggested_strategy"`
		Improvements      []struct {
			Area            string `json:"area"`
			Change          string `json:"change"`
			ExpectedBenefit string `json:"expected_benefit"`
		} `json:"improvements"`
		ExpectedImpact string `json:"expected_impact"`
	}

	if err := json.Unmarshal([]byte(jsonStr), &raw); err != nil {
		return nil, fmt.Errorf("json parse error: %w", err)
	}

	improvements := make([]StrategyImprovement, len(raw.Improvements))
	for i, imp := range raw.Improvements {
		improvements[i] = StrategyImprovement{
			Area:            imp.Area,
			Change:          imp.Change,
			ExpectedBenefit: imp.ExpectedBenefit,
		}
	}

	return &StrategyRecommendation{
		CurrentStrategy:   currentStrategy,
		SuggestedStrategy: raw.SuggestedStrategy,
		Improvements:      improvements,
		ExpectedImpact:    raw.ExpectedImpact,
	}, nil
}

// AutoOptimizer automatically applies small optimizations without LLM
type AutoOptimizer struct {
	patternCache map[string][]string
}

// NewAutoOptimizer creates a new auto optimizer
func NewAutoOptimizer() *AutoOptimizer {
	return &AutoOptimizer{
		patternCache: make(map[string][]string),
	}
}

// QuickOptimize applies quick heuristic-based optimizations
func (ao *AutoOptimizer) QuickOptimize(prompt string, history []LearningRecord) string {
	optimized := prompt

	// Add timestamp if not present
	if !strings.Contains(optimized, "Current date") && !strings.Contains(optimized, "Today is") {
		optimized = ao.injectTimestamp(optimized)
	}

	// Add iteration limit reminder if iterations are high
	highIterationCount := 0
	for _, record := range history {
		if record.ExecutionMetrics.IterationCount > 10 {
			highIterationCount++
		}
	}
	if highIterationCount > len(history)/3 {
		optimized = ao.addEfficiencyReminder(optimized)
	}

	return optimized
}

// injectTimestamp adds current date to prompt
func (ao *AutoOptimizer) injectTimestamp(prompt string) string {
	currentDate := time.Now().Format("2006-01-02")
	timestampLine := fmt.Sprintf("Current date: %s", currentDate)
	
	// Try to find a good place to insert
	if idx := strings.Index(prompt, "\n\n"); idx != -1 {
		return prompt[:idx] + "\n" + timestampLine + prompt[idx:]
	}
	
	return timestampLine + "\n\n" + prompt
}

// addEfficiencyReminder adds a reminder about efficiency
func (ao *AutoOptimizer) addEfficiencyReminder(prompt string) string {
	reminder := "\n\nEfficiency: Aim to complete tasks in the minimum number of steps. Avoid unnecessary tool calls."
	return prompt + reminder
}
