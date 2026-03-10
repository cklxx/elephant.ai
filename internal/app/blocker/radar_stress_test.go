package blocker

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/task"
	"alex/internal/infra/taskstore"
	"alex/internal/shared/notification"
)

type stressNotifier struct{}

func (n *stressNotifier) Send(_ context.Context, _ notification.Target, _ string) error {
	return nil
}

func seedStore(t *testing.T, store *taskstore.LocalStore, n int) {
	t.Helper()
	ctx := context.Background()
	stale := time.Now().Add(-2 * time.Hour)
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

func TestConcurrentScan(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	const goroutines = 50

	store := taskstore.New()
	defer store.Close()
	seedStore(t, store, 20)

	radar := NewRadar(store, &stressNotifier{}, Config{
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
