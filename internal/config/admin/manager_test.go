package admin

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"time"

	runtimeconfig "alex/internal/config"
)

type stubStore struct {
	loadFn func(context.Context) (runtimeconfig.Overrides, error)
	saveFn func(context.Context, runtimeconfig.Overrides) error
}

func (s *stubStore) LoadOverrides(ctx context.Context) (runtimeconfig.Overrides, error) {
	if s.loadFn != nil {
		return s.loadFn(ctx)
	}
	return runtimeconfig.Overrides{}, nil
}

func (s *stubStore) SaveOverrides(ctx context.Context, overrides runtimeconfig.Overrides) error {
	if s.saveFn != nil {
		return s.saveFn(ctx, overrides)
	}
	return nil
}

func TestManagerCurrentOverridesUsesCacheUntilExpired(t *testing.T) {
	t.Parallel()

	var loads atomic.Int64
	store := &stubStore{
		loadFn: func(context.Context) (runtimeconfig.Overrides, error) {
			loads.Add(1)
			value := "store-provider"
			return runtimeconfig.Overrides{LLMProvider: &value}, nil
		},
	}

	initialValue := "cached-provider"
	manager := NewManager(store, runtimeconfig.Overrides{LLMProvider: &initialValue}, WithCacheTTL(time.Hour))

	got, err := manager.CurrentOverrides(context.Background())
	if err != nil {
		t.Fatalf("CurrentOverrides returned error: %v", err)
	}
	if got.LLMProvider == nil || *got.LLMProvider != initialValue {
		t.Fatalf("expected cached value %q, got %#v", initialValue, got.LLMProvider)
	}
	if loads.Load() != 0 {
		t.Fatalf("expected store not to be hit while cache fresh, got %d loads", loads.Load())
	}

	manager.cachedAt = time.Now().Add(-2 * time.Hour)

	got, err = manager.CurrentOverrides(context.Background())
	if err != nil {
		t.Fatalf("CurrentOverrides returned error after expiry: %v", err)
	}
	if got.LLMProvider == nil || *got.LLMProvider != "store-provider" {
		t.Fatalf("expected store value after cache expiry, got %#v", got.LLMProvider)
	}
	if loads.Load() != 1 {
		t.Fatalf("expected one store load after expiry, got %d", loads.Load())
	}
}

func TestManagerUpdateOverridesPersistsAndNotifies(t *testing.T) {
	t.Parallel()

	var saved runtimeconfig.Overrides
	var savedCount atomic.Int64
	store := &stubStore{
		saveFn: func(_ context.Context, overrides runtimeconfig.Overrides) error {
			saved = overrides
			savedCount.Add(1)
			return nil
		},
	}

	manager := NewManager(store, runtimeconfig.Overrides{}, WithCacheTTL(time.Hour))

	ch, unsubscribe := manager.Subscribe()
	defer unsubscribe()

	llm := "new-provider"
	overrides := runtimeconfig.Overrides{LLMProvider: &llm}
	if err := manager.UpdateOverrides(context.Background(), overrides); err != nil {
		t.Fatalf("UpdateOverrides returned error: %v", err)
	}

	if savedCount.Load() != 1 {
		t.Fatalf("expected store save to be invoked once, got %d", savedCount.Load())
	}
	if saved.LLMProvider == nil || *saved.LLMProvider != llm {
		t.Fatalf("expected overrides to be persisted, got %#v", saved.LLMProvider)
	}

	select {
	case <-time.After(time.Second):
		t.Fatal("expected subscriber to receive update notification")
	case update := <-ch:
		if update.LLMProvider == nil || *update.LLMProvider != llm {
			t.Fatalf("subscriber received unexpected overrides: %#v", update)
		}
	}
}

func TestManagerUpdateOverridesIgnoresUnsubscribedChannels(t *testing.T) {
	t.Parallel()

	store := &stubStore{}
	manager := NewManager(store, runtimeconfig.Overrides{})

	ch, unsubscribe := manager.Subscribe()
	unsubscribe()

	llm := "unsubscribed"
	overrides := runtimeconfig.Overrides{LLMProvider: &llm}

	done := make(chan struct{})
	go func() {
		select {
		case <-time.After(100 * time.Millisecond):
		case <-ch:
		}
		close(done)
	}()

	if err := manager.UpdateOverrides(context.Background(), overrides); err != nil {
		t.Fatalf("UpdateOverrides returned error: %v", err)
	}

	<-done
}

func TestManagerSubscribeSafeForNil(t *testing.T) {
	var nilManager *Manager
	ch, unsubscribe := nilManager.Subscribe()
	defer unsubscribe()

	select {
	case _, ok := <-ch:
		if ok {
			t.Fatal("expected closed channel from nil manager")
		}
	default:
		t.Fatal("expected channel to be closed immediately")
	}
}

func TestManagerUpdateOverridesPropagatesStoreErrors(t *testing.T) {
	expectedErr := errors.New("boom")
	store := &stubStore{
		saveFn: func(context.Context, runtimeconfig.Overrides) error {
			return expectedErr
		},
	}
	manager := NewManager(store, runtimeconfig.Overrides{})

	if err := manager.UpdateOverrides(context.Background(), runtimeconfig.Overrides{}); !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}

func TestManagerCurrentOverridesPropagatesStoreErrors(t *testing.T) {
	expectedErr := errors.New("failed")
	store := &stubStore{
		loadFn: func(context.Context) (runtimeconfig.Overrides, error) {
			return runtimeconfig.Overrides{}, expectedErr
		},
	}
	manager := NewManager(store, runtimeconfig.Overrides{}, WithCacheTTL(time.Nanosecond))
	manager.cachedAt = time.Now().Add(-time.Minute)

	if _, err := manager.CurrentOverrides(context.Background()); !errors.Is(err, expectedErr) {
		t.Fatalf("expected error %v, got %v", expectedErr, err)
	}
}
