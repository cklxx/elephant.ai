package tape

import "context"

// TapeManager wraps a TapeStore with contextual defaults (tape name, run ID).
type TapeManager struct {
	store    TapeStore
	tapeCtx  TapeContext
}

// NewTapeManager creates a TapeManager with the given store and context.
func NewTapeManager(store TapeStore, tapeCtx TapeContext) *TapeManager {
	return &TapeManager{
		store:   store,
		tapeCtx: tapeCtx,
	}
}

// Append adds an entry to the current tape, auto-filling meta from context
// when the entry's own meta fields are empty.
func (m *TapeManager) Append(ctx context.Context, entry TapeEntry) error {
	if entry.Meta.SessionID == "" {
		entry.Meta.SessionID = m.tapeCtx.Meta.SessionID
	}
	if entry.Meta.RunID == "" {
		entry.Meta.RunID = m.tapeCtx.RunID
	}
	return m.store.Append(ctx, m.tapeCtx.TapeName, entry)
}

// Query queries the current tape.
func (m *TapeManager) Query(ctx context.Context, q TapeQuery) ([]TapeEntry, error) {
	return m.store.Query(ctx, m.tapeCtx.TapeName, q)
}

// Store returns the underlying store.
func (m *TapeManager) Store() TapeStore {
	return m.store
}

// Context returns the tape context.
func (m *TapeManager) Context() TapeContext {
	return m.tapeCtx
}
