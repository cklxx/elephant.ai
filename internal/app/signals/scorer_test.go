package signals

import (
	"context"
	"fmt"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
)

type stubLLM struct {
	response string
	err      error
	calls    int
}

func (s *stubLLM) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	s.calls++
	if s.err != nil {
		return nil, s.err
	}
	return &ports.CompletionResponse{Content: s.response}, nil
}

func (s *stubLLM) Model() string { return "test" }

func TestKeywordScore(t *testing.T) {
	tests := []struct {
		content string
		want    int
	}{
		{"", 0},
		{"hello world", 20},
		{"please review this PR", 40},
		{"need this today before eod", 60},
		{"urgent: server is down", 90},
		{"p0 incident outage!!!", 100},
		{"紧急 error blocked", 100},
		{"help!!!!!!", 80},
		{"sev0 production outage", 90},
	}
	for _, tt := range tests {
		t.Run(tt.content, func(t *testing.T) {
			got := keywordScore(tt.content)
			if got != tt.want {
				t.Errorf("keywordScore(%q) = %d, want %d", tt.content, got, tt.want)
			}
		})
	}
}

func TestScorerScore(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name       string
		content    string
		llmResp    string
		llmErr     error
		budget     int
		wantScore  int
		wantCalls  int
	}{
		{
			name:      "low score skips LLM",
			content:   "hello",
			budget:    10,
			wantScore: 20,
			wantCalls: 0,
		},
		{
			name:      "ambiguous uses LLM",
			content:   "please review this",
			llmResp:   "55",
			budget:    10,
			wantScore: 55,
			wantCalls: 1,
		},
		{
			name:      "LLM error falls back to keyword",
			content:   "please check this",
			llmErr:    fmt.Errorf("timeout"),
			budget:    10,
			wantScore: 40,
			wantCalls: 1,
		},
		{
			name:      "high score skips LLM",
			content:   "p0 incident",
			budget:    10,
			wantScore: 90,
			wantCalls: 0,
		},
		{
			name:      "budget exhausted skips LLM",
			content:   "please review",
			budget:    0,
			wantScore: 40,
			wantCalls: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubLLM{response: tt.llmResp, err: tt.llmErr}
			scorer := NewScorer(client, tt.budget, func() time.Time { return now })
			event := &SignalEvent{Content: tt.content}
			scorer.Score(context.Background(), event)
			if event.Score != tt.wantScore {
				t.Errorf("score = %d, want %d", event.Score, tt.wantScore)
			}
			if client.calls != tt.wantCalls {
				t.Errorf("LLM calls = %d, want %d", client.calls, tt.wantCalls)
			}
		})
	}
}

func TestScorerBudgetReset(t *testing.T) {
	now := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	current := now
	client := &stubLLM{response: "55"}
	scorer := NewScorer(client, 1, func() time.Time { return current })

	event := &SignalEvent{Content: "please review"}
	scorer.Score(context.Background(), event)
	if client.calls != 1 {
		t.Fatalf("expected 1 call, got %d", client.calls)
	}

	// Second call within same hour: budget exhausted.
	event2 := &SignalEvent{Content: "please check"}
	scorer.Score(context.Background(), event2)
	if client.calls != 1 {
		t.Fatalf("expected still 1 call, got %d", client.calls)
	}

	// Advance past budget reset.
	current = now.Add(2 * time.Hour)
	event3 := &SignalEvent{Content: "please look at"}
	scorer.Score(context.Background(), event3)
	if client.calls != 2 {
		t.Fatalf("expected 2 calls after reset, got %d", client.calls)
	}
}
