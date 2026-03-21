package tape

import (
	"context"

	coretape "alex/internal/core/tape"
	"alex/internal/domain/agent/ports"
)

// TurnRecorder writes per-message audit entries to a TapeManager. Errors are
// returned but callers should log rather than fail the iteration.
type TurnRecorder struct {
	mgr *coretape.TapeManager
}

// NewTurnRecorder returns a TurnRecorder backed by the given TapeManager.
func NewTurnRecorder(mgr *coretape.TapeManager) *TurnRecorder {
	return &TurnRecorder{mgr: mgr}
}

// RecordMessage appends a message entry to the tape so that tape replay can
// reconstruct the exact LLM context window including Thinking and Metadata.
func (r *TurnRecorder) RecordMessage(ctx context.Context, msg ports.Message, iteration int) error {
	payload := messageToPayload(msg)
	payload["iteration"] = iteration
	entry := coretape.NewMessageFromPayload(payload, coretape.EntryMeta{})
	return r.mgr.Append(ctx, entry)
}

// RecordCompression writes a compression boundary anchor to the tape.
func (r *TurnRecorder) RecordCompression(ctx context.Context, label string, meta map[string]any) error {
	entry := coretape.NewCompression(label, meta, coretape.EntryMeta{})
	return r.mgr.Append(ctx, entry)
}

// RecordCompactionArtifact writes evicted messages as a single tape entry.
func (r *TurnRecorder) RecordCompactionArtifact(ctx context.Context, evicted []ports.Message, meta map[string]any) error {
	entries := make([]map[string]any, 0, len(evicted))
	for _, msg := range evicted {
		entries = append(entries, messageToPayload(msg))
	}
	entry := coretape.NewCompactionArtifact(entries, meta, coretape.EntryMeta{})
	return r.mgr.Append(ctx, entry)
}
