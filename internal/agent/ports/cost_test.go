package ports

import (
	"testing"
	"time"
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
			expectedInputCost:  0.005,  // 1000/1000 * 0.005
			expectedOutputCost: 0.0075, // 500/1000 * 0.015
			expectedTotalCost:  0.0125,
		},
		{
			name:               "gpt-4o-mini basic calculation",
			inputTokens:        10000,
			outputTokens:       5000,
			model:              "gpt-4o-mini",
			expectedInputCost:  0.0015, // 10000/1000 * 0.00015
			expectedOutputCost: 0.003,  // 5000/1000 * 0.0006
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
			expectedInputCost:  0.0007,  // 5000/1000 * 0.00014
			expectedOutputCost: 0.00056, // 2000/1000 * 0.00028
			expectedTotalCost:  0.00126,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			inputCost, outputCost, totalCost := CalculateCost(tt.inputTokens, tt.outputTokens, tt.model)

			// Use tolerance for floating point comparison
			tolerance := 0.000001
			if abs(inputCost-tt.expectedInputCost) > tolerance {
				t.Errorf("inputCost = %f, want %f", inputCost, tt.expectedInputCost)
			}
			if abs(outputCost-tt.expectedOutputCost) > tolerance {
				t.Errorf("outputCost = %f, want %f", outputCost, tt.expectedOutputCost)
			}
			if abs(totalCost-tt.expectedTotalCost) > tolerance {
				t.Errorf("totalCost = %f, want %f", totalCost, tt.expectedTotalCost)
			}
		})
	}
}

func TestUsageRecord(t *testing.T) {
	record := UsageRecord{
		ID:           "test-1",
		SessionID:    "session-123",
		Model:        "gpt-4o",
		Provider:     "openrouter",
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
		InputCost:    0.005,
		OutputCost:   0.0075,
		TotalCost:    0.0125,
		Timestamp:    time.Now(),
	}

	if record.ID != "test-1" {
		t.Errorf("ID = %s, want test-1", record.ID)
	}
	if record.TotalTokens != record.InputTokens+record.OutputTokens {
		t.Errorf("TotalTokens mismatch: %d != %d + %d", record.TotalTokens, record.InputTokens, record.OutputTokens)
	}
}

func TestCostSummary(t *testing.T) {
	summary := CostSummary{
		TotalCost:    0.0125,
		InputTokens:  1000,
		OutputTokens: 500,
		TotalTokens:  1500,
		RequestCount: 1,
		ByModel: map[string]float64{
			"gpt-4o": 0.0125,
		},
		ByProvider: map[string]float64{
			"openrouter": 0.0125,
		},
		StartTime: time.Now(),
		EndTime:   time.Now(),
	}

	if summary.TotalCost != 0.0125 {
		t.Errorf("TotalCost = %f, want 0.0125", summary.TotalCost)
	}
	if summary.RequestCount != 1 {
		t.Errorf("RequestCount = %d, want 1", summary.RequestCount)
	}
	if len(summary.ByModel) != 1 {
		t.Errorf("len(ByModel) = %d, want 1", len(summary.ByModel))
	}
}

func abs(x float64) float64 {
	if x < 0 {
		return -x
	}
	return x
}
