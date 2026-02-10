package task

import (
	"context"
	"encoding/json"
	"time"

	"alex/internal/delivery/channels/lark"
	taskdomain "alex/internal/domain/task"
)

// LarkAdapter implements lark.TaskStore by delegating to the unified task store.
type LarkAdapter struct {
	store taskdomain.Store
}

var _ lark.TaskStore = (*LarkAdapter)(nil)

// NewLarkAdapter wraps a unified task store to satisfy the Lark gateway's TaskStore port.
func NewLarkAdapter(store taskdomain.Store) *LarkAdapter {
	return &LarkAdapter{store: store}
}

// EnsureSchema delegates to the unified store.
func (a *LarkAdapter) EnsureSchema(ctx context.Context) error {
	return a.store.EnsureSchema(ctx)
}

// SaveTask persists a Lark task record by converting to the unified domain model.
func (a *LarkAdapter) SaveTask(ctx context.Context, rec lark.TaskRecord) error {
	t := &taskdomain.Task{
		TaskID:      rec.TaskID,
		Channel:     "lark",
		ChatID:      rec.ChatID,
		UserID:      rec.UserID,
		AgentType:   rec.AgentType,
		Description: rec.Description,
		Status:      taskdomain.Status(rec.Status),
		CreatedAt:   rec.CreatedAt,
		UpdatedAt:   rec.UpdatedAt,
	}
	if !rec.CompletedAt.IsZero() {
		t.CompletedAt = &rec.CompletedAt
	}
	if rec.AnswerPreview != "" {
		t.AnswerPreview = rec.AnswerPreview
	}
	if rec.Error != "" {
		t.Error = rec.Error
	}
	if rec.TokensUsed > 0 {
		t.TokensUsed = rec.TokensUsed
	}

	return a.store.Create(ctx, t)
}

// UpdateStatus updates the status (and optional fields) for a task.
func (a *LarkAdapter) UpdateStatus(ctx context.Context, taskID, status string, opts ...lark.TaskUpdateOption) error {
	// Resolve Lark options into exported values.
	vals := lark.ResolveTaskUpdateOptions(opts)

	var transOpts []taskdomain.TransitionOption
	if vals.AnswerPreview != nil {
		transOpts = append(transOpts, taskdomain.WithTransitionAnswerPreview(*vals.AnswerPreview))
	}
	if vals.ErrorText != nil {
		transOpts = append(transOpts, taskdomain.WithTransitionError(*vals.ErrorText))
	}
	if vals.TokensUsed != nil {
		transOpts = append(transOpts, taskdomain.WithTransitionTokens(*vals.TokensUsed))
	}

	return a.store.SetStatus(ctx, taskID, taskdomain.Status(status), transOpts...)
}

// GetTask fetches a single task by ID and converts to Lark format.
func (a *LarkAdapter) GetTask(ctx context.Context, taskID string) (lark.TaskRecord, bool, error) {
	t, err := a.store.Get(ctx, taskID)
	if err != nil {
		if isNotFound(err) {
			return lark.TaskRecord{}, false, nil
		}
		return lark.TaskRecord{}, false, err
	}
	return domainToLarkRecord(t), true, nil
}

// ListByChat returns tasks for a chat, optionally filtered to active-only.
func (a *LarkAdapter) ListByChat(ctx context.Context, chatID string, activeOnly bool, limit int) ([]lark.TaskRecord, error) {
	tasks, err := a.store.ListByChat(ctx, chatID, activeOnly, limit)
	if err != nil {
		return nil, err
	}
	records := make([]lark.TaskRecord, len(tasks))
	for i, t := range tasks {
		records[i] = domainToLarkRecord(t)
	}
	return records, nil
}

// DeleteExpired removes task records older than the given time.
func (a *LarkAdapter) DeleteExpired(ctx context.Context, before time.Time) error {
	return a.store.DeleteExpired(ctx, before)
}

// MarkStaleRunning marks all running/pending tasks as failed.
func (a *LarkAdapter) MarkStaleRunning(ctx context.Context, reason string) error {
	return a.store.MarkStaleRunning(ctx, reason)
}

// SetBridgeMeta persists bridge subprocess metadata for resilience.
// The info parameter is expected to implement BridgeInfoProvider, or be a
// map[string]any with "pid" and "output_file" keys.
func (a *LarkAdapter) SetBridgeMeta(ctx context.Context, taskID string, info any) error {
	meta := extractBridgeMeta(info)
	return a.store.SetBridgeMeta(ctx, taskID, meta)
}

// BridgeInfoProvider is the interface that bridge info structs should satisfy
// for type-safe extraction. This avoids circular imports between infra/task and
// infra/external/bridge packages.
type BridgeInfoProvider interface {
	BridgePID() int
	BridgeOutputFile() string
}

// extractBridgeMeta converts an opaque bridge info value to a domain BridgeMeta.
func extractBridgeMeta(info any) taskdomain.BridgeMeta {
	var meta taskdomain.BridgeMeta

	if provider, ok := info.(BridgeInfoProvider); ok {
		meta.PID = provider.BridgePID()
		meta.OutputFile = provider.BridgeOutputFile()
		return meta
	}

	// Fallback: JSON round-trip for struct types without the interface.
	if m, ok := info.(map[string]any); ok {
		if pid, ok := m["pid"].(int); ok {
			meta.PID = pid
		}
		if f, ok := m["output_file"].(string); ok {
			meta.OutputFile = f
		}
		return meta
	}

	// Generic struct: JSON round-trip (handles float64 from JSON numbers).
	data, err := json.Marshal(info)
	if err != nil {
		return meta
	}
	var m map[string]any
	if err := json.Unmarshal(data, &m); err != nil {
		return meta
	}
	// BridgeStartedInfo fields: PID (int), OutputFile (string), TaskID (string)
	if pid, ok := m["PID"]; ok {
		switch v := pid.(type) {
		case float64:
			meta.PID = int(v)
		case int:
			meta.PID = v
		}
	}
	if f, ok := m["OutputFile"].(string); ok {
		meta.OutputFile = f
	}

	return meta
}

// ── Conversion helpers ──────────────────────────────────────────────────────

func domainToLarkRecord(t *taskdomain.Task) lark.TaskRecord {
	rec := lark.TaskRecord{
		ChatID:        t.ChatID,
		TaskID:        t.TaskID,
		UserID:        t.UserID,
		AgentType:     t.AgentType,
		Description:   t.Description,
		Status:        string(t.Status),
		CreatedAt:     t.CreatedAt,
		UpdatedAt:     t.UpdatedAt,
		AnswerPreview: t.AnswerPreview,
		Error:         t.Error,
		TokensUsed:    t.TokensUsed,
	}
	if t.CompletedAt != nil {
		rec.CompletedAt = *t.CompletedAt
	}
	return rec
}
