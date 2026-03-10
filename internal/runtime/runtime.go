// Package runtime is the Kaku CLI runtime — a multi-session manager that
// launches member CLIs (claude_code, codex, kimi, …) in Kaku terminal panes
// and tracks their lifecycle.
//
// Architecture:
//
//	Runtime
//	  ├─ Manager (panel) — controls Kaku panes via `kaku cli`
//	  ├─ Store           — persists session metadata as JSON files
//	  ├─ Bus             — in-process event pub/sub
//	  ├─ Factory         — creates Adapters per member type
//	  ├─ Pool            — optional fixed-size pane pool
//	  └─ sessions map    — in-memory session registry
package runtime

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"alex/internal/runtime/adapter"
	"alex/internal/runtime/hooks"
	"alex/internal/runtime/panel"
	"alex/internal/runtime/pool"
	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

// Runtime manages multiple Kaku sessions.
// It implements adapter.HookSink so adapters can call back directly.
type Runtime struct {
	mu       sync.RWMutex
	sessions map[string]*session.Session
	adapters map[string]adapter.Adapter

	panel   panel.ManagerIface
	store   *store.Store
	bus     hooks.Bus
	factory AdapterFactory
	pool    *pool.Pool // optional; nil disables pool mode
}

// AdapterFactory creates runtime adapters for member types.
type AdapterFactory interface {
	New(member session.MemberType) (adapter.Adapter, error)
}

// Config holds optional wiring for the Runtime.
type Config struct {
	// Factory is the adapter factory for launching member CLIs.
	// If nil, StartSession will fail for any member type.
	Factory AdapterFactory

	// Bus is the event bus. If nil, an in-process bus is created.
	Bus hooks.Bus
}

// New creates a Runtime, restoring any previously persisted sessions from storeDir.
func New(storeDir string, cfg Config) (*Runtime, error) {
	pm, err := panel.NewManager()
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}

	st, err := store.New(storeDir)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}

	bus := cfg.Bus
	if bus == nil {
		bus = hooks.NewInProcessBus()
	}

	rt := &Runtime{
		sessions: make(map[string]*session.Session),
		adapters: make(map[string]adapter.Adapter),
		panel:    pm,
		store:    st,
		bus:      bus,
		factory:  cfg.Factory,
	}

	// Restore session metadata from disk.
	saved, err := st.LoadAll()
	if err != nil {
		return nil, fmt.Errorf("runtime: load sessions: %w", err)
	}
	for _, s := range saved {
		rt.sessions[s.ID] = s
	}

	return rt, nil
}

// Bus returns the event bus (read-only; for wiring scheduler/leader/detector).
func (rt *Runtime) Bus() hooks.Bus { return rt.bus }

// SetFactory wires an adapter factory after construction.
// Useful when the factory needs the runtime as its HookSink (circular init).
func (rt *Runtime) SetFactory(f AdapterFactory) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.factory = f
}

// SetPool wires a pane pool. When set, StartSession will acquire panes from
// the pool instead of splitting new ones (unless parentPaneID is explicitly provided).
func (rt *Runtime) SetPool(p *pool.Pool) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.pool = p
}

// Pool returns the pane pool (may be nil).
func (rt *Runtime) Pool() *pool.Pool {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.pool
}

// CreateSession allocates a new session and persists its metadata.
func (rt *Runtime) CreateSession(member session.MemberType, goal, workDir, parentSessionID string) (*session.Session, error) {
	id := newSessionID()
	s := session.New(id, member, goal, workDir)
	if parentSessionID != "" {
		s.SetParentSession(parentSessionID)
	}

	rt.mu.Lock()
	rt.sessions[id] = s
	rt.mu.Unlock()

	if err := rt.store.Save(s); err != nil {
		return nil, fmt.Errorf("runtime: persist session: %w", err)
	}
	return s, nil
}

// StartSession creates a Kaku pane for the session and launches the member CLI.
// parentPaneID is the pane to split from (use -1 to skip pane creation for testing).
// When a pool is configured and parentPaneID is -1, a pane is acquired from the pool.
func (rt *Runtime) StartSession(ctx context.Context, id string, parentPaneID int) error {
	s := rt.get(id)
	if s == nil {
		return fmt.Errorf("runtime: session %s not found", id)
	}

	if err := s.Transition(session.StateStarting); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	_ = rt.store.Save(s)

	snap := s.Snapshot()

	// Determine pane acquisition strategy:
	//   1. parentPaneID >= 0 → split from that pane (explicit)
	//   2. pool available and parentPaneID == -1 → acquire from pool
	//   3. parentPaneID == -1 and no pool → tracking-only (no pane)
	rt.mu.RLock()
	p := rt.pool
	rt.mu.RUnlock()

	if rt.factory != nil && parentPaneID >= 0 {
		// Explicit parent pane — use split mode.
		return rt.startWithAdapter(ctx, s, id, snap, adapter.StartOpts{
			SessionID: id,
			Goal:      snap.Goal,
			WorkDir:   snap.WorkDir,
			PaneID:    parentPaneID,
			Mode:      adapter.ModeSplit,
		}, false)
	}

	if rt.factory != nil && p != nil && p.Size() > 0 && parentPaneID < 0 {
		// Pool mode — acquire an idle pane.
		paneID, err := p.Acquire(ctx, id)
		if err != nil {
			_ = s.Transition(session.StateFailed)
			s.SetError(err.Error())
			_ = rt.store.Save(s)
			return fmt.Errorf("runtime: pool acquire: %w", err)
		}
		s.SetPane(paneID, paneID)
		s.SetPoolPane(true)
		_ = rt.store.Save(s)

		return rt.startWithAdapter(ctx, s, id, snap, adapter.StartOpts{
			SessionID: id,
			Goal:      snap.Goal,
			WorkDir:   snap.WorkDir,
			PaneID:    paneID,
			Mode:      adapter.ModeDirectPane,
		}, true)
	}

	if parentPaneID >= 0 {
		// Legacy path: no factory, split pane directly.
		pane, err := rt.panel.Split(ctx, panel.SplitOpts{
			ParentPaneID: parentPaneID,
			Direction:    "bottom",
			Percent:      65,
			WorkDir:      snap.WorkDir,
		})
		if err != nil {
			_ = s.Transition(session.StateFailed)
			s.SetError(err.Error())
			_ = rt.store.Save(s)
			return fmt.Errorf("runtime: create pane: %w", err)
		}
		s.SetPane(pane.PaneID(), pane.PaneID())
		_ = pane.Activate(ctx)
	}

	if err := s.Transition(session.StateRunning); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	_ = rt.store.Save(s)
	rt.bus.Publish(id, hooks.Event{Type: hooks.EventStarted, SessionID: id, At: time.Now()})
	return nil
}

// startWithAdapter is the common path for launching via an adapter (split or pool mode).
func (rt *Runtime) startWithAdapter(ctx context.Context, s *session.Session, id string, snap session.SessionData, opts adapter.StartOpts, isPoolPane bool) error {
	adp, err := rt.factory.New(snap.Member)
	if err != nil {
		_ = s.Transition(session.StateFailed)
		s.SetError(err.Error())
		_ = rt.store.Save(s)
		return fmt.Errorf("runtime: create adapter: %w", err)
	}

	rt.mu.Lock()
	rt.adapters[id] = adp
	rt.mu.Unlock()

	if err := adp.Start(ctx, opts); err != nil {
		rt.mu.Lock()
		delete(rt.adapters, id)
		rt.mu.Unlock()

		_ = s.Transition(session.StateFailed)
		s.SetError(err.Error())
		_ = rt.store.Save(s)

		// Release pool pane back on failure.
		if isPoolPane {
			rt.mu.RLock()
			p := rt.pool
			rt.mu.RUnlock()
			if p != nil {
				p.Release(opts.PaneID)
			}
		}
		return fmt.Errorf("runtime: start adapter: %w", err)
	}

	if err := s.Transition(session.StateRunning); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	_ = rt.store.Save(s)
	rt.bus.Publish(id, hooks.Event{Type: hooks.EventStarted, SessionID: id, At: time.Now()})
	return nil
}

// StopSession kills the pane and marks the session cancelled.
func (rt *Runtime) StopSession(ctx context.Context, id string) error {
	s := rt.get(id)
	if s == nil {
		return fmt.Errorf("runtime: session %s not found", id)
	}

	snap := s.Snapshot()

	// Stop the adapter if one is running.
	rt.mu.Lock()
	adp := rt.adapters[id]
	delete(rt.adapters, id)
	rt.mu.Unlock()

	if adp != nil {
		_ = adp.Stop(ctx, id, snap.PoolPane) // best-effort
	} else {
		if snap.PaneID >= 0 && !snap.PoolPane {
			p := panel.NewPane(snap.PaneID)
			_ = p.Kill(ctx)
		}
	}

	// Release pool pane if applicable.
	rt.releasePoolPane(snap)

	if err := s.Transition(session.StateCancelled); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	return rt.store.Save(s)
}

// InjectText sends text into a running session's CLI.
func (rt *Runtime) InjectText(ctx context.Context, id, text string) error {
	rt.mu.RLock()
	adp := rt.adapters[id]
	rt.mu.RUnlock()

	if adp == nil {
		return fmt.Errorf("runtime: no adapter for session %s", id)
	}
	return adp.Inject(ctx, id, text)
}

// RecordEvent persists ev to the session's event log.
func (rt *Runtime) RecordEvent(sessionID, eventType string, payload map[string]any) {
	rt.store.AppendEvent(sessionID, eventType, payload)
}

// ListSessions returns snapshots of all known sessions.
func (rt *Runtime) ListSessions() []session.SessionData {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	out := make([]session.SessionData, 0, len(rt.sessions))
	for _, s := range rt.sessions {
		out = append(out, s.Snapshot())
	}
	return out
}

// GetSession returns a snapshot of a single session.
func (rt *Runtime) GetSession(id string) (session.SessionData, bool) {
	s := rt.get(id)
	if s == nil {
		return session.SessionData{}, false
	}
	return s.Snapshot(), true
}

// RecordHeartbeat updates the session's last-active timestamp.
func (rt *Runtime) RecordHeartbeat(id string) {
	if s := rt.get(id); s != nil {
		s.RecordHeartbeat()
		_ = rt.store.Save(s)
	}
}

// MarkCompleted moves the session to completed and records the answer.
func (rt *Runtime) MarkCompleted(id, answer string) error {
	s := rt.get(id)
	if s == nil {
		return fmt.Errorf("runtime: session %s not found", id)
	}
	s.SetResult(answer)
	if err := s.Transition(session.StateCompleted); err != nil {
		return err
	}

	snap := s.Snapshot()

	if saveErr := rt.store.Save(s); saveErr != nil {
		return saveErr
	}

	// Release pool pane back to the pool.
	rt.releasePoolPane(snap)

	rt.bus.Publish(id, hooks.Event{
		Type:      hooks.EventCompleted,
		SessionID: id,
		At:        time.Now(),
		Payload:   map[string]any{"answer": answer},
	})

	// If this session has a parent, notify the parent via EventChildCompleted.
	if snap.ParentSessionID != "" {
		rt.bus.Publish(snap.ParentSessionID, hooks.Event{
			Type:      hooks.EventChildCompleted,
			SessionID: snap.ParentSessionID,
			At:        time.Now(),
			Payload: map[string]any{
				"child_id":     id,
				"child_goal":   snap.Goal,
				"child_answer": answer,
			},
		})
	}

	return nil
}

// MarkFailed moves the session to failed and records the error.
func (rt *Runtime) MarkFailed(id, errMsg string) error {
	s := rt.get(id)
	if s == nil {
		return fmt.Errorf("runtime: session %s not found", id)
	}
	s.SetError(errMsg)
	if err := s.Transition(session.StateFailed); err != nil {
		return err
	}

	snap := s.Snapshot()

	if saveErr := rt.store.Save(s); saveErr != nil {
		return saveErr
	}

	// Release pool pane back to the pool.
	rt.releasePoolPane(snap)

	rt.bus.Publish(id, hooks.Event{
		Type:      hooks.EventFailed,
		SessionID: id,
		At:        time.Now(),
		Payload:   map[string]any{"error": errMsg},
	})

	// If this session has a parent, notify the parent via EventChildCompleted
	// (even on failure, so the leader can decide what to do).
	if snap.ParentSessionID != "" {
		rt.bus.Publish(snap.ParentSessionID, hooks.Event{
			Type:      hooks.EventChildCompleted,
			SessionID: snap.ParentSessionID,
			At:        time.Now(),
			Payload: map[string]any{
				"child_id":    id,
				"child_goal":  snap.Goal,
				"child_error": errMsg,
			},
		})
	}

	return nil
}

// ScanStalled checks all running sessions against the stall threshold and
// returns the IDs of any that have gone quiet.
func (rt *Runtime) ScanStalled(threshold time.Duration) []string {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	var stalled []string
	for id, s := range rt.sessions {
		if s.IsStalled(threshold) {
			stalled = append(stalled, id)
		}
	}
	return stalled
}

// — adapter.HookSink implementation —

// OnHeartbeat is called by an adapter when the member CLI shows sign of life.
func (rt *Runtime) OnHeartbeat(sessionID string) {
	rt.RecordHeartbeat(sessionID)
	rt.bus.Publish(sessionID, hooks.Event{
		Type:      hooks.EventHeartbeat,
		SessionID: sessionID,
		At:        time.Now(),
	})
	rt.store.AppendEvent(sessionID, string(hooks.EventHeartbeat), nil)
}

// OnCompleted is called by an adapter when the member CLI finishes successfully.
func (rt *Runtime) OnCompleted(sessionID, answer string) {
	_ = rt.MarkCompleted(sessionID, answer)
	rt.store.AppendEvent(sessionID, string(hooks.EventCompleted), map[string]any{"answer": answer})
}

// OnFailed is called by an adapter when the member CLI exits with an error.
func (rt *Runtime) OnFailed(sessionID, errMsg string) {
	_ = rt.MarkFailed(sessionID, errMsg)
	rt.store.AppendEvent(sessionID, string(hooks.EventFailed), map[string]any{"error": errMsg})
}

// OnNeedsInput is called by an adapter when the member CLI is waiting for input.
func (rt *Runtime) OnNeedsInput(sessionID, prompt string) {
	if s := rt.get(sessionID); s != nil {
		_ = s.Transition(session.StateNeedsInput)
		_ = rt.store.Save(s)
	}
	rt.bus.Publish(sessionID, hooks.Event{
		Type:      hooks.EventNeedsInput,
		SessionID: sessionID,
		At:        time.Now(),
		Payload:   map[string]any{"prompt": prompt},
	})
	rt.store.AppendEvent(sessionID, string(hooks.EventNeedsInput), map[string]any{"prompt": prompt})
}

// releasePoolPane returns a pool pane to the pool if applicable.
func (rt *Runtime) releasePoolPane(snap session.SessionData) {
	if !snap.PoolPane || snap.PaneID < 0 {
		return
	}
	rt.mu.RLock()
	p := rt.pool
	rt.mu.RUnlock()
	if p != nil {
		p.Release(snap.PaneID)
	}
}

func (rt *Runtime) get(id string) *session.Session {
	rt.mu.RLock()
	defer rt.mu.RUnlock()
	return rt.sessions[id]
}

// newSessionID generates a short random session identifier.
func newSessionID() string {
	const chars = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = chars[rand.Intn(len(chars))]
	}
	return "rs-" + string(b)
}
