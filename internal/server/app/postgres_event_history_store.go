package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/attachments"
	"alex/internal/logging"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHistoryBatchSize               = 500
	historyInlineAttachmentRetentionLimit = 128 * 1024
	defaultHistoryQueryTimeout            = 5 * time.Second
	defaultHistoryRetentionInterval       = 10 * time.Minute
	defaultHistoryRetentionBatchSize      = 1000
)

type eventRecord struct {
	id            int64
	sessionID     string
	taskID        string
	parentTaskID  string
	agentLevel    string
	eventType     string
	eventTS       time.Time
	envelopeVer   int
	workflowID    string
	runID         string
	nodeID        string
	nodeKind      string
	isSubtask     bool
	subtaskIndex  int
	totalSubtasks int
	subtaskPrev   string
	maxParallel   int
	payload       []byte
}

// AttachmentStorer persists attachment payloads and returns a stable URI.
type AttachmentStorer interface {
	StoreBytes(name, mediaType string, data []byte) (string, error)
}

// PostgresEventHistoryStore persists event history in Postgres.
type PostgresEventHistoryStore struct {
	pool            *pgxpool.Pool
	batchSize       int
	retentionWindow time.Duration
	retentionEvery  time.Duration
	retentionBatch  int
	logger          logging.Logger
	attachmentStore AttachmentStorer
	pruneMu         sync.Mutex
	pruning         bool
	lastPrunedAt    time.Time
}

// PostgresEventHistoryStoreOption configures a PostgresEventHistoryStore.
type PostgresEventHistoryStoreOption func(*PostgresEventHistoryStore)

// WithHistoryAttachmentStore wires an attachment store so inline payloads
// can be persisted during event history writes.
func WithHistoryAttachmentStore(store AttachmentStorer) PostgresEventHistoryStoreOption {
	return func(s *PostgresEventHistoryStore) {
		s.attachmentStore = store
	}
}

// WithHistoryRetention configures the retention window for persisted events.
// A zero or negative duration disables pruning.
func WithHistoryRetention(window time.Duration) PostgresEventHistoryStoreOption {
	return func(s *PostgresEventHistoryStore) {
		if window <= 0 {
			s.retentionWindow = 0
			return
		}
		s.retentionWindow = window
	}
}

// WithHistoryRetentionInterval controls how often the store attempts retention pruning.
func WithHistoryRetentionInterval(interval time.Duration) PostgresEventHistoryStoreOption {
	return func(s *PostgresEventHistoryStore) {
		if interval > 0 {
			s.retentionEvery = interval
		}
	}
}

// WithHistoryRetentionBatchSize limits how many rows are removed per prune pass.
func WithHistoryRetentionBatchSize(batch int) PostgresEventHistoryStoreOption {
	return func(s *PostgresEventHistoryStore) {
		if batch > 0 {
			s.retentionBatch = batch
		}
	}
}

// NewPostgresEventHistoryStore constructs a Postgres-backed history store.
func NewPostgresEventHistoryStore(pool *pgxpool.Pool, opts ...PostgresEventHistoryStoreOption) *PostgresEventHistoryStore {
	store := &PostgresEventHistoryStore{
		pool:           pool,
		batchSize:      defaultHistoryBatchSize,
		retentionEvery: defaultHistoryRetentionInterval,
		retentionBatch: defaultHistoryRetentionBatchSize,
		logger:         logging.NewComponentLogger("EventHistoryStore"),
	}
	for _, opt := range opts {
		if opt != nil {
			opt(store)
		}
	}
	return store
}

func (s *PostgresEventHistoryStore) withTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if ctx == nil {
		ctx = context.Background()
	}
	return context.WithTimeout(ctx, defaultHistoryQueryTimeout)
}

// EnsureSchema creates the event history table if needed.
func (s *PostgresEventHistoryStore) EnsureSchema(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("history store not initialized")
	}
	statements := []string{
		`CREATE TABLE IF NOT EXISTS agent_session_events (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    task_id TEXT NOT NULL DEFAULT '',
    parent_task_id TEXT NOT NULL DEFAULT '',
    agent_level TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL,
    event_ts TIMESTAMPTZ NOT NULL,
    envelope_version INTEGER NOT NULL DEFAULT 0,
    workflow_id TEXT NOT NULL DEFAULT '',
    run_id TEXT NOT NULL DEFAULT '',
    node_id TEXT NOT NULL DEFAULT '',
    node_kind TEXT NOT NULL DEFAULT '',
    is_subtask BOOLEAN NOT NULL DEFAULT FALSE,
    subtask_index INTEGER NOT NULL DEFAULT 0,
    total_subtasks INTEGER NOT NULL DEFAULT 0,
    subtask_preview TEXT NOT NULL DEFAULT '',
    max_parallel INTEGER NOT NULL DEFAULT 0,
    payload JSONB
);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_session ON agent_session_events (session_id, id);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_type ON agent_session_events (event_type, id);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_session_type ON agent_session_events (session_id, event_type, id);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_session_ts ON agent_session_events (session_id, event_ts DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_type_ts ON agent_session_events (event_type, event_ts DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_ts ON agent_session_events (event_ts DESC);`,
		`CREATE INDEX IF NOT EXISTS idx_agent_session_events_payload_gin ON agent_session_events USING GIN (payload);`,
	}

	for _, stmt := range statements {
		if _, err := s.pool.Exec(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}

// Append persists a new event.
func (s *PostgresEventHistoryStore) Append(ctx context.Context, event ports.AgentEvent) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("history store not initialized")
	}
	if event == nil {
		return nil
	}

	record, err := recordFromEventWithStore(BaseAgentEvent(event), s.attachmentStore)
	if err != nil {
		return err
	}

	var payloadParam any
	if len(record.payload) > 0 {
		payloadParam = record.payload
	}

	ctxWithTimeout, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err = s.pool.Exec(ctxWithTimeout, `
INSERT INTO agent_session_events (
    session_id, task_id, parent_task_id, agent_level, event_type, event_ts,
    envelope_version, workflow_id, run_id, node_id, node_kind,
    is_subtask, subtask_index, total_subtasks, subtask_preview, max_parallel, payload
) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17::jsonb)
`,
		record.sessionID,
		record.taskID,
		record.parentTaskID,
		record.agentLevel,
		record.eventType,
		record.eventTS,
		record.envelopeVer,
		record.workflowID,
		record.runID,
		record.nodeID,
		record.nodeKind,
		record.isSubtask,
		record.subtaskIndex,
		record.totalSubtasks,
		record.subtaskPrev,
		record.maxParallel,
		payloadParam,
	)
	if err != nil {
		return err
	}
	s.pruneIfNeeded()
	return nil
}

// AppendBatch persists a group of events in a single statement.
// It is intended for non-latency-sensitive background flushers.
func (s *PostgresEventHistoryStore) AppendBatch(ctx context.Context, events []ports.AgentEvent) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("history store not initialized")
	}
	if len(events) == 0 {
		return nil
	}

	records := make([]eventRecord, 0, len(events))
	for _, event := range events {
		if event == nil {
			continue
		}
		record, err := recordFromEventWithStore(BaseAgentEvent(event), s.attachmentStore)
		if err != nil {
			return err
		}
		records = append(records, record)
	}
	if len(records) == 0 {
		return nil
	}

	const columns = `session_id, task_id, parent_task_id, agent_level, event_type, event_ts,
    envelope_version, workflow_id, run_id, node_id, node_kind,
    is_subtask, subtask_index, total_subtasks, subtask_preview, max_parallel, payload`

	args := make([]any, 0, len(records)*17)
	var sb strings.Builder
	sb.WriteString("INSERT INTO agent_session_events (")
	sb.WriteString(columns)
	sb.WriteString(") VALUES ")

	argPos := 1
	for idx, record := range records {
		if idx > 0 {
			sb.WriteString(",")
		}
		sb.WriteString("(")
		for i := 0; i < 17; i++ {
			if i > 0 {
				sb.WriteString(",")
			}
			sb.WriteString(fmt.Sprintf("$%d", argPos))
			argPos++
		}
		sb.WriteString(")")

		var payloadParam any
		if len(record.payload) > 0 {
			payloadParam = record.payload
		}
		args = append(args,
			record.sessionID,
			record.taskID,
			record.parentTaskID,
			record.agentLevel,
			record.eventType,
			record.eventTS,
			record.envelopeVer,
			record.workflowID,
			record.runID,
			record.nodeID,
			record.nodeKind,
			record.isSubtask,
			record.subtaskIndex,
			record.totalSubtasks,
			record.subtaskPrev,
			record.maxParallel,
			payloadParam,
		)
	}

	sb.WriteString(";")
	ctxWithTimeout, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(ctxWithTimeout, sb.String(), args...)
	if err != nil {
		return err
	}
	s.pruneIfNeeded()
	return nil
}

// Stream replays events matching the filter in order.
func (s *PostgresEventHistoryStore) Stream(ctx context.Context, filter EventHistoryFilter, fn func(ports.AgentEvent) error) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("history store not initialized")
	}
	if fn == nil {
		return nil
	}

	afterID := int64(0)
	for {
		records, err := s.fetchBatch(ctx, filter, afterID)
		if err != nil {
			return err
		}
		if len(records) == 0 {
			break
		}
		for _, record := range records {
			if ctx.Err() != nil {
				return ctx.Err()
			}
			event, err := eventFromRecord(record)
			if err != nil {
				logging.OrNop(s.logger).Warn("Failed to decode event history row %d: %v", record.id, err)
				continue
			}
			if event == nil {
				continue
			}
			if err := fn(event); err != nil {
				return err
			}
			afterID = record.id
		}
		if len(records) < s.batchSize {
			break
		}
	}
	return nil
}

// DeleteSession removes event history for a session.
func (s *PostgresEventHistoryStore) DeleteSession(ctx context.Context, sessionID string) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("history store not initialized")
	}
	ctxWithTimeout, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(ctxWithTimeout, `DELETE FROM agent_session_events WHERE session_id = $1`, sessionID)
	return err
}

// HasSessionEvents checks if session history exists.
func (s *PostgresEventHistoryStore) HasSessionEvents(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.pool == nil {
		return false, fmt.Errorf("history store not initialized")
	}
	var exists bool
	ctxWithTimeout, cancel := s.withTimeout(ctx)
	defer cancel()

	err := s.pool.QueryRow(ctxWithTimeout, `SELECT EXISTS(SELECT 1 FROM agent_session_events WHERE session_id = $1)`, sessionID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
}

// Prune removes event history older than the configured retention window.
func (s *PostgresEventHistoryStore) Prune(ctx context.Context) error {
	if s == nil || s.pool == nil {
		return fmt.Errorf("history store not initialized")
	}
	if s.retentionWindow <= 0 {
		return nil
	}
	return s.pruneOnce(ctx)
}

func (s *PostgresEventHistoryStore) fetchBatch(ctx context.Context, filter EventHistoryFilter, afterID int64) ([]eventRecord, error) {
	args := []any{filter.SessionID, afterID}
	query := `
SELECT id, session_id, task_id, parent_task_id, agent_level, event_type, event_ts,
       envelope_version, workflow_id, run_id, node_id, node_kind,
       is_subtask, subtask_index, total_subtasks, subtask_preview, max_parallel, payload
FROM agent_session_events
WHERE session_id = $1 AND id > $2`

	if len(filter.EventTypes) > 0 {
		args = append(args, filter.EventTypes)
		query += fmt.Sprintf(" AND event_type = ANY($%d)", len(args))
	}

	args = append(args, s.batchSize)
	query += fmt.Sprintf(" ORDER BY id ASC LIMIT $%d", len(args))

	ctxWithTimeout, cancel := s.withTimeout(ctx)
	defer cancel()

	rows, err := s.pool.Query(ctxWithTimeout, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// Pre-allocate with batchSize capacity to avoid repeated slice growth
	records := make([]eventRecord, 0, s.batchSize)
	for rows.Next() {
		var rec eventRecord
		var payload []byte
		if err := rows.Scan(
			&rec.id,
			&rec.sessionID,
			&rec.taskID,
			&rec.parentTaskID,
			&rec.agentLevel,
			&rec.eventType,
			&rec.eventTS,
			&rec.envelopeVer,
			&rec.workflowID,
			&rec.runID,
			&rec.nodeID,
			&rec.nodeKind,
			&rec.isSubtask,
			&rec.subtaskIndex,
			&rec.totalSubtasks,
			&rec.subtaskPrev,
			&rec.maxParallel,
			&payload,
		); err != nil {
			return nil, err
		}
		if len(payload) > 0 {
			rec.payload = payload
		}
		records = append(records, rec)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return records, nil
}

func (s *PostgresEventHistoryStore) pruneIfNeeded() {
	if s == nil || s.pool == nil || s.retentionWindow <= 0 {
		return
	}
	now := time.Now()

	s.pruneMu.Lock()
	if s.pruning {
		s.pruneMu.Unlock()
		return
	}
	if !s.lastPrunedAt.IsZero() && now.Sub(s.lastPrunedAt) < s.retentionEvery {
		s.pruneMu.Unlock()
		return
	}
	s.pruning = true
	s.lastPrunedAt = now
	s.pruneMu.Unlock()

	go func() {
		ctx, cancel := s.withTimeout(context.Background())
		defer cancel()

		if err := s.pruneOnce(ctx); err != nil {
			logging.OrNop(s.logger).Warn("Failed to prune event history: %v", err)
		}
		s.pruneMu.Lock()
		s.pruning = false
		s.pruneMu.Unlock()
	}()
}

func (s *PostgresEventHistoryStore) pruneOnce(ctx context.Context) error {
	if s.retentionWindow <= 0 {
		return nil
	}
	cutoff := time.Now().Add(-s.retentionWindow)
	batch := s.retentionBatch
	if batch <= 0 {
		batch = defaultHistoryRetentionBatchSize
	}

	ctxWithTimeout, cancel := s.withTimeout(ctx)
	defer cancel()

	_, err := s.pool.Exec(ctxWithTimeout, `
DELETE FROM agent_session_events
WHERE id IN (
    SELECT id
    FROM agent_session_events
    WHERE event_ts < $1
    ORDER BY event_ts ASC
    LIMIT $2
)`, cutoff, batch)
	return err
}

func recordFromEvent(event ports.AgentEvent) (eventRecord, error) {
	return recordFromEventWithStore(event, nil)
}

func recordFromEventWithStore(event ports.AgentEvent, store AttachmentStorer) (eventRecord, error) {
	if event == nil {
		return eventRecord{}, fmt.Errorf("event is nil")
	}
	base := BaseAgentEvent(event)
	if base == nil {
		return eventRecord{}, fmt.Errorf("event missing base")
	}

	ts := event.Timestamp()
	if ts.IsZero() {
		ts = time.Now()
	}

	agentLevel := event.GetAgentLevel()
	if agentLevel == "" {
		agentLevel = base.GetAgentLevel()
	}

	record := eventRecord{
		sessionID:    event.GetSessionID(),
		taskID:       event.GetTaskID(),
		parentTaskID: event.GetParentTaskID(),
		agentLevel:   string(agentLevel),
		eventType:    base.EventType(),
		eventTS:      ts,
	}

	hasSubtaskWrapper := false
	if wrapper, ok := event.(ports.SubtaskWrapper); ok && wrapper != nil {
		meta := wrapper.SubtaskDetails()
		record.isSubtask = true
		record.subtaskIndex = meta.Index
		record.totalSubtasks = meta.Total
		record.subtaskPrev = meta.Preview
		record.maxParallel = meta.MaxParallel
		hasSubtaskWrapper = true
	}

	var payload any
	switch e := base.(type) {
	case *domain.WorkflowEventEnvelope:
		record.envelopeVer = e.Version
		if record.envelopeVer == 0 {
			record.envelopeVer = 1
		}
		record.workflowID = e.WorkflowID
		record.runID = e.RunID
		record.nodeID = e.NodeID
		record.nodeKind = e.NodeKind
		if !hasSubtaskWrapper {
			record.isSubtask = e.IsSubtask
			record.subtaskIndex = e.SubtaskIndex
			record.totalSubtasks = e.TotalSubtasks
			record.subtaskPrev = e.SubtaskPreview
			record.maxParallel = e.MaxParallel
		}
		payload = stripBinaryPayloadsWithStore(e.Payload, store)
	case *domain.WorkflowInputReceivedEvent:
		payload = map[string]any{
			"task":        e.Task,
			"attachments": stripBinaryPayloadsWithStore(e.Attachments, store),
		}
	case *domain.WorkflowDiagnosticContextSnapshotEvent:
		payload = map[string]any{
			"iteration":    e.Iteration,
			"llm_turn_seq": e.LLMTurnSeq,
			"request_id":   e.RequestID,
			"messages":     e.Messages,
			"excluded":     e.Excluded,
		}
	}

	if payload != nil {
		data, err := json.Marshal(payload)
		if err != nil {
			return eventRecord{}, fmt.Errorf("encode payload: %w", err)
		}
		record.payload = data
	}

	return record, nil
}

func stripBinaryPayloadsWithStore(value any, store AttachmentStorer) any {
	switch v := value.(type) {
	case nil:
		return nil
	case ports.Attachment:
		return sanitizeAttachmentForHistoryWithStore(v, store)
	case *ports.Attachment:
		if v == nil {
			return nil
		}
		cleaned := sanitizeAttachmentForHistoryWithStore(*v, store)
		return &cleaned
	case map[string]ports.Attachment:
		cleaned := make(map[string]ports.Attachment, len(v))
		for key, att := range v {
			cleaned[key] = sanitizeAttachmentForHistoryWithStore(att, store)
		}
		return cleaned
	case []ports.Attachment:
		cleaned := make([]ports.Attachment, len(v))
		for i, att := range v {
			cleaned[i] = sanitizeAttachmentForHistoryWithStore(att, store)
		}
		return cleaned
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for key, val := range v {
			cleaned[key] = stripBinaryPayloadsWithStore(val, store)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(v))
		for i, val := range v {
			cleaned[i] = stripBinaryPayloadsWithStore(val, store)
		}
		return cleaned
	}

	rv := reflect.ValueOf(value)
	if rv.IsValid() && rv.Kind() == reflect.Slice && rv.Type().Elem().Kind() == reflect.Uint8 {
		// Avoid persisting raw byte blobs.
		return nil
	}

	return value
}

func sanitizeAttachmentForHistoryWithStore(att ports.Attachment, store AttachmentStorer) ports.Attachment {
	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
		att.MediaType = mediaType
	}

	trimmedURI := strings.TrimSpace(att.URI)
	if att.Data == "" && trimmedURI != "" && !strings.HasPrefix(strings.ToLower(trimmedURI), "data:") {
		return att
	}

	inline := strings.TrimSpace(ports.AttachmentInlineBase64(att))
	if inline != "" {
		size := base64.StdEncoding.DecodedLen(len(inline))
		if shouldRetainInlinePayload(mediaType, size) {
			att.Data = inline
			// Avoid persisting redundant data URIs; keep the base64-only payload.
			if strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.URI)), "data:") {
				att.URI = ""
			}
			return att
		}
		if store != nil {
			if decoded, err := attachments.DecodeBase64(inline); err == nil && len(decoded) > 0 {
				uri, err := store.StoreBytes(att.Name, mediaType, decoded)
				if err == nil && strings.TrimSpace(uri) != "" {
					att.URI = uri
				}
			}
		}
	}

	// Default: drop inline payloads; rely on URI (e.g. CDN URL) when present.
	att.Data = ""
	return att
}

func shouldRetainInlinePayload(mediaType string, size int) bool {
	if size <= 0 || size > historyInlineAttachmentRetentionLimit {
		return false
	}

	media := strings.ToLower(strings.TrimSpace(mediaType))
	if media == "" {
		return false
	}

	if strings.HasPrefix(media, "text/") {
		return true
	}

	return strings.Contains(media, "markdown") || strings.Contains(media, "json")
}

func eventFromRecord(record eventRecord) (ports.AgentEvent, error) {
	level := ports.AgentLevel(record.agentLevel)
	base := domain.NewBaseEvent(level, record.sessionID, record.taskID, record.parentTaskID, record.eventTS)

	if record.envelopeVer > 0 {
		payload := map[string]any{}
		if len(record.payload) > 0 {
			if err := json.Unmarshal(record.payload, &payload); err != nil {
				return nil, err
			}
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent:      base,
			Version:        record.envelopeVer,
			Event:          record.eventType,
			WorkflowID:     record.workflowID,
			RunID:          record.runID,
			NodeID:         record.nodeID,
			NodeKind:       record.nodeKind,
			IsSubtask:      record.isSubtask,
			SubtaskIndex:   record.subtaskIndex,
			TotalSubtasks:  record.totalSubtasks,
			SubtaskPreview: record.subtaskPrev,
			MaxParallel:    record.maxParallel,
			Payload:        payload,
		}, nil
	}

	switch record.eventType {
	case (&domain.WorkflowInputReceivedEvent{}).EventType():
		var payload struct {
			Task        string                      `json:"task"`
			Attachments map[string]ports.Attachment `json:"attachments"`
		}
		if len(record.payload) > 0 {
			if err := json.Unmarshal(record.payload, &payload); err != nil {
				return nil, err
			}
		}
		return domain.NewWorkflowInputReceivedEvent(level, record.sessionID, record.taskID, record.parentTaskID, payload.Task, payload.Attachments, record.eventTS), nil
	case (&domain.WorkflowDiagnosticContextSnapshotEvent{}).EventType():
		var payload struct {
			Iteration  int             `json:"iteration"`
			LLMTurnSeq int             `json:"llm_turn_seq"`
			RequestID  string          `json:"request_id"`
			Messages   []ports.Message `json:"messages"`
			Excluded   []ports.Message `json:"excluded"`
		}
		if len(record.payload) > 0 {
			if err := json.Unmarshal(record.payload, &payload); err != nil {
				return nil, err
			}
		}
		return domain.NewWorkflowDiagnosticContextSnapshotEvent(level, record.sessionID, record.taskID, record.parentTaskID, payload.Iteration, payload.LLMTurnSeq, payload.RequestID, payload.Messages, payload.Excluded, record.eventTS), nil
	default:
		payload := map[string]any{}
		if len(record.payload) > 0 {
			if err := json.Unmarshal(record.payload, &payload); err != nil {
				return nil, err
			}
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent: base,
			Version:   1,
			Event:     record.eventType,
			NodeID:    record.nodeID,
			NodeKind:  record.nodeKind,
			Payload:   payload,
		}, nil
	}
}
