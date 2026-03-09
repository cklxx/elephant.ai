// Package scheduler provides dependency-graph-driven session scheduling for the
// Kaku runtime. Sessions can declare that they depend on other sessions; the
// engine starts each session automatically once all its dependencies complete.
package scheduler

import (
	"context"
	"fmt"
	"sync"
	"time"

	"alex/internal/runtime/hooks"
	"alex/internal/runtime/session"
)

// SessionSpec describes a session to be scheduled.
type SessionSpec struct {
	Member    session.MemberType
	Goal      string
	WorkDir   string
	DependsOn []string // session IDs that must complete before this one starts
}

// RuntimeManager is the minimal interface the Engine needs from Runtime.
type RuntimeManager interface {
	CreateSession(member session.MemberType, goal, workDir, parentSessionID string) (*session.Session, error)
	StartSession(ctx context.Context, id string, parentPaneID int) error
}

// Engine subscribes to the runtime event bus and starts sessions when their
// dependencies complete. It supports serial (DependsOn) and parallel
// (same DependsOn set, multiple specs) execution models.
type Engine struct {
	rt          RuntimeManager
	bus         hooks.Bus
	parentPaneID int

	mu       sync.Mutex
	specs    []sessionSpec // internal copy with assigned IDs
	done     map[string]bool
}

type sessionSpec struct {
	SessionSpec
	id      string
	started bool
}

// NewEngine creates a dependency scheduler.
// parentPaneID is passed to StartSession for each new session (-1 to disable pane creation).
func NewEngine(rt RuntimeManager, bus hooks.Bus, parentPaneID int) *Engine {
	return &Engine{
		rt:           rt,
		bus:          bus,
		parentPaneID: parentPaneID,
		done:         make(map[string]bool),
	}
}

// Schedule enqueues a set of session specs. Call before Run.
// Returns the assigned session IDs in the same order as specs.
func (e *Engine) Schedule(ctx context.Context, specs []SessionSpec) ([]string, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	ids := make([]string, 0, len(specs))
	for _, sp := range specs {
		s, err := e.rt.CreateSession(sp.Member, sp.Goal, sp.WorkDir, "")
		if err != nil {
			return nil, fmt.Errorf("scheduler: create session: %w", err)
		}
		e.specs = append(e.specs, sessionSpec{SessionSpec: sp, id: s.ID})
		ids = append(ids, s.ID)
	}
	return ids, nil
}

// Run subscribes to the bus and starts sessions whose dependencies are satisfied.
// Blocks until ctx is cancelled or all sessions have reached a terminal state.
func (e *Engine) Run(ctx context.Context) error {
	ch, cancel := e.bus.SubscribeAll()
	defer cancel()

	// Try to start any sessions with no dependencies immediately.
	e.tryStartReady(ctx)

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case ev := <-ch:
			if ev.Type == hooks.EventCompleted || ev.Type == hooks.EventFailed {
				e.mu.Lock()
				e.done[ev.SessionID] = true
				e.mu.Unlock()

				e.tryStartReady(ctx)

				if e.allDone() {
					return nil
				}
			}
		case <-time.After(30 * time.Second):
			// Periodic check in case events were missed.
			e.tryStartReady(ctx)
			if e.allDone() {
				return nil
			}
		}
	}
}

// tryStartReady starts all sessions whose dependencies are satisfied.
func (e *Engine) tryStartReady(ctx context.Context) {
	e.mu.Lock()
	defer e.mu.Unlock()

	for i := range e.specs {
		sp := &e.specs[i]
		if sp.started {
			continue
		}
		if !e.depsMetLocked(sp.DependsOn) {
			continue
		}
		sp.started = true
		id := sp.id
		go func() {
			_ = e.rt.StartSession(ctx, id, e.parentPaneID)
		}()
	}
}

// depsMetLocked reports whether all dependency IDs are in the done set.
// Must be called with e.mu held.
func (e *Engine) depsMetLocked(deps []string) bool {
	for _, dep := range deps {
		if !e.done[dep] {
			return false
		}
	}
	return true
}

// allDone reports whether every scheduled session is in the done set.
func (e *Engine) allDone() bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	for _, sp := range e.specs {
		if !e.done[sp.id] {
			return false
		}
	}
	return true
}
