package tape

import (
	"context"
	"encoding/json"

	coretape "alex/internal/core/tape"
	"alex/internal/domain/agent/ports"
)

// TurnRecorder writes per-message audit entries to a TapeManager. All writes
// are fire-and-forget: errors are returned but callers should log rather than
// fail the iteration.
type TurnRecorder struct {
	mgr *coretape.TapeManager
}

// NewTurnRecorder returns a TurnRecorder backed by the given TapeManager.
func NewTurnRecorder(mgr *coretape.TapeManager) *TurnRecorder {
	return &TurnRecorder{mgr: mgr}
}

// RecordMessage appends a full-fidelity message entry to the tape.
func (r *TurnRecorder) RecordMessage(ctx context.Context, msg ports.Message, iteration int) error {
	payload := map[string]any{
		"role":      msg.Role,
		"content":   msg.Content,
		"iteration": iteration,
	}
	if msg.Source != "" {
		payload["source"] = string(msg.Source)
	}
	if len(msg.ToolCalls) > 0 {
		payload["tool_calls"] = jsonRT(msg.ToolCalls)
	}
	if len(msg.ToolResults) > 0 {
		payload["tool_results"] = jsonRT(msg.ToolResults)
	}
	if msg.ToolCallID != "" {
		payload["tool_call_id"] = msg.ToolCallID
	}
	if len(msg.Attachments) > 0 {
		payload["attachments"] = jsonRT(msg.Attachments)
	}
	if len(msg.Thinking.Parts) > 0 {
		payload["thinking"] = jsonRT(msg.Thinking)
	}
	if len(msg.Metadata) > 0 {
		payload["metadata"] = jsonRT(msg.Metadata)
	}
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
		e := map[string]any{
			"role":    msg.Role,
			"content": msg.Content,
		}
		if msg.Source != "" {
			e["source"] = string(msg.Source)
		}
		if len(msg.ToolCalls) > 0 {
			e["tool_calls"] = jsonRT(msg.ToolCalls)
		}
		entries = append(entries, e)
	}
	entry := coretape.NewCompactionArtifact(entries, meta, coretape.EntryMeta{})
	return r.mgr.Append(ctx, entry)
}

func jsonRT(v any) any {
	data, err := json.Marshal(v)
	if err != nil {
		return nil
	}
	var out any
	_ = json.Unmarshal(data, &out)
	return out
}
