package distillation

import (
	"context"
	"fmt"
	"time"

	core "alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/llm"
	jsonx "alex/internal/shared/json"
	"alex/internal/shared/utils/id"
)

// PatternAnalyzer derives weekly patterns from daily extractions.
type PatternAnalyzer struct {
	llmClient llm.LLMClient
	nowFn     func() time.Time
}

// NewPatternAnalyzer creates a PatternAnalyzer.
func NewPatternAnalyzer(client llm.LLMClient, nowFn func() time.Time) *PatternAnalyzer {
	return &PatternAnalyzer{llmClient: client, nowFn: nowFn}
}

// AnalyzeWeek takes daily extractions from the past week and finds patterns.
func (p *PatternAnalyzer) AnalyzeWeek(ctx context.Context, extractions []DailyExtraction) ([]WeeklyPattern, error) {
	prompt := buildPatternPrompt(extractions)
	resp, err := p.llmClient.Complete(ctx, core.CompletionRequest{
		Messages:  []core.Message{{Role: "user", Content: prompt}},
		MaxTokens: 4096,
	})
	if err != nil {
		return nil, fmt.Errorf("pattern analysis: %w", err)
	}

	var raw []weeklyPatternRaw
	if err := jsonx.Unmarshal([]byte(resp.Content), &raw); err != nil {
		return nil, fmt.Errorf("parse pattern response: %w", err)
	}

	now := p.nowFn()
	patterns := make([]WeeklyPattern, len(raw))
	for i, r := range raw {
		patterns[i] = WeeklyPattern{
			ID: id.NewKSUID(), Description: r.Description, Category: r.Category,
			Evidence: r.Evidence, Confidence: r.Confidence,
			CreatedAt: now, UpdatedAt: now,
		}
	}
	return patterns, nil
}

type weeklyPatternRaw struct {
	Description string   `json:"description"`
	Category    string   `json:"category"`
	Evidence    []string `json:"evidence"`
	Confidence  float64  `json:"confidence"`
}

func buildPatternPrompt(extractions []DailyExtraction) string {
	factsJSON, _ := jsonx.Marshal(extractions)
	return fmt.Sprintf(`Analyze these daily fact extractions from the past week and identify higher-level patterns.

Look for:
- Recurring decisions or preferences
- Consistent behavioral patterns
- Evolving trends across days

Return a JSON array of objects with fields: description (string), category (string), evidence (array of fact IDs), confidence (0.0-1.0).

Daily extractions:
%s`, string(factsJSON))
}
