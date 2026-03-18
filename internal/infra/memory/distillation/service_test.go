package distillation

import (
	"context"
	"testing"
	"time"

	"alex/internal/infra/memory"
)

type mockMemoryEngine struct {
	dailyContent string
	err          error
}

func (m *mockMemoryEngine) EnsureSchema(context.Context) error { return nil }
func (m *mockMemoryEngine) AppendDaily(context.Context, string, memory.DailyEntry) (string, error) {
	return "", nil
}
func (m *mockMemoryEngine) Search(context.Context, string, string, int, float64) ([]memory.SearchHit, error) {
	return nil, nil
}
func (m *mockMemoryEngine) Related(context.Context, string, string, int, int, int) ([]memory.RelatedHit, error) {
	return nil, nil
}
func (m *mockMemoryEngine) GetLines(context.Context, string, string, int, int) (string, error) {
	return "", nil
}
func (m *mockMemoryEngine) LoadDaily(_ context.Context, _ string, _ time.Time) (string, error) {
	return m.dailyContent, m.err
}
func (m *mockMemoryEngine) LoadLongTerm(context.Context, string) (string, error) { return "", nil }
func (m *mockMemoryEngine) LoadIdentity(context.Context, string, string, string) (string, string, error) {
	return "", "", nil
}
func (m *mockMemoryEngine) ListDailyEntries(context.Context, string) ([]memory.DailySnapshot, error) {
	return nil, nil
}
func (m *mockMemoryEngine) SavePredictions(context.Context, string, []string) error   { return nil }
func (m *mockMemoryEngine) LoadPredictions(context.Context, string) ([]string, error) { return nil, nil }

func TestServiceRunDaily(t *testing.T) {
	tests := []struct {
		name         string
		dailyContent string
		llmResponse  string
		wantErr      bool
	}{
		{
			name:         "extracts facts from daily content",
			dailyContent: "We decided to use YAML for configs.",
			llmResponse:  `[{"content":"YAML for configs","category":"decision","confidence":0.9}]`,
		},
		{
			name:         "empty content skips extraction",
			dailyContent: "",
			llmResponse:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dir := t.TempDir()
			client := &mockLLMClient{response: tt.llmResponse, model: "test"}
			now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
			nowFn := func() time.Time { return now }

			svc := NewService(
				&mockMemoryEngine{dailyContent: tt.dailyContent},
				NewExtractor(client, 4096, nowFn),
				NewPatternAnalyzer(client, nowFn),
				NewStore(dir),
				nowFn,
			)

			err := svc.RunDaily(context.Background(), "user1")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}

func TestServiceRunWeekly(t *testing.T) {
	dir := t.TempDir()
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }
	store := NewStore(dir)
	ctx := context.Background()

	// Seed some daily extractions
	for i := 0; i < 3; i++ {
		date := now.AddDate(0, 0, -i).Format("2006-01-02")
		_ = store.SaveDailyExtraction(ctx, &DailyExtraction{
			Date:  date,
			Facts: []ExtractedFact{{ID: date, Category: "fact"}},
		})
	}

	client := &mockLLMClient{
		response: `[{"description":"recurring pattern","category":"preference","evidence":[],"confidence":0.85}]`,
		model:    "test",
	}

	svc := NewService(
		&mockMemoryEngine{},
		NewExtractor(client, 4096, nowFn),
		NewPatternAnalyzer(client, nowFn),
		store,
		nowFn,
	)

	if err := svc.RunWeekly(ctx, "user1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	weekStart := now.AddDate(0, 0, -7).Format("2006-01-02")
	patterns, err := store.LoadWeeklyPatterns(ctx, weekStart)
	if err != nil {
		t.Fatalf("load patterns: %v", err)
	}
	if len(patterns) != 1 {
		t.Errorf("got %d patterns, want 1", len(patterns))
	}
}
