// Package runtime is the Kaku CLI runtime — a multi-session manager that
// launches member CLIs (claude_code, codex, kimi, …) in Kaku terminal panes
// and tracks their lifecycle.
//
// Architecture:
//
//	Runtime
//	  ├─ Manager (panel) — controls Kaku panes via `kaku cli`
//	  ├─ Store           — persists session metadata as JSON files
//	  └─ sessions map    — in-memory session registry
package runtime

import (
	"context"
	"fmt"
	"math/rand"
	"sync"
	"time"

	"alex/internal/runtime/panel"
	"alex/internal/runtime/session"
	"alex/internal/runtime/store"
)

// Runtime manages multiple Kaku sessions.
type Runtime struct {
	mu      sync.RWMutex
	sessions map[string]*session.Session

	panel *panel.Manager
	store *store.Store
}

// New creates a Runtime, restoring any previously persisted sessions from storeDir.
func New(storeDir string) (*Runtime, error) {
	pm, err := panel.NewManager()
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}

	st, err := store.New(storeDir)
	if err != nil {
		return nil, fmt.Errorf("runtime: %w", err)
	}

	rt := &Runtime{
		sessions: make(map[string]*session.Session),
		panel:    pm,
		store:    st,
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

// CreateSession allocates a new session and persists its metadata.
func (rt *Runtime) CreateSession(member session.MemberType, goal, workDir string) (*session.Session, error) {
	id := newSessionID()
	s := session.New(id, member, goal, workDir)

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
func (rt *Runtime) StartSession(ctx context.Context, id string, parentPaneID int) error {
	s := rt.get(id)
	if s == nil {
		return fmt.Errorf("runtime: session %s not found", id)
	}

	if err := s.Transition(session.StateStarting); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	_ = rt.store.Save(s)

	if parentPaneID >= 0 {
		pane, err := rt.panel.Split(ctx, panel.SplitOpts{
			ParentPaneID: parentPaneID,
			Direction:    "bottom",
			Percent:      65,
			WorkDir:      s.WorkDir,
		})
		if err != nil {
			_ = s.Transition(session.StateFailed)
			s.SetError(err.Error())
			_ = rt.store.Save(s)
			return fmt.Errorf("runtime: create pane: %w", err)
		}
		s.SetPane(pane.ID, pane.ID) // tab detection is a future enhancement
		_ = pane.Activate(ctx)
	}

	if err := s.Transition(session.StateRunning); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	_ = rt.store.Save(s)
	return nil
}

// StopSession kills the pane and marks the session cancelled.
func (rt *Runtime) StopSession(ctx context.Context, id string) error {
	s := rt.get(id)
	if s == nil {
		return fmt.Errorf("runtime: session %s not found", id)
	}

	snap := s.Snapshot()
	if snap.PaneID >= 0 {
		pane := &panel.Pane{ID: snap.PaneID}
		_ = pane.Kill(ctx) // best-effort
	}

	if err := s.Transition(session.StateCancelled); err != nil {
		return fmt.Errorf("runtime: %w", err)
	}
	return rt.store.Save(s)
}

// ListSessions returns snapshots of all known sessions.
func (rt *Runtime) ListSessions() []session.Session {
	rt.mu.RLock()
	defer rt.mu.RUnlock()

	out := make([]session.Session, 0, len(rt.sessions))
	for _, s := range rt.sessions {
		out = append(out, s.Snapshot())
	}
	return out
}

// GetSession returns a snapshot of a single session.
func (rt *Runtime) GetSession(id string) (session.Session, bool) {
	s := rt.get(id)
	if s == nil {
		return session.Session{}, false
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
	return rt.store.Save(s)
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
	return rt.store.Save(s)
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
