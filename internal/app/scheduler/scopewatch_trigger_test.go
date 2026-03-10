package scheduler

import (
	"context"
	"testing"

	"alex/internal/shared/config"
)

type stubScopeWatchService struct {
	called bool
	err    error
}

func (s *stubScopeWatchService) NotifyScopeChanges(_ context.Context) error {
	s.called = true
	return s.err
}

func TestRegisterScopeWatchJob_Disabled(t *testing.T) {
	s := New(Config{Enabled: true}, nil, nil, nil)

	s.mu.Lock()
	s.registerScopeWatchJob(context.Background())
	s.mu.Unlock()

	if _, ok := s.entryIDs[scopeWatchTriggerName]; ok {
		t.Error("scope watch job should not be registered when disabled")
	}
}

func TestRegisterScopeWatchJob_NoService(t *testing.T) {
	s := New(Config{
		Enabled:    true,
		ScopeWatch: config.ScopeWatchConfig{Enabled: true},
	}, nil, nil, nil)

	s.mu.Lock()
	s.registerScopeWatchJob(context.Background())
	s.mu.Unlock()

	if _, ok := s.entryIDs[scopeWatchTriggerName]; ok {
		t.Error("scope watch job should not be registered without service")
	}
}

func TestRegisterScopeWatchJob_Registers(t *testing.T) {
	svc := &stubScopeWatchService{}
	s := New(Config{
		Enabled:           true,
		ScopeWatch:        config.ScopeWatchConfig{Enabled: true, Schedule: "*/30 * * * *"},
		ScopeWatchService: svc,
	}, nil, nil, nil)

	s.mu.Lock()
	s.registerScopeWatchJob(context.Background())
	s.mu.Unlock()

	if _, ok := s.entryIDs[scopeWatchTriggerName]; !ok {
		t.Error("scope watch job should be registered")
	}
}

func TestRegisterScopeWatchJob_DefaultSchedule(t *testing.T) {
	svc := &stubScopeWatchService{}
	s := New(Config{
		Enabled:           true,
		ScopeWatch:        config.ScopeWatchConfig{Enabled: true},
		ScopeWatchService: svc,
	}, nil, nil, nil)

	s.mu.Lock()
	s.registerScopeWatchJob(context.Background())
	s.mu.Unlock()

	if _, ok := s.entryIDs[scopeWatchTriggerName]; !ok {
		t.Error("scope watch should register with default schedule when schedule is empty")
	}
}
