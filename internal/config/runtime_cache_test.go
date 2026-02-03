package config

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"
)

func TestRuntimeConfigCacheReloadUpdatesSnapshot(t *testing.T) {
	model := atomic.Value{}
	model.Store("initial")
	loader := func(context.Context) (RuntimeConfig, Metadata, error) {
		cfg := RuntimeConfig{LLMModel: model.Load().(string)}
		meta := Metadata{loadedAt: time.Now()}
		return cfg, meta, nil
	}

	cache, err := NewRuntimeConfigCache(loader)
	if err != nil {
		t.Fatalf("NewRuntimeConfigCache failed: %v", err)
	}

	cfg, meta, err := cache.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve returned error: %v", err)
	}
	if cfg.LLMModel != "initial" {
		t.Fatalf("expected initial model, got %q", cfg.LLMModel)
	}
	initialLoadedAt := meta.LoadedAt()

	model.Store("updated")
	if err := cache.Reload(context.Background()); err != nil {
		t.Fatalf("Reload returned error: %v", err)
	}

	cfg, meta, err = cache.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve returned error after reload: %v", err)
	}
	if cfg.LLMModel != "updated" {
		t.Fatalf("expected updated model, got %q", cfg.LLMModel)
	}
	if !meta.LoadedAt().After(initialLoadedAt) {
		t.Fatalf("expected LoadedAt to advance after reload")
	}
}

func TestRuntimeConfigCacheReloadKeepsLastOnError(t *testing.T) {
	var calls atomic.Int64
	loader := func(context.Context) (RuntimeConfig, Metadata, error) {
		if calls.Add(1) == 1 {
			cfg := RuntimeConfig{LLMProvider: "mock"}
			meta := Metadata{loadedAt: time.Now()}
			return cfg, meta, nil
		}
		return RuntimeConfig{}, Metadata{}, errors.New("boom")
	}

	cache, err := NewRuntimeConfigCache(loader)
	if err != nil {
		t.Fatalf("NewRuntimeConfigCache failed: %v", err)
	}

	if err := cache.Reload(context.Background()); err == nil {
		t.Fatalf("expected reload error")
	}

	cfg, _, err := cache.Resolve(context.Background())
	if err != nil {
		t.Fatalf("Resolve returned error after failed reload: %v", err)
	}
	if cfg.LLMProvider != "mock" {
		t.Fatalf("expected cached provider to remain, got %q", cfg.LLMProvider)
	}
}

func TestRuntimeConfigCacheUpdatesChannelNonBlocking(t *testing.T) {
	loader := func(context.Context) (RuntimeConfig, Metadata, error) {
		cfg := RuntimeConfig{LLMProvider: "mock"}
		meta := Metadata{loadedAt: time.Now()}
		return cfg, meta, nil
	}

	cache, err := NewRuntimeConfigCache(loader)
	if err != nil {
		t.Fatalf("NewRuntimeConfigCache failed: %v", err)
	}

	done := make(chan struct{})
	go func() {
		_ = cache.Reload(context.Background())
		close(done)
	}()

	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Reload blocked on updates channel")
	}

	select {
	case <-cache.Updates():
	case <-time.After(time.Second):
		t.Fatal("expected updates channel to receive signal")
	}
}
