package context

import (
	"context"
	"errors"
	"fmt"
	"sort"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	sessionstate "alex/internal/session/state_store"
)

const historyPageSize = 200

// HistoryManager provides turn-level history persistence and replay with stable
// ordering guarantees. It is designed to live alongside context snapshots so
// that history retrieval uses a single module.
type HistoryManager struct {
	store  sessionstate.Store
	logger agent.Logger
	clock  agent.Clock
}

// NewHistoryManager constructs a HistoryManager backed by the provided snapshot
// store.
func NewHistoryManager(store sessionstate.Store, logger agent.Logger, clock agent.Clock) *HistoryManager {
	if logger == nil {
		logger = agent.NoopLogger{}
	}
	if clock == nil {
		clock = agent.SystemClock{}
	}
	return &HistoryManager{
		store:  store,
		logger: logger,
		clock:  clock,
	}
}

// AppendTurn saves the newly added turn detected within the provided
// conversation messages. It detects the stable prefix from previously
// recorded turns and only stores the delta as the next turn. If the prefix
// diverges, it resets the stored history and records the supplied messages as
// the first turn to keep downstream replay consistent.
func (m *HistoryManager) AppendTurn(ctx context.Context, sessionID string, messages []ports.Message) error {
	if m == nil || m.store == nil || sessionID == "" || len(messages) == 0 {
		return nil
	}

	snapshots, err := m.listSnapshots(ctx, sessionID)
	if err != nil {
		return err
	}

	flattened := flattenSnapshots(snapshots)
	commonPrefix := commonPrefixLen(flattened, messages)

	if commonPrefix < len(flattened) {
		if err := m.store.ClearSession(ctx, sessionID); err != nil {
			return err
		}
		snapshots = nil
		commonPrefix = 0
		if m.logger != nil {
			m.logger.Warn("Resetting history for session %s due to prefix divergence", sessionID)
		}
	}

	newTurnMessages := agent.CloneMessages(messages[commonPrefix:])
	if len(newTurnMessages) == 0 {
		return nil
	}

	turnID := len(snapshots) + 1
	snapshot := sessionstate.Snapshot{
		SessionID: sessionID,
		TurnID:    turnID,
		CreatedAt: m.clock.Now(),
		Messages:  newTurnMessages,
	}

	return m.store.SaveSnapshot(ctx, snapshot)
}

// Replay flattens the recorded turns for the provided session. If uptoTurn is
// greater than zero, only turns with IDs less than uptoTurn are included.
func (m *HistoryManager) Replay(ctx context.Context, sessionID string, uptoTurn int) ([]ports.Message, error) {
	if m == nil || m.store == nil || sessionID == "" {
		return nil, nil
	}

	snapshots, err := m.listSnapshots(ctx, sessionID)
	if err != nil {
		if errors.Is(err, sessionstate.ErrSnapshotNotFound) {
			return nil, nil
		}
		return nil, err
	}

	if uptoTurn > 0 {
		filtered := make([]sessionstate.Snapshot, 0, len(snapshots))
		for _, snap := range snapshots {
			if snap.TurnID < uptoTurn {
				filtered = append(filtered, snap)
			}
		}
		snapshots = filtered
	}

	if len(snapshots) == 0 {
		return nil, nil
	}

	return agent.CloneMessages(flattenSnapshots(snapshots)), nil
}

// ClearSession removes all stored turns for the session.
func (m *HistoryManager) ClearSession(ctx context.Context, sessionID string) error {
	if m == nil || m.store == nil || sessionID == "" {
		return nil
	}
	return m.store.ClearSession(ctx, sessionID)
}

func (m *HistoryManager) listSnapshots(ctx context.Context, sessionID string) ([]sessionstate.Snapshot, error) {
	cursor := ""
	var snapshots []sessionstate.Snapshot

	for {
		metas, next, err := m.store.ListSnapshots(ctx, sessionID, cursor, historyPageSize)
		if err != nil {
			return nil, err
		}
		if len(metas) == 0 {
			break
		}
		for _, meta := range metas {
			snap, err := m.store.GetSnapshot(ctx, sessionID, meta.TurnID)
			if err != nil {
				if errors.Is(err, sessionstate.ErrSnapshotNotFound) {
					continue
				}
				return nil, err
			}
			snapshots = append(snapshots, snap)
		}
		if next == "" {
			break
		}
		cursor = next
	}

	if len(snapshots) == 0 {
		return nil, nil
	}

	sort.Slice(snapshots, func(i, j int) bool {
		return snapshots[i].TurnID < snapshots[j].TurnID
	})

	return snapshots, nil
}

func flattenSnapshots(snapshots []sessionstate.Snapshot) []ports.Message {
	if len(snapshots) == 0 {
		return nil
	}
	var messages []ports.Message
	for _, snap := range snapshots {
		if len(snap.Messages) == 0 {
			continue
		}
		messages = append(messages, snap.Messages...)
	}
	return messages
}

func commonPrefixLen(existing []ports.Message, incoming []ports.Message) int {
	limit := len(existing)
	if len(incoming) < limit {
		limit = len(incoming)
	}
	for i := 0; i < limit; i++ {
		if !messagesEqual(existing[i], incoming[i]) {
			return i
		}
	}
	return limit
}

func messagesEqual(a, b ports.Message) bool {
	if a.Role != b.Role ||
		a.Content != b.Content ||
		a.ToolCallID != b.ToolCallID ||
		a.Source != b.Source {
		return false
	}

	if !thinkingEqual(a.Thinking, b.Thinking) {
		return false
	}

	if !toolCallsEqual(a.ToolCalls, b.ToolCalls) {
		return false
	}

	if !toolResultsEqual(a.ToolResults, b.ToolResults) {
		return false
	}

	if !mapStringAnyEqual(a.Metadata, b.Metadata) {
		return false
	}

	if !attachmentsEqual(a.Attachments, b.Attachments) {
		return false
	}

	return true
}

func toolCallsEqual(a, b []ports.ToolCall) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ID != b[i].ID ||
			a[i].Name != b[i].Name ||
			a[i].SessionID != b[i].SessionID ||
			a[i].TaskID != b[i].TaskID ||
			a[i].ParentTaskID != b[i].ParentTaskID {
			return false
		}
		if !mapStringAnyEqual(a[i].Arguments, b[i].Arguments) {
			return false
		}
	}
	return true
}

func toolResultsEqual(a, b []ports.ToolResult) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].CallID != b[i].CallID ||
			a[i].Content != b[i].Content ||
			a[i].SessionID != b[i].SessionID {
			return false
		}
		// Compare errors properly using errors.Is
		if !errorsEqual(a[i].Error, b[i].Error) {
			return false
		}
		if !mapStringAnyEqual(a[i].Metadata, b[i].Metadata) {
			return false
		}
	}
	return true
}

func errorsEqual(a, b error) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	// For error comparison, check if one wraps the other or compare messages
	if errors.Is(a, b) || errors.Is(b, a) {
		return true
	}
	return a.Error() == b.Error()
}

func attachmentsEqual(a, b map[string]ports.Attachment) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !attachmentEqual(va, vb) {
			return false
		}
	}
	return true
}

func thinkingEqual(a, b ports.Thinking) bool {
	if len(a.Parts) != len(b.Parts) {
		return false
	}
	for i := range a.Parts {
		if a.Parts[i] != b.Parts[i] {
			return false
		}
	}
	return true
}

func attachmentEqual(a, b ports.Attachment) bool {
	if a.Name != b.Name ||
		a.MediaType != b.MediaType ||
		a.Data != b.Data ||
		a.URI != b.URI ||
		a.Fingerprint != b.Fingerprint ||
		a.Source != b.Source ||
		a.Description != b.Description ||
		a.Kind != b.Kind ||
		a.Format != b.Format ||
		a.PreviewProfile != b.PreviewProfile ||
		a.RetentionTTLSeconds != b.RetentionTTLSeconds {
		return false
	}
	if len(a.PreviewAssets) != len(b.PreviewAssets) {
		return false
	}
	for i := range a.PreviewAssets {
		if a.PreviewAssets[i] != b.PreviewAssets[i] {
			return false
		}
	}
	return true
}

func mapStringAnyEqual(a, b map[string]any) bool {
	if len(a) != len(b) {
		return false
	}
	for k, va := range a {
		vb, ok := b[k]
		if !ok {
			return false
		}
		if !anyValueEqual(va, vb) {
			return false
		}
	}
	return true
}

func anyValueEqual(a, b any) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	switch va := a.(type) {
	case string:
		vb, ok := b.(string)
		return ok && va == vb
	case float64:
		vb, ok := b.(float64)
		return ok && va == vb
	case bool:
		vb, ok := b.(bool)
		return ok && va == vb
	case int:
		vb, ok := b.(int)
		return ok && va == vb
	case int64:
		vb, ok := b.(int64)
		return ok && va == vb
	case []any:
		vb, ok := b.([]any)
		if !ok || len(va) != len(vb) {
			return false
		}
		for i := range va {
			if !anyValueEqual(va[i], vb[i]) {
				return false
			}
		}
		return true
	case map[string]any:
		vb, ok := b.(map[string]any)
		if !ok {
			return false
		}
		return mapStringAnyEqual(va, vb)
	default:
		return fmt.Sprint(a) == fmt.Sprint(b)
	}
}
