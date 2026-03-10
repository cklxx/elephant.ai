//go:build integration

package runtime

import (
	"context"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"testing"
	"time"

	"alex/internal/runtime/adapter"
	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/runtime/panel"
	"alex/internal/runtime/pool"
	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

type integrationFactory struct {
	mu        sync.Mutex
	manager   *mockManager
	poolPanes map[int]*mockPane
	adapters  map[string]*integrationAdapter
}

func newIntegrationFactory(manager *mockManager) *integrationFactory {
	return &integrationFactory{
		manager:   manager,
		poolPanes: make(map[int]*mockPane),
		adapters:  make(map[string]*integrationAdapter),
	}
}

func (f *integrationFactory) New(_ session.MemberType) (adapter.Adapter, error) {
	return &integrationAdapter{factory: f}, nil
}

func (f *integrationFactory) registerPoolPanes(paneIDs []int) {
	f.mu.Lock()
	defer f.mu.Unlock()
	for _, paneID := range paneIDs {
		if _, ok := f.poolPanes[paneID]; ok {
			continue
		}
		f.poolPanes[paneID] = newMockPane(paneID)
	}
}

func (f *integrationFactory) adapterFor(sessionID string) *integrationAdapter {
	f.mu.Lock()
	defer f.mu.Unlock()
	return f.adapters[sessionID]
}

type integrationAdapter struct {
	mu       sync.Mutex
	factory  *integrationFactory
	pane     *mockPane
	started  []adapter.StartOpts
	injected []string
	stopped  []struct {
		sessionID string
		poolPane  bool
	}
}

func (a *integrationAdapter) Start(ctx context.Context, opts adapter.StartOpts) error {
	a.mu.Lock()
	a.started = append(a.started, opts)
	a.mu.Unlock()

	var paneIface panel.PaneIface
	switch opts.Mode {
	case adapter.ModeDirectPane:
		a.factory.mu.Lock()
		paneIface = a.factory.poolPanes[opts.PaneID]
		a.factory.mu.Unlock()
		if paneIface == nil {
			return os.ErrNotExist
		}
	default:
		var err error
		paneIface, err = a.factory.manager.Split(ctx, panel.SplitOpts{
			ParentPaneID: opts.PaneID,
			Direction:    "bottom",
			Percent:      65,
			WorkDir:      opts.WorkDir,
		})
		if err != nil {
			return err
		}
	}

	pane, ok := paneIface.(*mockPane)
	if !ok {
		return os.ErrInvalid
	}

	if err := pane.Activate(ctx); err != nil {
		return err
	}
	if err := pane.InjectText(ctx, opts.Goal); err != nil {
		return err
	}
	if err := pane.Submit(ctx); err != nil {
		return err
	}

	a.mu.Lock()
	a.pane = pane
	a.mu.Unlock()

	a.factory.mu.Lock()
	a.factory.adapters[opts.SessionID] = a
	a.factory.mu.Unlock()
	return nil
}

func (a *integrationAdapter) Inject(ctx context.Context, sessionID, text string) error {
	a.mu.Lock()
	a.injected = append(a.injected, text)
	pane := a.pane
	a.mu.Unlock()

	if pane == nil {
		return os.ErrNotExist
	}
	if err := pane.InjectText(ctx, text); err != nil {
		return err
	}
	return pane.Submit(ctx)
}

func (a *integrationAdapter) Stop(_ context.Context, sessionID string, poolPane bool) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	a.stopped = append(a.stopped, struct {
		sessionID string
		poolPane  bool
	}{sessionID: sessionID, poolPane: poolPane})
	return nil
}

func newIntegrationRuntime(t *testing.T) (*Runtime, *mockManager, *integrationFactory, string) {
	t.Helper()

	storeDir := t.TempDir()
	st, err := store.New(storeDir)
	if err != nil {
		t.Fatalf("store.New: %v", err)
	}

	manager := &mockManager{nextPane: newMockPane(100)}
	bus := hooks.NewInProcessBus()
	factory := newIntegrationFactory(manager)

	rt := &Runtime{
		sessions: make(map[string]*session.Session),
		adapters: make(map[string]adapter.Adapter),
		panel:    manager,
		store:    st,
		bus:      bus,
		factory:  factory,
	}
	return rt, manager, factory, storeDir
}

func waitForCondition(t *testing.T, timeout time.Duration, fn func() bool) {
	t.Helper()

	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if fn() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition not met before timeout")
}

func TestRuntime_FullLifecycle(t *testing.T) {
	rt, manager, factory, _ := newIntegrationRuntime(t)

	p := pool.New()
	p.Register([]int{901})
	factory.registerPoolPanes([]int{901})
	rt.SetPool(p)

	s, err := rt.CreateSession(session.MemberClaudeCode, "implement full lifecycle", t.TempDir(), "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	sessionCh, cancel := rt.Bus().Subscribe(s.Snapshot().ID)
	defer cancel()

	if err := rt.StartSession(context.Background(), s.Snapshot().ID, 1); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	adapterInstance := factory.adapterFor(s.Snapshot().ID)
	if adapterInstance == nil {
		t.Fatal("expected adapter to be registered")
	}

	manager.mu.Lock()
	if len(manager.splits) != 1 {
		manager.mu.Unlock()
		t.Fatalf("expected 1 split, got %d", len(manager.splits))
	}
	manager.mu.Unlock()

	adapterInstance.mu.Lock()
	if len(adapterInstance.started) != 1 {
		adapterInstance.mu.Unlock()
		t.Fatalf("expected 1 adapter start, got %d", len(adapterInstance.started))
	}
	if adapterInstance.started[0].Goal != "implement full lifecycle" {
		adapterInstance.mu.Unlock()
		t.Fatalf("start goal = %q", adapterInstance.started[0].Goal)
	}
	pane := adapterInstance.pane
	adapterInstance.mu.Unlock()

	pane.mu.Lock()
	if !slices.Equal(pane.injected, []string{"implement full lifecycle"}) {
		pane.mu.Unlock()
		t.Fatalf("pane injected = %v", pane.injected)
	}
	if pane.submitted != 1 {
		pane.mu.Unlock()
		t.Fatalf("pane submitted = %d, want 1", pane.submitted)
	}
	pane.mu.Unlock()

	// Simulate a pooled pane assignment so completion exercises release logic.
	s.SetPane(901, 901)
	s.SetPoolPane(true)

	rt.OnHeartbeat(s.Snapshot().ID)
	rt.OnCompleted(s.Snapshot().ID, "done")

	snap := s.Snapshot()
	if snap.State != session.StateCompleted {
		t.Fatalf("state = %q, want %q", snap.State, session.StateCompleted)
	}
	if snap.Answer != "done" {
		t.Fatalf("answer = %q, want done", snap.Answer)
	}

	slots := p.Slots()
	if len(slots) != 1 || slots[0].State != pool.SlotIdle {
		t.Fatalf("pool slots = %+v, want released idle pane", slots)
	}

	var eventTypes []hooks.EventType
	for len(eventTypes) < 3 {
		select {
		case ev := <-sessionCh:
			eventTypes = append(eventTypes, ev.Type)
		case <-time.After(time.Second):
			t.Fatalf("timed out waiting for lifecycle events, got %v", eventTypes)
		}
	}
	if !slices.Equal(eventTypes, []hooks.EventType{hooks.EventStarted, hooks.EventHeartbeat, hooks.EventCompleted}) {
		t.Fatalf("events = %v", eventTypes)
	}
}

func TestRuntime_StallDetection(t *testing.T) {
	rt, _, _, _ := newIntegrationRuntime(t)

	s, err := rt.CreateSession(session.MemberClaudeCode, "stalled work", t.TempDir(), "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := rt.StartSession(context.Background(), s.Snapshot().ID, 1); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	startedAt := time.Now().Add(-3 * time.Second)
	s.StartedAt = &startedAt

	stalled := rt.ScanStalled(2 * time.Second)
	if !slices.Contains(stalled, s.Snapshot().ID) {
		t.Fatalf("stalled = %v, want %s", stalled, s.Snapshot().ID)
	}

	sessionCh, cancelSub := rt.Bus().Subscribe(s.Snapshot().ID)
	defer cancelSub()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go hooks.NewStallDetector(rt, rt.Bus(), 2*time.Second, 10*time.Millisecond).Run(ctx)

	select {
	case ev := <-sessionCh:
		if ev.Type != hooks.EventStalled {
			t.Fatalf("event type = %q, want %q", ev.Type, hooks.EventStalled)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for stalled event")
	}
}

func TestRuntime_LeaderIntervention(t *testing.T) {
	rt, _, factory, _ := newIntegrationRuntime(t)

	s, err := rt.CreateSession(session.MemberClaudeCode, "repair failing session", t.TempDir(), "")
	if err != nil {
		t.Fatalf("CreateSession: %v", err)
	}
	if err := rt.StartSession(context.Background(), s.Snapshot().ID, 1); err != nil {
		t.Fatalf("StartSession: %v", err)
	}

	var (
		mu      sync.Mutex
		prompts []string
	)
	execute := func(_ context.Context, prompt, sessionID string) (string, error) {
		mu.Lock()
		prompts = append(prompts, sessionID+"|"+prompt)
		mu.Unlock()
		return "INJECT apply the fix and continue", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := leader.New(rt, rt.Bus(), execute)
	go agent.Run(ctx)

	time.Sleep(20 * time.Millisecond)

	rt.Bus().Publish(s.Snapshot().ID, hooks.Event{
		Type:      hooks.EventStalled,
		SessionID: s.Snapshot().ID,
		At:        time.Now(),
	})

	waitForCondition(t, time.Second, func() bool {
		adapterInstance := factory.adapterFor(s.Snapshot().ID)
		if adapterInstance == nil {
			return false
		}
		adapterInstance.mu.Lock()
		defer adapterInstance.mu.Unlock()
		return slices.Equal(adapterInstance.injected, []string{"apply the fix and continue"})
	})

	mu.Lock()
	if len(prompts) != 1 {
		mu.Unlock()
		t.Fatalf("execute calls = %d, want 1", len(prompts))
	}
	mu.Unlock()

	rt.OnHeartbeat(s.Snapshot().ID)

	snap := s.Snapshot()
	if snap.State != session.StateRunning {
		t.Fatalf("state = %q, want %q", snap.State, session.StateRunning)
	}
	if snap.LastHeartbeat == nil {
		t.Fatal("expected recovery heartbeat to be recorded")
	}
}

func TestRuntime_PoolAcquireRelease(t *testing.T) {
	rt, _, factory, _ := newIntegrationRuntime(t)

	p := pool.New()
	if added := p.Register([]int{201, 202}); added != 2 {
		t.Fatalf("Register added %d panes, want 2", added)
	}
	factory.registerPoolPanes([]int{201, 202})
	rt.SetPool(p)

	s1, _ := rt.CreateSession(session.MemberClaudeCode, "task 1", t.TempDir(), "")
	s2, _ := rt.CreateSession(session.MemberClaudeCode, "task 2", t.TempDir(), "")
	s3, _ := rt.CreateSession(session.MemberClaudeCode, "task 3", t.TempDir(), "")

	if err := rt.StartSession(context.Background(), s1.Snapshot().ID, -1); err != nil {
		t.Fatalf("StartSession s1: %v", err)
	}
	if err := rt.StartSession(context.Background(), s2.Snapshot().ID, -1); err != nil {
		t.Fatalf("StartSession s2: %v", err)
	}

	s1Snap := s1.Snapshot()
	s2Snap := s2.Snapshot()
	if !s1Snap.PoolPane || !s2Snap.PoolPane {
		t.Fatalf("expected pool panes, got s1=%+v s2=%+v", s1Snap, s2Snap)
	}
	if s1Snap.PaneID == s2Snap.PaneID {
		t.Fatalf("expected different pane IDs, got %d", s1Snap.PaneID)
	}

	if err := rt.MarkCompleted(s1.Snapshot().ID, "done"); err != nil {
		t.Fatalf("MarkCompleted s1: %v", err)
	}

	if err := rt.StartSession(context.Background(), s3.Snapshot().ID, -1); err != nil {
		t.Fatalf("StartSession s3: %v", err)
	}

	s3Snap := s3.Snapshot()
	if s3Snap.PaneID != s1Snap.PaneID {
		t.Fatalf("pane recycle = %d, want %d", s3Snap.PaneID, s1Snap.PaneID)
	}

	slots := p.Slots()
	if len(slots) != 2 {
		t.Fatalf("slots = %+v", slots)
	}
	busyCount := 0
	for _, slot := range slots {
		if slot.State == pool.SlotBusy {
			busyCount++
		}
	}
	if busyCount != 2 {
		t.Fatalf("busy slots = %d, want 2 (%+v)", busyCount, slots)
	}
}

func TestRuntime_PersistenceRecovery(t *testing.T) {
	rt, _, factory, storeDir := newIntegrationRuntime(t)

	p := pool.New()
	p.Register([]int{301})
	factory.registerPoolPanes([]int{301})
	rt.SetPool(p)

	runningSession, err := rt.CreateSession(session.MemberClaudeCode, "running task", t.TempDir(), "")
	if err != nil {
		t.Fatalf("CreateSession running: %v", err)
	}
	completedSession, err := rt.CreateSession(session.MemberClaudeCode, "completed task", t.TempDir(), "")
	if err != nil {
		t.Fatalf("CreateSession completed: %v", err)
	}

	if err := rt.StartSession(context.Background(), runningSession.Snapshot().ID, 1); err != nil {
		t.Fatalf("StartSession running: %v", err)
	}
	if err := rt.StartSession(context.Background(), completedSession.Snapshot().ID, -1); err != nil {
		t.Fatalf("StartSession completed: %v", err)
	}
	if err := rt.MarkCompleted(completedSession.Snapshot().ID, "persisted answer"); err != nil {
		t.Fatalf("MarkCompleted completed: %v", err)
	}

	fakeKaku := filepath.Join(t.TempDir(), "kaku")
	if err := os.WriteFile(fakeKaku, []byte("#!/bin/sh\nexit 0\n"), 0o755); err != nil {
		t.Fatalf("WriteFile fake kaku: %v", err)
	}
	t.Setenv("KAKU_BIN", fakeKaku)

	recovered, err := New(storeDir, Config{Bus: hooks.NewInProcessBus()})
	if err != nil {
		t.Fatalf("New recovered runtime: %v", err)
	}

	runningSnap, ok := recovered.GetSession(runningSession.Snapshot().ID)
	if !ok {
		t.Fatal("expected running session to be restored")
	}
	if runningSnap.State != session.StateRunning {
		t.Fatalf("running state = %q, want %q", runningSnap.State, session.StateRunning)
	}

	completedSnap, ok := recovered.GetSession(completedSession.Snapshot().ID)
	if !ok {
		t.Fatal("expected completed session to be restored")
	}
	if completedSnap.State != session.StateCompleted {
		t.Fatalf("completed state = %q, want %q", completedSnap.State, session.StateCompleted)
	}
	if completedSnap.Answer != "persisted answer" {
		t.Fatalf("completed answer = %q, want persisted answer", completedSnap.Answer)
	}
}
