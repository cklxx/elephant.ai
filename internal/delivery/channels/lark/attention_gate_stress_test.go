package lark

import (
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestConcurrentAttentionGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 100

	gate := NewAttentionGate(AttentionGateConfig{
		Enabled:        true,
		UrgentKeywords: []string{"urgent", "ASAP", "blocked"},
		BudgetWindow:   10 * time.Minute,
		BudgetMax:      50,
	})

	messages := []string{
		"just a normal message",
		"this is urgent please help",
		"ASAP fix the deployment",
		"everything is fine",
		"the service is down!!!",
		"blocked by dependency",
		"",
		"routine update on progress",
		"error in production 故障",
		"nothing special here",
	}

	chatIDs := make([]string, 10)
	for i := range chatIDs {
		chatIDs[i] = fmt.Sprintf("chat-%d", i)
	}

	var (
		wg             sync.WaitGroup
		classifyPanics atomic.Int32
		dispatchPanics atomic.Int32
		classifyDone   atomic.Int32
		dispatchDone   atomic.Int32
	)

	// Phase 1: concurrent ClassifyUrgency
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					classifyPanics.Add(1)
					t.Errorf("ClassifyUrgency panicked: %v", r)
				}
			}()
			msg := messages[idx%len(messages)]
			level := gate.ClassifyUrgency(msg)
			if msg == "this is urgent please help" && level != UrgencyHigh {
				t.Errorf("expected UrgencyHigh for urgent message, got %d", level)
			}
			classifyDone.Add(1)
		}(i)
	}
	wg.Wait()

	// Phase 2: concurrent RecordDispatch
	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func(idx int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					dispatchPanics.Add(1)
					t.Errorf("RecordDispatch panicked: %v", r)
				}
			}()
			chatID := chatIDs[idx%len(chatIDs)]
			_ = gate.RecordDispatch(chatID, time.Now())
			dispatchDone.Add(1)
		}(i)
	}
	wg.Wait()

	t.Logf("ClassifyUrgency: done=%d panics=%d", classifyDone.Load(), classifyPanics.Load())
	t.Logf("RecordDispatch:  done=%d panics=%d", dispatchDone.Load(), dispatchPanics.Load())

	if p := classifyPanics.Load(); p > 0 {
		t.Fatalf("detected %d panics during concurrent ClassifyUrgency", p)
	}
	if p := dispatchPanics.Load(); p > 0 {
		t.Fatalf("detected %d panics during concurrent RecordDispatch", p)
	}
	if classifyDone.Load() != goroutines {
		t.Errorf("expected %d ClassifyUrgency completions, got %d", goroutines, classifyDone.Load())
	}
	if dispatchDone.Load() != goroutines {
		t.Errorf("expected %d RecordDispatch completions, got %d", goroutines, dispatchDone.Load())
	}
}
