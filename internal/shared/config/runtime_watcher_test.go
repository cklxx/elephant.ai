package config

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"
)

func TestRuntimeConfigWatcherStopConcurrent(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	path := filepath.Join(dir, "runtime.yaml")
	if err := os.WriteFile(path, []byte("runtime: {}\n"), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cache, err := NewRuntimeConfigCache(func(context.Context) (RuntimeConfig, Metadata, error) {
		return RuntimeConfig{}, Metadata{}, nil
	})
	if err != nil {
		t.Fatalf("new runtime config cache: %v", err)
	}

	watcher, err := NewRuntimeConfigWatcher(path, cache)
	if err != nil {
		t.Fatalf("new runtime config watcher: %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := watcher.Start(ctx); err != nil {
		t.Fatalf("start watcher: %v", err)
	}

	time.Sleep(20 * time.Millisecond)
	if err := os.WriteFile(path, []byte("runtime: {enabled: true}\n"), 0o644); err != nil {
		t.Fatalf("update config file: %v", err)
	}

	var wg sync.WaitGroup
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			watcher.Stop()
		}()
	}
	cancel()
	wg.Wait()
	watcher.Stop()
}

