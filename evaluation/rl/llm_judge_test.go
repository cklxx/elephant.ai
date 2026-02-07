package rl

import (
	"context"
	"fmt"
	"testing"
	"time"

	ports "alex/internal/domain/agent/ports"
)

// stubLLMClient implements portsllm.LLMClient for testing.
type stubLLMClient struct {
	response string
	err      error
}

func (s *stubLLMClient) Complete(_ context.Context, _ ports.CompletionRequest) (*ports.CompletionResponse, error) {
	if s.err != nil {
		return nil, s.err
	}
	return &ports.CompletionResponse{Content: s.response}, nil
}

func (s *stubLLMClient) Model() string { return "test-model" }

func TestLLMJudge_Score(t *testing.T) {
	client := &stubLLMClient{
		response: `{"score": 72.5, "reasoning": "Good trajectory with minor inefficiencies"}`,
	}
	judge := NewLLMJudge(client)

	traj := &RLTrajectory{
		ID:        "test-1",
		TaskID:    "task-1",
		AutoScore: 65,
		Grade:     "B",
		Steps: []TrajectoryStep{
			{StepIndex: 0, Thought: "Let me search", Action: "search(query)", Observation: "results found", Reward: 0.1},
			{StepIndex: 1, Action: "apply_fix()", Reward: 1.0},
		},
		Metadata: TrajectoryMeta{
			TotalSteps: 2,
			Duration:   30 * time.Second,
			TokensUsed: 1500,
			Outcome:    "completed",
			ToolsUsed:  []string{"search", "apply_fix"},
		},
	}

	score, err := judge.Score(context.Background(), traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 72.5 {
		t.Errorf("expected score 72.5, got %f", score)
	}
}

func TestLLMJudge_ScoreWithMarkdownWrappedJSON(t *testing.T) {
	client := &stubLLMClient{
		response: "```json\n{\"score\": 85, \"reasoning\": \"Excellent\"}\n```",
	}
	judge := NewLLMJudge(client)

	traj := &RLTrajectory{
		ID:        "test-2",
		TaskID:    "task-2",
		AutoScore: 70,
		Steps:     []TrajectoryStep{{StepIndex: 0, Action: "do_thing()"}},
		Metadata:  TrajectoryMeta{TotalSteps: 1, Outcome: "completed"},
	}

	score, err := judge.Score(context.Background(), traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 85 {
		t.Errorf("expected score 85, got %f", score)
	}
}

func TestLLMJudge_LLMError(t *testing.T) {
	client := &stubLLMClient{err: fmt.Errorf("rate limited")}
	judge := NewLLMJudge(client)

	traj := &RLTrajectory{ID: "test-3", Steps: []TrajectoryStep{}}
	_, err := judge.Score(context.Background(), traj)
	if err == nil {
		t.Fatal("expected error from LLM failure")
	}
	if got := err.Error(); got != "llm completion: rate limited" {
		t.Errorf("unexpected error message: %s", got)
	}
}

func TestLLMJudge_InvalidJSON(t *testing.T) {
	client := &stubLLMClient{response: "I think the score is about 75"}
	judge := NewLLMJudge(client)

	traj := &RLTrajectory{ID: "test-4", Steps: []TrajectoryStep{}}
	_, err := judge.Score(context.Background(), traj)
	if err == nil {
		t.Fatal("expected error from invalid JSON")
	}
}

func TestLLMJudge_ScoreOutOfRange(t *testing.T) {
	tests := []struct {
		name     string
		response string
	}{
		{"too_high", `{"score": 150, "reasoning": "amazing"}`},
		{"negative", `{"score": -10, "reasoning": "terrible"}`},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := &stubLLMClient{response: tt.response}
			judge := NewLLMJudge(client)

			traj := &RLTrajectory{ID: "test-range", Steps: []TrajectoryStep{}}
			_, err := judge.Score(context.Background(), traj)
			if err == nil {
				t.Fatal("expected error for out-of-range score")
			}
		})
	}
}

func TestLLMJudge_LongTrajectoryTruncated(t *testing.T) {
	client := &stubLLMClient{
		response: `{"score": 60, "reasoning": "ok"}`,
	}
	judge := NewLLMJudge(client)

	// Build trajectory with 30 steps — prompt should truncate to 20
	steps := make([]TrajectoryStep, 30)
	for i := range steps {
		steps[i] = TrajectoryStep{
			StepIndex:   i,
			Thought:     fmt.Sprintf("Step %d thought", i),
			Action:      fmt.Sprintf("action_%d()", i),
			Observation: fmt.Sprintf("result_%d", i),
			Reward:      0.1,
		}
	}

	traj := &RLTrajectory{
		ID:       "test-long",
		TaskID:   "long-task",
		Steps:    steps,
		Metadata: TrajectoryMeta{TotalSteps: 30, Outcome: "completed"},
	}

	score, err := judge.Score(context.Background(), traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if score != 60 {
		t.Errorf("expected score 60, got %f", score)
	}
}

func TestLLMJudge_IntegrationWithQualityGate(t *testing.T) {
	// Verify LLMJudge satisfies the Judge interface and works end-to-end
	// with the QualityGate
	client := &stubLLMClient{
		response: `{"score": 90, "reasoning": "excellent trajectory"}`,
	}
	judge := NewLLMJudge(client)

	cfg := DefaultQualityConfig()
	cfg.JudgeEnabled = true
	cfg.BorderlineLower = 55
	cfg.BorderlineUpper = 75

	gate := NewQualityGate(cfg, judge)
	ctx := context.Background()

	// Auto score 65 is in borderline [55, 75)
	// Judge returns 90, combined = (65 + 90) / 2 = 77.5 → Silver
	traj := &RLTrajectory{AutoScore: 65, Steps: []TrajectoryStep{}}
	tier, err := gate.Classify(ctx, traj)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if tier != TierSilver {
		t.Errorf("expected silver, got %s", tier)
	}
	if traj.JudgeScore == nil || *traj.JudgeScore != 90 {
		t.Errorf("expected judge score 90, got %v", traj.JudgeScore)
	}
}

func TestParseJudgeResponse(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    float64
		wantErr bool
	}{
		{"clean_json", `{"score": 75, "reasoning": "good"}`, 75, false},
		{"markdown_wrapped", "```json\n{\"score\": 80}\n```", 80, false},
		{"extra_text_before", "Here is my evaluation:\n{\"score\": 55, \"reasoning\": \"ok\"}", 55, false},
		{"zero_score", `{"score": 0, "reasoning": "terrible"}`, 0, false},
		{"max_score", `{"score": 100, "reasoning": "perfect"}`, 100, false},
		{"over_100", `{"score": 101}`, 0, true},
		{"negative", `{"score": -1}`, 0, true},
		{"no_json", "the score is 75", 0, true},
		{"empty", "", 0, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			score, err := parseJudgeResponse(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if score != tt.want {
				t.Errorf("expected %f, got %f", tt.want, score)
			}
		})
	}
}
