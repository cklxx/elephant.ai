package cost

import (
	"testing"
)

func TestGetModelPricing(t *testing.T) {
	tests := []struct {
		name           string
		model          string
		expectedInput  float64
		expectedOutput float64
	}{
		{
			name:           "gpt-4",
			model:          "gpt-4",
			expectedInput:  0.03,
			expectedOutput: 0.06,
		},
		{
			name:           "gpt-4o",
			model:          "gpt-4o",
			expectedInput:  0.005,
			expectedOutput: 0.015,
		},
		{
			name:           "gpt-4o-mini",
			model:          "gpt-4o-mini",
			expectedInput:  0.00015,
			expectedOutput: 0.0006,
		},
		{
			name:           "deepseek-chat",
			model:          "deepseek-chat",
			expectedInput:  0.00014,
			expectedOutput: 0.00028,
		},
		{
			name:           "unknown model defaults",
			model:          "unknown-model",
			expectedInput:  0.001,
			expectedOutput: 0.002,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pricing := GetModelPricing(tt.model)
			if pricing.InputPer1K != tt.expectedInput {
				t.Errorf("InputPer1K = %f, want %f", pricing.InputPer1K, tt.expectedInput)
			}
			if pricing.OutputPer1K != tt.expectedOutput {
				t.Errorf("OutputPer1K = %f, want %f", pricing.OutputPer1K, tt.expectedOutput)
			}
		})
	}
}

func TestCalculateCost(t *testing.T) {
	tests := []struct {
		name               string
		inputTokens        int
		outputTokens       int
		model              string
		expectedInputCost  float64
		expectedOutputCost float64
		expectedTotalCost  float64
	}{
		{
			name:               "gpt-4o basic calculation",
			inputTokens:        1000,
			outputTokens:       500,
			model:              "gpt-4o",
			expectedInputCost:  0.005,
			expectedOutputCost: 0.0075,
			expectedTotalCost:  0.0125,
		},
		{
			name:               "gpt-4o-mini basic calculation",
			inputTokens:        10000,
			outputTokens:       5000,
			model:              "gpt-4o-mini",
			expectedInputCost:  0.0015,
			expectedOutputCost: 0.003,
			expectedTotalCost:  0.0045,
		},
		{
			name:               "zero tokens",
			inputTokens:        0,
			outputTokens:       0,
			model:              "gpt-4",
			expectedInputCost:  0,
			expectedOutputCost: 0,
			expectedTotalCost:  0,
		},
		{
			name:               "deepseek-chat calculation",
			inputTokens:        5000,
			outputTokens:       2000,
			model:              "deepseek-chat",
			expectedInputCost:  0.0007,
			expectedOutputCost: 0.00056,
			expectedTotalCost:  0.00126,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputCost, outputCost, totalCost := CalculateCost(tt.inputTokens, tt.outputTokens, tt.model)

			tolerance := 0.000001
			if absFloat(inputCost-tt.expectedInputCost) > tolerance {
				t.Errorf("inputCost = %f, want %f", inputCost, tt.expectedInputCost)
			}
			if absFloat(outputCost-tt.expectedOutputCost) > tolerance {
				t.Errorf("outputCost = %f, want %f", outputCost, tt.expectedOutputCost)
			}
			if absFloat(totalCost-tt.expectedTotalCost) > tolerance {
				t.Errorf("totalCost = %f, want %f", totalCost, tt.expectedTotalCost)
			}
		})
	}
}

// absFloat is already defined in decorator_test.go
