package leader_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/app/blocker"
	"alex/internal/delivery/channels/lark"
	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
	"alex/internal/shared/notification"
)

// --- mock notifier (implements notification.Notifier) ---

type stressNotifier struct{}

func (n *stressNotifier) Send(_ context.Context, _ notification.Target, _ string) error {
	return nil
}

// --- helpers ---

// seedStore creates n running tasks in the store so Radar.Scan has work.
func seedStore(t *testing.T, store *taskstore.LocalStore, n int) {
	t.Helper()
	ctx := context.Background()
	stale := time.Now().Add(-2 * time.Hour) // older than default 30-min threshold
	for i := 0; i < n; i++ {
		tk := &task.Task{
			TaskID:      fmt.Sprintf("task-%d", i),
			Status:      task.StatusRunning,
			Description: fmt.Sprintf("stress task %d", i),
			CreatedAt:   stale,
			UpdatedAt:   stale,
		}
		if err := store.Create(ctx, tk); err != nil {
			t.Fatalf("seed task %d: %v", i, err)
		}
	}
}

// ─────────────────────────────────────────────────────────
// 1. TestConcurrentScan — 50 goroutines calling Scan()
// ─────────────────────────────────────────────────────────

func TestConcurrentScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 50

	store := taskstore.New() // in-memory, no file path
	defer store.Close()
	seedStore(t, store, 20)

	radar := blocker.NewRadar(store, &stressNotifier{}, blocker.Config{
		Enabled:        true,
		StaleThreshold: 30 * time.Minute,
	})

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var (
		wg      sync.WaitGroup
		panics  atomic.Int32
		errors  atomic.Int32
		success atomic.Int32
	)

	wg.Add(goroutines)
	for i := 0; i < goroutines; i++ {
		go func() {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					panics.Add(1)
					t.Errorf("Scan panicked: %v", r)
				}
			}()
			result, err := radar.Scan(ctx)
			if err != nil {
				errors.Add(1)
				return
			}
			if result == nil {
				t.Error("Scan returned nil result without error")
				return
			}
			success.Add(1)
		}()
	}

	wg.Wait()

	t.Logf("Results: success=%d errors=%d panics=%d",
		success.Load(), errors.Load(), panics.Load())

	if p := panics.Load(); p > 0 {
		t.Fatalf("detected %d panics during concurrent Scan", p)
	}
	if s := success.Load(); s != goroutines {
		t.Errorf("expected %d successful scans, got %d", goroutines, s)
	}
}

// ─────────────────────────────────────────────────────────
// 2. TestConcurrentAttentionGate — 100 goroutines on
//    ClassifyUrgency + RecordDispatch
// ─────────────────────────────────────────────────────────

func TestConcurrentAttentionGate(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 100

	gate := lark.NewAttentionGate(lark.AttentionGateConfig{
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
			// Basic sanity: urgent keywords should yield High.
			if msg == "this is urgent please help" && level != lark.UrgencyHigh {
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

// ─────────────────────────────────────────────────────────
// 3. TestConcurrentTaskStore — parallel Create/Get/List
// ─────────────────────────────────────────────────────────

func TestConcurrentTaskStore(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const (
		writers   = 30
		readers   = 30
		listers   = 20
		tasksEach = 5
	)

	store := taskstore.New() // in-memory, no file path
	defer store.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	var (
		wg           sync.WaitGroup
		writePanics  atomic.Int32
		readPanics   atomic.Int32
		listPanics   atomic.Int32
		writeSuccess atomic.Int32
		readSuccess  atomic.Int32
		listSuccess  atomic.Int32
	)

	// Writers: each goroutine creates tasksEach tasks.
	wg.Add(writers)
	for w := 0; w < writers; w++ {
		go func(workerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					writePanics.Add(1)
					t.Errorf("writer %d panicked: %v", workerID, r)
				}
			}()
			for j := 0; j < tasksEach; j++ {
				tk := &task.Task{
					TaskID:      fmt.Sprintf("w%d-t%d", workerID, j),
					SessionID:   fmt.Sprintf("session-%d", workerID%5),
					Status:      task.StatusRunning,
					Description: fmt.Sprintf("worker %d task %d", workerID, j),
					CreatedAt:   time.Now(),
					UpdatedAt:   time.Now(),
				}
				if err := store.Create(ctx, tk); err != nil {
					t.Errorf("writer %d task %d: %v", workerID, j, err)
					continue
				}
				writeSuccess.Add(1)
			}
		}(w)
	}

	// Wait for writers to finish so readers have data.
	wg.Wait()

	totalCreated := writeSuccess.Load()
	t.Logf("Writers done: %d tasks created (panics=%d)", totalCreated, writePanics.Load())

	// Readers + Listers run concurrently.
	wg.Add(readers + listers)

	// Readers: Get random existing tasks.
	for r := 0; r < readers; r++ {
		go func(readerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					readPanics.Add(1)
					t.Errorf("reader %d panicked: %v", readerID, r)
				}
			}()
			for j := 0; j < tasksEach; j++ {
				// Read a task from a different worker to exercise cross-goroutine access.
				workerID := (readerID + j) % writers
				taskID := fmt.Sprintf("w%d-t%d", workerID, j%tasksEach)
				got, err := store.Get(ctx, taskID)
				if err != nil {
					// Task may not exist if its writer failed.
					continue
				}
				if got.TaskID != taskID {
					t.Errorf("reader %d: expected task %s, got %s", readerID, taskID, got.TaskID)
				}
				readSuccess.Add(1)
			}
		}(r)
	}

	// Listers: call List and ListActive concurrently.
	for l := 0; l < listers; l++ {
		go func(listerID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					listPanics.Add(1)
					t.Errorf("lister %d panicked: %v", listerID, r)
				}
			}()
			if listerID%2 == 0 {
				tasks, total, err := store.List(ctx, 50, 0)
				if err != nil {
					t.Errorf("lister %d List: %v", listerID, err)
					return
				}
				if total == 0 || len(tasks) == 0 {
					t.Errorf("lister %d: expected tasks, got total=%d len=%d", listerID, total, len(tasks))
					return
				}
			} else {
				tasks, err := store.ListActive(ctx)
				if err != nil {
					t.Errorf("lister %d ListActive: %v", listerID, err)
					return
				}
				if len(tasks) == 0 {
					t.Errorf("lister %d: ListActive returned 0 tasks", listerID)
					return
				}
			}
			listSuccess.Add(1)
		}(l)
	}

	wg.Wait()

	t.Logf("Readers: success=%d panics=%d", readSuccess.Load(), readPanics.Load())
	t.Logf("Listers: success=%d panics=%d", listSuccess.Load(), listPanics.Load())

	if p := writePanics.Load(); p > 0 {
		t.Fatalf("detected %d panics in writers", p)
	}
	if p := readPanics.Load(); p > 0 {
		t.Fatalf("detected %d panics in readers", p)
	}
	if p := listPanics.Load(); p > 0 {
		t.Fatalf("detected %d panics in listers", p)
	}

	// Verify final count.
	_, total, err := store.List(ctx, 1, 0)
	if err != nil {
		t.Fatalf("final List: %v", err)
	}
	if int32(total) != totalCreated {
		t.Errorf("expected %d total tasks, got %d", totalCreated, total)
	}
}
