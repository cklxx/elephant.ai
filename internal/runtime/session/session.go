// Package session defines the core runtime session model and state machine.
// A Session represents one member CLI process managed by the Kaku runtime.
package session

import (
	"fmt"
	"sync"
	"time"
)

// State is the lifecycle state of a runtime session.
type State string

const (
	StateCreated      State = "created"
	StateStarting     State = "starting"
	StateRunning      State = "running"
	StateNeedsInput   State = "needs_input"
	StateStalled      State = "stalled"
	StateCompleted    State = "completed"
	StateFailed       State = "failed"
	StateCancelled    State = "cancelled"
)

// MemberType identifies which CLI member is running in the session.
type MemberType string

const (
	MemberClaudeCode MemberType = "claude_code"
	MemberCodex      MemberType = "codex"
	MemberKimi       MemberType = "kimi"
	MemberShell      MemberType = "shell"
)

// SessionData holds the serialisable fields of a session without any
// synchronisation primitives. It is used for snapshots and JSON persistence.
type SessionData struct {
	ID              string     `json:"id"`
	Member          MemberType `json:"member"`
	Goal            string     `json:"goal"`
	WorkDir         string     `json:"work_dir"`
	State           State      `json:"state"`
	PaneID          int        `json:"pane_id"`                    // Kaku pane ID (-1 if not yet assigned)
	TabID           int        `json:"tab_id"`                     // Kaku tab ID (-1 if not yet assigned)
	PoolPane        bool       `json:"pool_pane,omitempty"`        // true if pane acquired from pool
	ParentSessionID string     `json:"parent_session_id,omitempty"` // leader session that spawned this child
	CreatedAt       time.Time  `json:"created_at"`
	UpdatedAt       time.Time  `json:"updated_at"`
	StartedAt       *time.Time `json:"started_at,omitempty"`
	EndedAt         *time.Time `json:"ended_at,omitempty"`
	LastHeartbeat   *time.Time `json:"last_heartbeat,omitempty"`
	ErrorMsg        string     `json:"error_msg,omitempty"`
	Answer          string     `json:"answer,omitempty"`
}

// Session is the unit of execution in the Kaku runtime.
// It tracks one member CLI process and its metadata.
type Session struct {
	mu sync.RWMutex
	SessionData
}

// New creates a new Session in the Created state.
func New(id string, member MemberType, goal, workDir string) *Session {
	now := time.Now()
	return &Session{
		SessionData: SessionData{
			ID:        id,
			Member:    member,
			Goal:      goal,
			WorkDir:   workDir,
			State:     StateCreated,
			PaneID:    -1,
			TabID:     -1,
			CreatedAt: now,
			UpdatedAt: now,
		},
	}
}

// Transition moves the session to the target state if the transition is valid.
// Returns an error if the transition is not allowed.
func (s *Session) Transition(target State) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !isValidTransition(s.State, target) {
		return fmt.Errorf("invalid state transition: %s → %s", s.State, target)
	}

	now := time.Now()
	s.State = target
	s.UpdatedAt = now

	switch target {
	case StateRunning:
		if s.StartedAt == nil {
			s.StartedAt = &now
		}
	case StateCompleted, StateFailed, StateCancelled:
		s.EndedAt = &now
	}
	return nil
}

// RecordHeartbeat updates the last heartbeat timestamp.
func (s *Session) RecordHeartbeat() {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	s.LastHeartbeat = &now
	s.UpdatedAt = now
}

// SetPane assigns the Kaku pane and tab IDs.
func (s *Session) SetPane(paneID, tabID int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PaneID = paneID
	s.TabID = tabID
	s.UpdatedAt = time.Now()
}

// SetParentSession records the parent (leader) session ID.
func (s *Session) SetParentSession(parentID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ParentSessionID = parentID
	s.UpdatedAt = time.Now()
}

// SetPoolPane marks whether the pane was acquired from the pool.
func (s *Session) SetPoolPane(pool bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.PoolPane = pool
	s.UpdatedAt = time.Now()
}

// SetResult records the final answer (for completed sessions).
func (s *Session) SetResult(answer string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Answer = answer
	s.UpdatedAt = time.Now()
}

// SetError records an error message (for failed sessions).
func (s *Session) SetError(msg string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ErrorMsg = msg
	s.UpdatedAt = time.Now()
}

// Snapshot returns a copy of the session data safe for reading without holding the lock.
func (s *Session) Snapshot() SessionData {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.SessionData
}

// validTransitions defines the allowed state machine edges.
// Hoisted to package level to avoid re-allocating the map on every call.
var validTransitions = map[State][]State{
	StateCreated:    {StateStarting, StateCancelled},
	StateStarting:   {StateRunning, StateFailed, StateCancelled},
	StateRunning:    {StateNeedsInput, StateStalled, StateCompleted, StateFailed, StateCancelled},
	StateNeedsInput: {StateRunning, StateStalled, StateCancelled},
	StateStalled:    {StateRunning, StateCancelled, StateFailed},
	// terminal states — no further transitions
	StateCompleted: {},
	StateFailed:    {},
	StateCancelled: {},
}

// isValidTransition checks whether from→to is an allowed state machine edge.
func isValidTransition(from, to State) bool {
	for _, t := range validTransitions[from] {
		if t == to {
			return true
		}
	}
	return false
}

// IsTerminal reports whether the session has reached a terminal state.
func (s *Session) IsTerminal() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return isTerminal(s.State)
}

func isTerminal(st State) bool {
	return st == StateCompleted || st == StateFailed || st == StateCancelled
}

// IsStalled reports whether the session has had no heartbeat for longer than threshold.
func (s *Session) IsStalled(threshold time.Duration) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if isTerminal(s.State) {
		return false
	}
	if s.LastHeartbeat == nil {
		if s.StartedAt == nil {
			return false
		}
		return time.Since(*s.StartedAt) > threshold
	}
	return time.Since(*s.LastHeartbeat) > threshold
}
