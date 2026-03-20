package builtin

import (
	"context"
	"crypto/rand"
	"fmt"

	"alex/internal/core/hook"
)

// ResolveSession implements hook.SessionResolver.
// Uses the session ID from TurnState if already set, otherwise generates a new one.
func (p *Plugin) ResolveSession(_ context.Context, state *hook.TurnState) error {
	if state.SessionID != "" {
		return nil
	}
	id, err := generateID()
	if err != nil {
		return fmt.Errorf("builtin: generate session ID: %w", err)
	}
	state.SessionID = id

	if state.RunID == "" {
		runID, err := generateID()
		if err != nil {
			return fmt.Errorf("builtin: generate run ID: %w", err)
		}
		state.RunID = runID
	}
	return nil
}

// generateID creates a random hex ID.
func generateID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return fmt.Sprintf("%x", b), nil
}

var _ hook.SessionResolver = (*Plugin)(nil)
