package configcenter

import (
	"context"
	"testing"
	"time"

	runtimeconfig "alex/internal/config"
	"alex/internal/serverconfig"
)

type fakeStore struct {
	cfg serverconfig.Config
	err error
}

func (s *fakeStore) Load() (serverconfig.Config, error) {
	if s.err != nil {
		return serverconfig.Config{}, s.err
	}
	return s.cfg, nil
}

func (s *fakeStore) Save(cfg serverconfig.Config) error {
	if s.err != nil {
		return s.err
	}
	s.cfg = cfg
	return nil
}

func TestServiceCachesAndNotifies(t *testing.T) {
	store := &fakeStore{}
	svc := NewService(store, time.Minute)

	ctx := context.Background()
	base := serverconfig.Config{Runtime: runtimeconfig.RuntimeConfig{LLMProvider: "mock"}}

	if _, err := svc.SeedIfEmpty(ctx, base); err != nil {
		t.Fatalf("seed failed: %v", err)
	}

	ch, cancel := svc.Subscribe()
	defer cancel()
	<-ch // drain initial snapshot

	updated := base
	updated.Runtime.LLMModel = "updated"

	errCh := make(chan error, 1)
	go func() {
		time.Sleep(10 * time.Millisecond)
		_, err := svc.Update(ctx, updated)
		errCh <- err
	}()

	select {
	case snap := <-ch:
		if snap.Config.Runtime.LLMModel != "updated" {
			t.Fatalf("expected model to be updated, got %s", snap.Config.Runtime.LLMModel)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for update")
	}

	if err := <-errCh; err != nil {
		t.Fatalf("update failed: %v", err)
	}
}
