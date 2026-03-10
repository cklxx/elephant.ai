package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	agentstorage "alex/internal/domain/agent/ports/storage"
)

func TestIsUsageCommand(t *testing.T) {
	g := &Gateway{}
	tests := []struct {
		input string
		want  bool
	}{
		{"/usage", true},
		{"/stats", true},
		{"/Usage", true},
		{"/STATS", true},
		{"/usage today", true},
		{"/stats week", true},
		{"/usages", false},
		{"usage", false},
		{"/model", false},
		{"/tasks", false},
	}
	for _, tt := range tests {
		if got := g.isUsageCommand(tt.input); got != tt.want {
			t.Errorf("isUsageCommand(%q) = %v, want %v", tt.input, got, tt.want)
		}
	}
}

func TestStartOfWeek(t *testing.T) {
	tests := []struct {
		name string
		t    time.Time
		want time.Time
	}{
		{
			name: "Monday stays",
			t:    time.Date(2026, 3, 9, 14, 30, 0, 0, time.UTC),
			want: time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Wednesday goes back to Monday",
			t:    time.Date(2026, 3, 11, 10, 0, 0, 0, time.UTC),
			want: time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC),
		},
		{
			name: "Sunday goes back to Monday",
			t:    time.Date(2026, 3, 15, 23, 59, 0, 0, time.UTC),
			want: time.Date(2026, 3, 9, 0, 0, 0, 0, time.UTC),
		},
	}
	for _, tt := range tests {
		got := startOfWeek(tt.t)
		if !got.Equal(tt.want) {
			t.Errorf("%s: startOfWeek(%v) = %v, want %v", tt.name, tt.t, got, tt.want)
		}
	}
}

func TestFormatCostSummaryBlock(t *testing.T) {
	s := &agentstorage.CostSummary{
		TotalCost:    0.1234,
		InputTokens:  5000,
		OutputTokens: 3000,
		TotalTokens:  8000,
		RequestCount: 10,
		ByModel: map[string]float64{
			"gpt-4": 0.1,
		},
	}
	result := formatCostSummaryBlock(s)
	if !strings.Contains(result, "8.0k") {
		t.Errorf("expected total tokens 8.0k, got: %s", result)
	}
	if !strings.Contains(result, "$0.1234") {
		t.Errorf("expected cost $0.1234, got: %s", result)
	}
	if !strings.Contains(result, "10") {
		t.Errorf("expected request count 10, got: %s", result)
	}
	if !strings.Contains(result, "gpt-4") {
		t.Errorf("expected model name gpt-4, got: %s", result)
	}
}

func TestFormatCostSummaryBlockNil(t *testing.T) {
	if got := formatCostSummaryBlock(nil); got != "" {
		t.Errorf("expected empty for nil, got: %q", got)
	}
}

// mockCostTracker implements CostTrackerReader for testing.
type mockCostTracker struct {
	daily  *agentstorage.CostSummary
	weekly *agentstorage.CostSummary
}

func (m *mockCostTracker) GetDailyCost(_ context.Context, _ time.Time) (*agentstorage.CostSummary, error) {
	return m.daily, nil
}

func (m *mockCostTracker) GetDateRangeCost(_ context.Context, _, _ time.Time) (*agentstorage.CostSummary, error) {
	return m.weekly, nil
}

func TestFormatCostSummary_WithData(t *testing.T) {
	ct := &mockCostTracker{
		daily: &agentstorage.CostSummary{
			TotalCost:    0.05,
			InputTokens:  2000,
			OutputTokens: 1000,
			TotalTokens:  3000,
			RequestCount: 5,
		},
		weekly: &agentstorage.CostSummary{
			TotalCost:    0.25,
			InputTokens:  10000,
			OutputTokens: 5000,
			TotalTokens:  15000,
			RequestCount: 20,
		},
	}
	g := &Gateway{costTracker: ct}
	now := time.Date(2026, 3, 10, 12, 0, 0, 0, time.UTC)
	result := g.formatCostSummary(context.Background(), now)

	if !strings.Contains(result, "今日用量") {
		t.Errorf("expected daily section, got: %s", result)
	}
	if !strings.Contains(result, "本周累计") {
		t.Errorf("expected weekly section, got: %s", result)
	}
	if !strings.Contains(result, "3.0k") {
		t.Errorf("expected daily tokens, got: %s", result)
	}
	if !strings.Contains(result, "15.0k") {
		t.Errorf("expected weekly tokens, got: %s", result)
	}
}

func TestFormatCostSummary_NilTracker(t *testing.T) {
	g := &Gateway{costTracker: nil}
	result := g.formatCostSummary(context.Background(), time.Now())
	if result != "" {
		t.Errorf("expected empty for nil tracker, got: %q", result)
	}
}

func TestFormatActiveTaskSummary_NoStore(t *testing.T) {
	g := &Gateway{taskStore: nil}
	result := g.formatActiveTaskSummary(context.Background(), "chat1")
	if result != "" {
		t.Errorf("expected empty for nil store, got: %q", result)
	}
}

func TestFormatTopTasks_NoStore(t *testing.T) {
	g := &Gateway{taskStore: nil}
	result := g.formatTopTasks(context.Background(), "chat1")
	if result != "" {
		t.Errorf("expected empty for nil store, got: %q", result)
	}
}

func TestBuildUsageReply_MinimalGateway(t *testing.T) {
	g := &Gateway{}
	msg := &incomingMessage{chatID: "chat1"}
	result := g.buildUsageReply(context.Background(), msg)
	if !strings.Contains(result, "AI 用量统计") {
		t.Errorf("expected header, got: %s", result)
	}
	if !strings.Contains(result, "当前模型: 未配置") {
		t.Errorf("expected unconfigured model, got: %s", result)
	}
	if !strings.Contains(result, "/tasks") {
		t.Errorf("expected footer with /tasks hint, got: %s", result)
	}
}

func TestFormatTopTasks_WithData(t *testing.T) {
	store := NewTaskMemoryStore(0, 0)
	ctx := context.Background()
	_ = store.EnsureSchema(ctx)

	// Insert tasks with token data
	for _, rec := range []TaskRecord{
		{TaskID: "bg-aaa", ChatID: "chat1", Status: taskStatusCompleted, Description: "Task A", TokensUsed: 50000},
		{TaskID: "bg-bbb", ChatID: "chat1", Status: taskStatusCompleted, Description: "Task B", TokensUsed: 30000},
		{TaskID: "bg-ccc", ChatID: "chat1", Status: taskStatusCompleted, Description: "Task C", TokensUsed: 10000},
		{TaskID: "bg-ddd", ChatID: "chat1", Status: taskStatusCompleted, Description: "Task D", TokensUsed: 5000},
	} {
		_ = store.SaveTask(ctx, rec)
	}

	g := &Gateway{taskStore: store}
	result := g.formatTopTasks(ctx, "chat1")

	if !strings.Contains(result, "Top 3") {
		t.Errorf("expected top 3 header, got: %s", result)
	}
	if !strings.Contains(result, "Task A") {
		t.Errorf("expected Task A (highest), got: %s", result)
	}
	if !strings.Contains(result, "Task B") {
		t.Errorf("expected Task B, got: %s", result)
	}
	if !strings.Contains(result, "Task C") {
		t.Errorf("expected Task C, got: %s", result)
	}
	if strings.Contains(result, "Task D") {
		t.Errorf("should not include Task D (4th), got: %s", result)
	}
}
