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

const (
	charsPerToken    = 4
	maxChunkTokens   = 4000
	minChunkTokens   = 2000
	overlapTokens    = 200
	chunkingThreshold = 3000
)

// Extractor uses an LLM to extract facts from conversation text.
type Extractor struct {
	llmClient llm.LLMClient
	budgetMax int
	nowFn     func() time.Time
}

// NewExtractor creates an Extractor with the given LLM client and token budget.
func NewExtractor(client llm.LLMClient, budgetMax int, nowFn func() time.Time) *Extractor {
	return &Extractor{llmClient: client, budgetMax: budgetMax, nowFn: nowFn}
}

// ExtractDaily reads a day's conversations and extracts structured facts.
func (e *Extractor) ExtractDaily(ctx context.Context, content string, date string) (*DailyExtraction, error) {
	chunks := chunkContent(content)
	var allFacts []ExtractedFact
	totalTokens := 0

	for _, chunk := range chunks {
		facts, tokens, err := e.extractChunk(ctx, chunk, date)
		if err != nil {
			return nil, fmt.Errorf("extract chunk: %w", err)
		}
		allFacts = append(allFacts, facts...)
		totalTokens += tokens
	}

	return &DailyExtraction{Date: date, Facts: allFacts, Tokens: totalTokens}, nil
}

func (e *Extractor) extractChunk(ctx context.Context, chunk string, date string) ([]ExtractedFact, int, error) {
	resp, err := e.llmClient.Complete(ctx, core.CompletionRequest{
		Messages:  []core.Message{{Role: "user", Content: buildExtractionPrompt(chunk, date)}},
		MaxTokens: e.budgetMax,
	})
	if err != nil {
		return nil, 0, err
	}

	var raw []extractedFactRaw
	if err := jsonx.Unmarshal([]byte(resp.Content), &raw); err != nil {
		return nil, resp.Usage.TotalTokens, fmt.Errorf("parse extraction response: %w", err)
	}

	facts := make([]ExtractedFact, len(raw))
	now := e.nowFn()
	for i, r := range raw {
		facts[i] = ExtractedFact{
			ID: id.NewKSUID(), Content: r.Content, Category: r.Category,
			Confidence: r.Confidence, Source: date, CreatedAt: now,
		}
	}
	return facts, resp.Usage.TotalTokens, nil
}

type extractedFactRaw struct {
	Content    string  `json:"content"`
	Category   string  `json:"category"`
	Confidence float64 `json:"confidence"`
}

func buildExtractionPrompt(content, date string) string {
	return fmt.Sprintf(`Analyze this conversation from %s and extract structured facts.

Identify:
- Decisions made (who decided what, why)
- Preferences expressed (language, format, timing)
- Facts stated (project status, team info, deadlines)
- Patterns observed (recurring behaviors, repeated choices)

Return a JSON array of objects with fields: content (string), category (one of: decision, preference, fact, pattern), confidence (0.0-1.0).

Conversation:
%s`, date, content)
}

func chunkContent(content string) []string {
	tokenEstimate := len(content) / charsPerToken
	if tokenEstimate <= chunkingThreshold {
		return []string{content}
	}

	chunkChars := minChunkTokens * charsPerToken
	overlapChars := overlapTokens * charsPerToken
	var chunks []string

	for start := 0; start < len(content); {
		end := start + chunkChars
		if end > len(content) {
			end = len(content)
		}
		chunks = append(chunks, content[start:end])
		start = end - overlapChars
		if start < 0 {
			break
		}
		if end == len(content) {
			break
		}
	}
	return chunks
}
