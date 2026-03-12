package leader

import (
	"strings"
	"sync"
	"testing"
	"time"
)

func TestDecisionHistoryAdd(t *testing.T) {
	h := &DecisionHistory{}
	if h.Len() != 0 {
		t.Fatal("new history should be empty")
	}

	h.Add(DecisionRecord{Attempt: 1, Action: "INJECT", Argument: "keep going"})
	h.Add(DecisionRecord{Attempt: 2, Action: "INJECT", Argument: "try harder"})

	if h.Len() != 2 {
		t.Fatalf("expected 2 records, got %d", h.Len())
	}
}

func TestDecisionHistoryLast(t *testing.T) {
	h := &DecisionHistory{}
	for i := 1; i <= 5; i++ {
		h.Add(DecisionRecord{Attempt: i, Action: "INJECT", Argument: "msg"})
	}

	last3 := h.Last(3)
	if len(last3) != 3 {
		t.Fatalf("expected 3, got %d", len(last3))
	}
	if last3[0].Attempt != 3 || last3[2].Attempt != 5 {
		t.Errorf("expected attempts [3,4,5], got [%d,%d,%d]", last3[0].Attempt, last3[1].Attempt, last3[2].Attempt)
	}

	// Request more than available.
	all := h.Last(100)
	if len(all) != 5 {
		t.Fatalf("expected 5, got %d", len(all))
	}

	// Zero or negative.
	if h.Last(0) != nil {
		t.Error("Last(0) should return nil")
	}
	if h.Last(-1) != nil {
		t.Error("Last(-1) should return nil")
	}
}

func TestDecisionHistoryRecordOutcome(t *testing.T) {
	h := &DecisionHistory{}
	h.Add(DecisionRecord{Attempt: 1, Action: "INJECT", Argument: "try"})

	h.RecordOutcome("still_stalled")
	records := h.Last(1)
	if records[0].Outcome != "still_stalled" {
		t.Errorf("expected still_stalled, got %q", records[0].Outcome)
	}

	// Second call should not overwrite.
	h.RecordOutcome("recovered")
	records = h.Last(1)
	if records[0].Outcome != "still_stalled" {
		t.Errorf("outcome should not be overwritten, got %q", records[0].Outcome)
	}
}

func TestDecisionHistoryRecordOutcomeEmpty(t *testing.T) {
	h := &DecisionHistory{}
	// Should not panic on empty history.
	h.RecordOutcome("recovered")
}

func TestDecisionHistorySummaryForPrompt(t *testing.T) {
	h := &DecisionHistory{}

	// Empty history returns empty string.
	if s := h.SummaryForPrompt(3); s != "" {
		t.Errorf("expected empty summary for empty history, got %q", s)
	}

	h.Add(DecisionRecord{Attempt: 1, Action: "INJECT", Argument: "keep going", Outcome: "still_stalled"})
	h.Add(DecisionRecord{Attempt: 2, Action: "INJECT", Argument: "try different"})

	summary := h.SummaryForPrompt(3)
	if !strings.Contains(summary, "Previous decisions") {
		t.Error("summary should contain header")
	}
	if !strings.Contains(summary, "Attempt 1: INJECT keep going → still_stalled") {
		t.Errorf("summary missing attempt 1 detail: %s", summary)
	}
	if !strings.Contains(summary, "Attempt 2: INJECT try different → pending") {
		t.Errorf("summary missing attempt 2 detail: %s", summary)
	}
}

func TestDecisionHistoryStoreConcurrent(t *testing.T) {
	store := newDecisionHistoryStore()
	var wg sync.WaitGroup

	for i := 0; i < 50; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			h := store.Get("sess-1")
			h.Add(DecisionRecord{Attempt: i, Action: "INJECT", Argument: "msg"})
		}(i)
	}
	wg.Wait()

	h := store.Get("sess-1")
	if h.Len() != 50 {
		t.Errorf("expected 50 records, got %d", h.Len())
	}
}

func TestDecisionHistoryStoreDelete(t *testing.T) {
	store := newDecisionHistoryStore()
	h := store.Get("sess-1")
	h.Add(DecisionRecord{Attempt: 1, Action: "INJECT"})

	store.Delete("sess-1")

	// After delete, Get returns a fresh empty history.
	h2 := store.Get("sess-1")
	if h2.Len() != 0 {
		t.Error("expected empty history after delete")
	}
}

func TestBuildStallPromptWithHistory(t *testing.T) {
	h := &DecisionHistory{}
	h.Add(DecisionRecord{Attempt: 1, Action: "INJECT", Argument: "keep going", Outcome: "still_stalled"})

	prompt := buildStallPrompt("s1", "backend", "fix bug", 90*time.Second, "stalled", 2, h, "", "", "", 0)

	// Should contain basic fields.
	for _, expected := range []string{"s1", "backend", "fix bug", "stalled", "1m30s", "2 of 3"} {
		if !strings.Contains(prompt, expected) {
			t.Errorf("prompt missing %q", expected)
		}
	}

	// Should contain history.
	if !strings.Contains(prompt, "Previous decisions") {
		t.Error("prompt should contain decision history")
	}
	if !strings.Contains(prompt, "Attempt 1: INJECT keep going → still_stalled") {
		t.Errorf("prompt missing history detail")
	}

	// Should contain the adaptive instruction.
	if !strings.Contains(prompt, "previous INJECT attempts failed") {
		t.Error("prompt should include adaptive instruction")
	}
}

func TestBuildStallPromptNilHistory(t *testing.T) {
	prompt := buildStallPrompt("s1", "backend", "fix bug", 90*time.Second, "stalled", 1, nil, "", "", "", 0)
	if !strings.Contains(prompt, "s1") {
		t.Error("prompt should still work with nil history")
	}
	if strings.Contains(prompt, "Previous decisions") {
		t.Error("prompt should not contain history section when nil")
	}
}

func TestBuildStallPromptEmptyHistory(t *testing.T) {
	h := &DecisionHistory{}
	prompt := buildStallPrompt("s1", "backend", "fix bug", 90*time.Second, "stalled", 1, h, "", "", "", 0)
	if strings.Contains(prompt, "Previous decisions") {
		t.Error("prompt should not contain history section when empty")
	}
}
