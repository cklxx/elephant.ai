//go:build integration

package hooks_test

import (
	"context"
	"sync"
	"testing"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/leader"
	"alex/internal/runtime/session"
)

type recordingPane struct {
	mu       sync.Mutex
	injected []string
}

func (p *recordingPane) record(text string) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.injected = append(p.injected, text)
}

func (p *recordingPane) injectedTexts() []string {
	p.mu.Lock()
	defer p.mu.Unlock()
	out := make([]string, len(p.injected))
	copy(out, p.injected)
	return out
}

type integrationRuntime struct {
	mu       sync.RWMutex
	sessions map[string]*session.Session
	pane     *recordingPane
	failures map[string]string
}

func newIntegrationRuntime() *integrationRuntime {
	return &integrationRuntime{
		sessions: make(map[string]*session.Session),
		pane:     &recordingPane{},
		failures: make(map[string]string),
	}
}

func (r *integrationRuntime) startSession(id, goal string, startedAt time.Time) {
	s := session.New(id, session.MemberClaudeCode, goal, "")
	if err := s.Transition(session.StateStarting); err != nil {
		panic(err)
	}
	if err := s.Transition(session.StateRunning); err != nil {
		panic(err)
	}
	s.StartedAt = &startedAt

	r.mu.Lock()
	r.sessions[id] = s
	r.mu.Unlock()
}

func (r *integrationRuntime) ScanStalled(threshold time.Duration) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	var stalled []string
	for id, s := range r.sessions {
		if s.IsStalled(threshold) {
			stalled = append(stalled, id)
		}
	}
	return stalled
}

func (r *integrationRuntime) GetSession(id string) (session.SessionData, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	s, ok := r.sessions[id]
	if !ok {
		return session.SessionData{}, false
	}
	return s.Snapshot(), true
}

func (r *integrationRuntime) InjectText(_ context.Context, id, text string) error {
	r.pane.record(id + "|" + text)
	return nil
}

func (r *integrationRuntime) MarkFailed(id, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.failures[id] = errMsg
	return nil
}

func waitForEvent(t *testing.T, ch <-chan hooks.Event, want hooks.EventType) hooks.Event {
	t.Helper()

	select {
	case ev := <-ch:
		if ev.Type != want {
			t.Fatalf("event type = %q, want %q", ev.Type, want)
		}
		return ev
	case <-time.After(time.Second):
		t.Fatalf("timed out waiting for %s", want)
		return hooks.Event{}
	}
}

func waitNoEvent(t *testing.T, ch <-chan hooks.Event) {
	t.Helper()

	select {
	case ev := <-ch:
		t.Fatalf("unexpected event after unsubscribe: %+v", ev)
	case <-time.After(100 * time.Millisecond):
	}
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

func TestEventBus_PubSub(t *testing.T) {
	bus := hooks.NewInProcessBus()

	sessionCh, cancelSession := bus.Subscribe("session-1")
	defer cancelSession()
	wildcardCh, cancelWildcard := bus.SubscribeAll()
	defer cancelWildcard()

	ev := hooks.Event{
		Type:      hooks.EventHeartbeat,
		SessionID: "session-1",
		At:        time.Now(),
	}
	bus.Publish("session-1", ev)

	gotSession := waitForEvent(t, sessionCh, hooks.EventHeartbeat)
	if gotSession.SessionID != "session-1" {
		t.Fatalf("session subscriber got session %q, want session-1", gotSession.SessionID)
	}

	gotWildcard := waitForEvent(t, wildcardCh, hooks.EventHeartbeat)
	if gotWildcard.SessionID != "session-1" {
		t.Fatalf("wildcard subscriber got session %q, want session-1", gotWildcard.SessionID)
	}
}

func TestEventBus_MultipleSubscribers(t *testing.T) {
	bus := hooks.NewInProcessBus()

	ch1, cancel1 := bus.Subscribe("session-1")
	defer cancel1()
	ch2, cancel2 := bus.Subscribe("session-1")
	defer cancel2()
	ch3, cancel3 := bus.Subscribe("session-1")
	defer cancel3()

	ev := hooks.Event{
		Type:      hooks.EventStarted,
		SessionID: "session-1",
		At:        time.Now(),
	}
	bus.Publish("session-1", ev)

	for idx, ch := range []<-chan hooks.Event{ch1, ch2, ch3} {
		got := waitForEvent(t, ch, hooks.EventStarted)
		if got.SessionID != "session-1" {
			t.Fatalf("subscriber %d got session %q, want session-1", idx+1, got.SessionID)
		}
	}
}

func TestStallDetector_Integration(t *testing.T) {
	bus := hooks.NewInProcessBus()
	rt := newIntegrationRuntime()
	rt.startSession("session-1", "repair stalled worker", time.Now().Add(-3*time.Second))

	leaderReceived := make(chan struct{}, 1)
	execute := func(_ context.Context, prompt, sessionID string) (string, error) {
		if sessionID != "leader-stall-session-1" {
			t.Fatalf("leader sessionID = %q, want leader-stall-session-1", sessionID)
		}
		if prompt == "" {
			t.Fatal("leader prompt should not be empty")
		}
		leaderReceived <- struct{}{}
		return "INJECT continue from the last successful step", nil
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	agent := leader.New(rt, bus, execute)
	go agent.Run(ctx)
	go hooks.NewStallDetector(rt, bus, 2*time.Second, 10*time.Millisecond).Run(ctx)

	select {
	case <-leaderReceived:
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for leader to receive stalled event")
	}

	waitForCondition(t, time.Second, func() bool {
		texts := rt.pane.injectedTexts()
		return len(texts) == 1 && texts[0] == "session-1|continue from the last successful step"
	})
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := hooks.NewInProcessBus()

	ch, cancel := bus.Subscribe("session-1")
	cancel()

	bus.Publish("session-1", hooks.Event{
		Type:      hooks.EventCompleted,
		SessionID: "session-1",
		At:        time.Now(),
	})

	waitNoEvent(t, ch)
}
