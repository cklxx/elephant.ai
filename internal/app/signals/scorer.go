package signals

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"alex/internal/domain/agent/ports"
	"alex/internal/domain/agent/ports/llm"
)

// Scorer assigns attention scores to signals using keyword matching
// with optional LLM escalation for ambiguous signals.
type Scorer struct {
	llmClient   llm.LLMClient
	budgetLimit int
	budgetUsed  int
	budgetReset time.Time
	nowFn       func() time.Time
	mu          sync.Mutex
}

// NewScorer creates a Scorer with the given LLM client and hourly budget.
func NewScorer(client llm.LLMClient, budgetPerHour int, nowFn func() time.Time) *Scorer {
	if nowFn == nil {
		nowFn = time.Now
	}
	return &Scorer{
		llmClient:   client,
		budgetLimit: budgetPerHour,
		nowFn:       nowFn,
		budgetReset: nowFn().Add(time.Hour),
	}
}

// Score assigns a 0-100 attention score to the event.
// Fast path uses keyword patterns. Slow path calls LLM for ambiguous signals.
func (s *Scorer) Score(ctx context.Context, event *SignalEvent) error {
	score := keywordScore(event.Content)
	if score >= 40 && score <= 80 && s.tryUseBudget() {
		llmScore, err := s.llmScore(ctx, event)
		if err == nil {
			score = llmScore
		}
	}
	event.Score = clamp(score, 0, 100)
	return nil
}

func (s *Scorer) tryUseBudget() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.nowFn()
	if now.After(s.budgetReset) {
		s.budgetUsed = 0
		s.budgetReset = now.Add(time.Hour)
	}
	if s.budgetUsed >= s.budgetLimit {
		return false
	}
	s.budgetUsed++
	return true
}

func (s *Scorer) llmScore(ctx context.Context, event *SignalEvent) (int, error) {
	if s.llmClient == nil {
		return 0, fmt.Errorf("no LLM client")
	}
	resp, err := s.llmClient.Complete(ctx, ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: llmScoringPrompt},
			{Role: "user", Content: event.Content},
		},
		MaxTokens:   16,
		Temperature: 0,
	})
	if err != nil {
		return 0, err
	}
	return parseScoreResponse(resp.Content), nil
}

func parseScoreResponse(raw string) int {
	trimmed := strings.TrimSpace(raw)
	n, err := strconv.Atoi(trimmed)
	if err != nil {
		return 50
	}
	return clamp(n, 0, 100)
}

const llmScoringPrompt = `Rate the urgency of this message 0-100.
0 = spam/noise, 50 = normal work message, 80 = urgent, 100 = critical incident.
Reply with ONLY a number.`

// keywordScore applies keyword pattern matching, ported from AttentionGate.
func keywordScore(content string) int {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return 0
	}
	lower := strings.ToLower(trimmed)
	score := 20 // base

	if containsAny(lower, summarizePatterns) {
		score = max(score, 40)
	}
	if containsAny(lower, queuePatterns) {
		score = max(score, 60)
	}

	urgentHits := countMatches(lower, urgentPatterns)
	if urgentHits > 0 {
		score = max(score, 80)
	}
	if hasExclamationBurst(lower) {
		score = max(score, 80)
		urgentHits++
	}

	escalateHits := countMatches(lower, escalatePatterns)
	if escalateHits > 0 || urgentHits >= 2 {
		score = max(score, 90)
	}
	if escalateHits+urgentHits >= 3 {
		score = 100
	}
	return score
}

var summarizePatterns = []string{
	"please", "pls", "review", "check", "look at", "take a look",
	"can you", "could you", "help", "请", "帮忙", "看一下", "看看", "麻烦",
}

var queuePatterns = []string{
	"today", "tonight", "this week", "before ", "follow up", "deadline",
	"eod", "尽快", "今天", "今晚", "本周", "截止", "稍后",
}

var urgentPatterns = []string{
	"紧急", "urgent", "asap", "deadline",
	"立刻", "马上", "immediately",
	"出错", "报错", "error", "失败", "failed", "故障",
	"挂了", "崩了", "down", "宕机",
	"blocked", "阻塞",
}

var escalatePatterns = []string{
	"p0", "sev0", "sev1", "incident", "outage",
	"生产事故", "生产故障", "宕机", "崩了",
}

func containsAny(lower string, patterns []string) bool {
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			return true
		}
	}
	return false
}

func countMatches(lower string, patterns []string) int {
	count := 0
	for _, p := range patterns {
		if strings.Contains(lower, p) {
			count++
		}
	}
	return count
}

func hasExclamationBurst(lower string) bool {
	exclamations := strings.Count(lower, "!") + strings.Count(lower, "！")
	return exclamations >= 3 && utf8.RuneCountInString(lower) < 50
}

func clamp(v, lo, hi int) int {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
