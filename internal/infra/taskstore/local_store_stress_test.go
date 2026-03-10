package taskstore

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/domain/task"
)

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

	store := New()
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

	wg.Wait()

	totalCreated := writeSuccess.Load()
	t.Logf("Writers done: %d tasks created (panics=%d)", totalCreated, writePanics.Load())

	// Readers + Listers run concurrently.
	wg.Add(readers + listers)

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
				workerID := (readerID + j) % writers
				taskID := fmt.Sprintf("w%d-t%d", workerID, j%tasksEach)
				got, err := store.Get(ctx, taskID)
				if err != nil {
					continue
				}
				if got.TaskID != taskID {
					t.Errorf("reader %d: expected task %s, got %s", readerID, taskID, got.TaskID)
				}
				readSuccess.Add(1)
			}
		}(r)
	}

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

	_, total, err := store.List(ctx, 1, 0)
	if err != nil {
		t.Fatalf("final List: %v", err)
	}
	if int32(total) != totalCreated {
		t.Errorf("expected %d total tasks, got %d", totalCreated, total)
	}
}
