package decision

import (
	"context"
	"testing"
	"time"

	"alex/internal/infra/memory/distillation"
)

func newTestEngine(t *testing.T) (*Engine, func() time.Time) {
	t.Helper()
	now := time.Date(2026, 3, 18, 12, 0, 0, 0, time.UTC)
	nowFn := func() time.Time { return now }

	ds, err := NewStore("")
	if err != nil {
		t.Fatalf("decision store: %v", err)
	}
	ps, err := NewPatternStore("")
	if err != nil {
		t.Fatalf("pattern store: %v", err)
	}
	al, err := NewAuditLog("", nowFn)
	if err != nil {
		t.Fatalf("audit log: %v", err)
	}
	return NewEngine(ds, ps, al, nowFn), nowFn
}

func TestEngineObserve(t *testing.T) {
	eng, _ := newTestEngine(t)

	d := &Decision{
		ID: "d1", Title: "use Go", Description: "backend language choice",
		DecidedBy: "ckl", Outcome: "Go selected", Tags: []string{"tech"},
	}
	if err := eng.Observe(context.Background(), d); err != nil {
		t.Fatalf("observe: %v", err)
	}

	got := eng.decisions.Get("d1")
	if got == nil {
		t.Fatal("decision should be stored")
	}
}

func TestEngineShouldAutoAct(t *testing.T) {
	eng, _ := newTestEngine(t)
	ctx := context.Background()

	_ = eng.patterns.Save(&Pattern{
		ID: "p1", Category: "escalation", Condition: "high-risk",
		Action: "notify", Confidence: 0.95,
	})

	tests := []struct {
		name      string
		enabled   bool
		category  string
		condition string
		wantAct   bool
	}{
		{name: "auto-act disabled", enabled: false, category: "escalation", condition: "high-risk", wantAct: false},
		{name: "auto-act enabled matching", enabled: true, category: "escalation", condition: "high-risk", wantAct: true},
		{name: "no match", enabled: true, category: "approval", condition: "low-risk", wantAct: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			eng.SetAutoAct(tt.enabled)
			_, act := eng.ShouldAutoAct(ctx, tt.category, tt.condition)
			if act != tt.wantAct {
				t.Errorf("got autoAct=%v, want %v", act, tt.wantAct)
			}
		})
	}
}

func TestEngineConfidenceFloorEnforcement(t *testing.T) {
	eng, _ := newTestEngine(t)
	eng.SetAutoAct(true)

	_ = eng.patterns.Save(&Pattern{
		ID: "p1", Category: "escalation", Condition: "medium-risk",
		Action: "notify", Confidence: 0.85, // below floor
	})

	_, act := eng.ShouldAutoAct(context.Background(), "escalation", "medium-risk")
	if act {
		t.Error("should not auto-act below confidence floor")
	}
}

func TestEngineRecordCorrection(t *testing.T) {
	eng, _ := newTestEngine(t)

	_ = eng.patterns.Save(&Pattern{
		ID: "p1", Category: "escalation", Condition: "high-risk",
		Action: "notify", Confidence: 0.95, SampleCount: 5,
	})

	err := eng.RecordCorrection(context.Background(), "p1", "should have escalated to manager")
	if err != nil {
		t.Fatalf("correction: %v", err)
	}

	p := eng.patterns.Get("p1")
	if p.Confidence >= 0.95 {
		t.Errorf("confidence should decrease after correction, got %f", p.Confidence)
	}
	// 0.95 * 0.8 = 0.76
	if p.Confidence < 0.75 || p.Confidence > 0.77 {
		t.Errorf("expected confidence ~0.76, got %f", p.Confidence)
	}
}

func TestEngineRecordCorrectionNotFound(t *testing.T) {
	eng, _ := newTestEngine(t)
	err := eng.RecordCorrection(context.Background(), "nonexistent", "fix")
	if err == nil {
		t.Error("expected error for nonexistent pattern")
	}
}

func TestEngineLearnPatterns(t *testing.T) {
	eng, _ := newTestEngine(t)

	weeklyPatterns := []distillation.WeeklyPattern{
		{ID: "wp1", Description: "prefers Go", Category: "preference", Confidence: 0.85},
		{ID: "wp2", Description: "reviews before merge", Category: "process", Confidence: 0.92},
	}

	err := eng.LearnPatterns(context.Background(), weeklyPatterns)
	if err != nil {
		t.Fatalf("learn: %v", err)
	}

	patterns := eng.patterns.List()
	if len(patterns) != 2 {
		t.Errorf("got %d patterns, want 2", len(patterns))
	}
}

func TestEngineObserveUpdatesMatchingPattern(t *testing.T) {
	eng, _ := newTestEngine(t)

	_ = eng.patterns.Save(&Pattern{
		ID: "p1", Category: "tech", Condition: "backend language choice",
		Action: "auto", Confidence: 0.9, SampleCount: 3,
	})

	d := &Decision{
		ID: "d1", Title: "use Go", Description: "backend",
		DecidedBy: "ckl", Tags: []string{"tech"},
	}
	_ = eng.Observe(context.Background(), d)

	p := eng.patterns.Get("p1")
	if p.SampleCount != 4 {
		t.Errorf("sample count should increase, got %d", p.SampleCount)
	}
}
