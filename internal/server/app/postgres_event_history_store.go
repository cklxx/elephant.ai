package app

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/logging"

	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	defaultHistoryBatchSize               = 500
	historyInlineAttachmentRetentionLimit = 128 * 1024
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

// PostgresEventHistoryStore persists event history in Postgres.
type PostgresEventHistoryStore struct {
	pool      *pgxpool.Pool
	batchSize int
	logger    logging.Logger
}

// NewPostgresEventHistoryStore constructs a Postgres-backed history store.
func NewPostgresEventHistoryStore(pool *pgxpool.Pool) *PostgresEventHistoryStore {
	return &PostgresEventHistoryStore{
		pool:      pool,
		batchSize: defaultHistoryBatchSize,
		logger:    logging.NewComponentLogger("EventHistoryStore"),
	}
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

	record, err := recordFromEvent(BaseAgentEvent(event))
	if err != nil {
		return err
	}

	var payloadParam any
	if len(record.payload) > 0 {
		payloadParam = record.payload
	}

	_, err = s.pool.Exec(ctx, `
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
	return err
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
		record, err := recordFromEvent(BaseAgentEvent(event))
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
	_, err := s.pool.Exec(ctx, sb.String(), args...)
	return err
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
	_, err := s.pool.Exec(ctx, `DELETE FROM agent_session_events WHERE session_id = $1`, sessionID)
	return err
}

// HasSessionEvents checks if session history exists.
func (s *PostgresEventHistoryStore) HasSessionEvents(ctx context.Context, sessionID string) (bool, error) {
	if s == nil || s.pool == nil {
		return false, fmt.Errorf("history store not initialized")
	}
	var exists bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agent_session_events WHERE session_id = $1)`, sessionID).Scan(&exists)
	if err != nil {
		return false, err
	}
	return exists, nil
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

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []eventRecord
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

func recordFromEvent(event ports.AgentEvent) (eventRecord, error) {
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
		payload = stripBinaryPayloads(e.Payload)
	case *domain.WorkflowInputReceivedEvent:
		payload = map[string]any{
			"task":        e.Task,
			"attachments": stripBinaryPayloads(e.Attachments),
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

func stripBinaryPayloads(value any) any {
	switch v := value.(type) {
	case nil:
		return nil
	case ports.Attachment:
		return sanitizeAttachmentForHistory(v)
	case *ports.Attachment:
		if v == nil {
			return nil
		}
		cleaned := sanitizeAttachmentForHistory(*v)
		return &cleaned
	case map[string]ports.Attachment:
		cleaned := make(map[string]ports.Attachment, len(v))
		for key, att := range v {
			cleaned[key] = sanitizeAttachmentForHistory(att)
		}
		return cleaned
	case []ports.Attachment:
		cleaned := make([]ports.Attachment, len(v))
		for i, att := range v {
			cleaned[i] = sanitizeAttachmentForHistory(att)
		}
		return cleaned
	case map[string]any:
		cleaned := make(map[string]any, len(v))
		for key, val := range v {
			cleaned[key] = stripBinaryPayloads(val)
		}
		return cleaned
	case []any:
		cleaned := make([]any, len(v))
		for i, val := range v {
			cleaned[i] = stripBinaryPayloads(val)
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

func sanitizeAttachmentForHistory(att ports.Attachment) ports.Attachment {
	mediaType := strings.TrimSpace(att.MediaType)
	if mediaType == "" {
		mediaType = "application/octet-stream"
		att.MediaType = mediaType
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
