package lark

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

// TestAttentionGate_ConcurrentClassifyUrgency verifies ClassifyUrgency is
// safe under concurrent access. The keyword list is read-only after init,
// so this mainly ensures no unexpected shared-state races.
func TestAttentionGate_ConcurrentClassifyUrgency(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"P0", "hotfix", "紧急"},
	})
	const goroutines = 100
	var wg sync.WaitGroup
	barrier := make(chan struct{})

	messages := []string{
		"deploy the service",
		"P0 incident",
		"hotfix needed now",
		"just a question",
		"紧急修复",
		"",
		"routine update",
	}

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			msg := messages[i%len(messages)]
			gate.ClassifyUrgency(msg)
		}(i)
	}

	close(barrier)
	wg.Wait()
}

// TestAttentionGate_ConcurrentRecordDispatch exercises RecordDispatch from
// many goroutines writing to the same chatID. Under -race this catches
// mutex or map races in the budget tracking code path.
func TestAttentionGate_ConcurrentRecordDispatch(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 10 * time.Minute,
		BudgetMax:    50,
	})
	const goroutines = 100
	var wg sync.WaitGroup
	barrier := make(chan struct{})

	now := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			// All goroutines dispatch to the same chatID.
			gate.RecordDispatch("chat-race", now.Add(time.Duration(i)*time.Millisecond))
		}(i)
	}

	close(barrier)
	wg.Wait()

	// Budget should be exhausted: 50 accepted, 50 rejected.
	gate.mu.Lock()
	b := gate.budgets["chat-race"]
	got := len(b.timestamps)
	gate.mu.Unlock()

	if got != 50 {
		t.Errorf("expected budget capped at 50, got %d dispatches recorded", got)
	}
}

// TestAttentionGate_ConcurrentRecordDispatch_MultiChat exercises RecordDispatch
// from many goroutines writing to different chatIDs simultaneously.
func TestAttentionGate_ConcurrentRecordDispatch_MultiChat(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 10 * time.Minute,
		BudgetMax:    5,
	})
	const goroutines = 100
	var wg sync.WaitGroup
	barrier := make(chan struct{})

	now := time.Now()

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			chatID := fmt.Sprintf("chat-%d", i%10)
			gate.RecordDispatch(chatID, now)
		}(i)
	}

	close(barrier)
	wg.Wait()
}

// TestAttentionGate_ConcurrentBudgetGCRace triggers budget GC from many
// goroutines concurrently by pre-filling the budget map above the GC
// threshold, then dispatching from many goroutines simultaneously.
// Under -race this catches races between GC iteration and map writes.
func TestAttentionGate_ConcurrentBudgetGCRace(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:      true,
		BudgetWindow: 5 * time.Minute,
		BudgetMax:    10,
	})

	t0 := time.Now()

	// Pre-fill beyond GC threshold with stale entries.
	gate.mu.Lock()
	for i := 0; i < budgetGCThreshold+20; i++ {
		gate.budgets[fmt.Sprintf("stale-%d", i)] = &chatBudget{
			timestamps: []time.Time{t0.Add(-10 * time.Minute)},
		}
	}
	gate.mu.Unlock()

	const goroutines = 100
	var wg sync.WaitGroup
	barrier := make(chan struct{})

	// Each goroutine dispatches to a unique chatID, which triggers GC
	// since we're above threshold. All goroutines hit GC concurrently.
	tNow := t0.Add(time.Minute)
	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			<-barrier
			gate.RecordDispatch(fmt.Sprintf("fresh-%d", i), tNow)
		}(i)
	}

	close(barrier)
	wg.Wait()

	// All stale entries should be GC'd.
	gate.mu.Lock()
	for id := range gate.budgets {
		if len(id) > 6 && id[:6] == "stale-" {
			t.Errorf("stale entry %q should have been GC'd", id)
		}
	}
	gate.mu.Unlock()
}

// TestAttentionGate_BlankKeywordFilter_ConcurrentClassify verifies that
// after blank keywords are filtered during init, concurrent classification
// calls don't produce false UrgencyHigh results.
func TestAttentionGate_BlankKeywordFilter_ConcurrentClassify(t *testing.T) {
	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"", "  ", "\t", "  \n  "},
	})

	const goroutines = 100
	var wg sync.WaitGroup
	barrier := make(chan struct{})

	for i := 0; i < goroutines; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-barrier
			level := gate.ClassifyUrgency("just a normal message")
			if level == UrgencyHigh {
				t.Errorf("blank keywords should never produce UrgencyHigh")
			}
		}()
	}

	close(barrier)
	wg.Wait()
}
