package context

import (
	"context"
	"errors"
	"reflect"
	"sort"

	"alex/internal/agent/ports"
	sessionstate "alex/internal/session/state_store"
)

const historyPageSize = 200

// HistoryManager provides turn-level history persistence and replay with stable
// ordering guarantees. It is designed to live alongside context snapshots so
// that history retrieval uses a single module.
type HistoryManager struct {
	store  sessionstate.Store
	logger ports.Logger
	clock  ports.Clock
}

// NewHistoryManager constructs a HistoryManager backed by the provided snapshot
// store.
func NewHistoryManager(store sessionstate.Store, logger ports.Logger, clock ports.Clock) *HistoryManager {
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	if clock == nil {
		clock = ports.SystemClock{}
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

	newTurnMessages := ports.CloneMessages(messages[commonPrefix:])
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

	return ports.CloneMessages(flattenSnapshots(snapshots)), nil
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
	return reflect.DeepEqual(a, b)
}
