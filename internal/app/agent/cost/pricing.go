package cost

import (
	"alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/modelregistry"
)

// GetModelPricing returns pricing for a given model.
// It tries the live models.dev registry first and falls back to a static table.
func GetModelPricing(model string) storage.ModelPricing {
	if info, ok := modelregistry.Lookup(model); ok && info.InputPer1M > 0 {
		return storage.ModelPricing{
			InputPer1K:  info.InputPer1M / 1000.0,
			OutputPer1K: info.OutputPer1M / 1000.0,
		}
	}

	// Static fallback table — values in USD per 1K tokens.
	pricingMap := map[string]storage.ModelPricing{
		// OpenAI
		"gpt-4":       {InputPer1K: 0.03, OutputPer1K: 0.06},
		"gpt-4-turbo": {InputPer1K: 0.01, OutputPer1K: 0.03},
		"gpt-4o":      {InputPer1K: 0.005, OutputPer1K: 0.015},
		"gpt-4o-mini": {InputPer1K: 0.00015, OutputPer1K: 0.0006},
		"gpt-5":       {InputPer1K: 0.015, OutputPer1K: 0.06},
		"gpt-5-mini":  {InputPer1K: 0.00150, OutputPer1K: 0.006},
		// Anthropic
		"claude-sonnet-4-6":         {InputPer1K: 0.003, OutputPer1K: 0.015},
		"claude-opus-4-6":           {InputPer1K: 0.015, OutputPer1K: 0.075},
		"claude-haiku-4-5-20251001": {InputPer1K: 0.00025, OutputPer1K: 0.00125},
		// DeepSeek
		"deepseek-chat":     {InputPer1K: 0.00014, OutputPer1K: 0.00028},
		"deepseek-reasoner": {InputPer1K: 0.00055, OutputPer1K: 0.00219},
		// OpenRouter prefixed
		"anthropic/claude-3-5-sonnet":       {InputPer1K: 0.003, OutputPer1K: 0.015},
		"anthropic/claude-3-opus":           {InputPer1K: 0.015, OutputPer1K: 0.075},
		"meta-llama/llama-3.1-70b-instruct": {InputPer1K: 0.0005, OutputPer1K: 0.0008},
	}

	if pricing, ok := pricingMap[model]; ok {
		return pricing
	}
	return storage.ModelPricing{InputPer1K: 0.001, OutputPer1K: 0.002}
}

// CalculateCost calculates cost based on token usage and model.
func CalculateCost(inputTokens, outputTokens int, model string) (inputCost, outputCost, totalCost float64) {
	pricing := GetModelPricing(model)

	inputCost = float64(inputTokens) / 1000.0 * pricing.InputPer1K
	outputCost = float64(outputTokens) / 1000.0 * pricing.OutputPer1K
	totalCost = inputCost + outputCost

	return
}
