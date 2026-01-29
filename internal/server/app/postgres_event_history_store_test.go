package app

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/testutil"
)

func TestRecordFromEventStripsAttachmentData(t *testing.T) {
	payload := map[string]any{
		"result": map[string]any{
			"attachments": map[string]ports.Attachment{
				"video.mp4": {
					Name:      "video.mp4",
					MediaType: "video/mp4",
					Data:      base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03}),
				},
			},
		},
	}

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", time.Now()),
		Version:   1,
		Payload:   payload,
	}

	record, err := recordFromEvent(envelope)
	if err != nil {
		t.Fatalf("recordFromEvent returned error: %v", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(record.payload, &stored); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	result, ok := stored["result"].(map[string]any)
	if !ok {
		t.Fatalf("expected result map, got %T", stored["result"])
	}

	attachments, ok := result["attachments"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachments map, got %T", result["attachments"])
	}

	att, ok := attachments["video.mp4"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment entry, got %T", attachments["video.mp4"])
	}

	if data, ok := att["data"].(string); ok && data != "" {
		t.Fatalf("expected attachment data to be stripped, got %q", data)
	}
}

type stubHistoryAttachmentCall struct {
	name      string
	mediaType string
	data      []byte
}

type stubHistoryAttachmentStore struct {
	calls []stubHistoryAttachmentCall
	uri   string
}

func (s *stubHistoryAttachmentStore) StoreBytes(name, mediaType string, data []byte) (string, error) {
	s.calls = append(s.calls, stubHistoryAttachmentCall{
		name:      name,
		mediaType: mediaType,
		data:      data,
	})
	if s.uri != "" {
		return s.uri, nil
	}
	return "/api/attachments/test.bin", nil
}

func TestRecordFromEventStoresInlineBinaryAttachment(t *testing.T) {
	payload := map[string]any{
		"attachments": map[string]ports.Attachment{
			"clip.mp4": {
				Name:      "clip.mp4",
				MediaType: "video/mp4",
				Data:      base64.StdEncoding.EncodeToString([]byte{0x01, 0x02, 0x03}),
			},
		},
	}

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", time.Now()),
		Version:   1,
		Payload:   payload,
	}

	store := &stubHistoryAttachmentStore{uri: "/api/attachments/clip.mp4"}
	record, err := recordFromEventWithStore(envelope, store)
	if err != nil {
		t.Fatalf("recordFromEventWithStore returned error: %v", err)
	}

	if len(store.calls) != 1 {
		t.Fatalf("expected 1 store call, got %d", len(store.calls))
	}
	if store.calls[0].name != "clip.mp4" {
		t.Fatalf("expected stored name clip.mp4, got %q", store.calls[0].name)
	}
	if store.calls[0].mediaType != "video/mp4" {
		t.Fatalf("expected stored media type video/mp4, got %q", store.calls[0].mediaType)
	}
	if !bytes.Equal(store.calls[0].data, []byte{0x01, 0x02, 0x03}) {
		t.Fatalf("expected stored data to match inline payload")
	}

	var stored map[string]any
	if err := json.Unmarshal(record.payload, &stored); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	attachments, ok := stored["attachments"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachments map, got %T", stored["attachments"])
	}

	att, ok := attachments["clip.mp4"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment entry, got %T", attachments["clip.mp4"])
	}

	if uri, ok := att["uri"].(string); !ok || uri != store.uri {
		t.Fatalf("expected uri %q, got %#v", store.uri, att["uri"])
	}
	if data, ok := att["data"].(string); ok && data != "" {
		t.Fatalf("expected attachment data to be stripped, got %q", data)
	}
}

func TestRecordFromEventRetainsSmallTextAttachmentData(t *testing.T) {
	content := []byte("# Title\nBody")
	b64 := base64.StdEncoding.EncodeToString(content)

	payload := map[string]any{
		"attachments": map[string]ports.Attachment{
			"note.md": {
				Name:      "note.md",
				MediaType: "text/markdown",
				Data:      b64,
				URI:       "data:text/markdown;base64," + b64,
			},
		},
	}

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", time.Now()),
		Version:   1,
		Payload:   payload,
	}

	record, err := recordFromEvent(envelope)
	if err != nil {
		t.Fatalf("recordFromEvent returned error: %v", err)
	}

	var stored map[string]any
	if err := json.Unmarshal(record.payload, &stored); err != nil {
		t.Fatalf("failed to decode payload: %v", err)
	}

	attachments, ok := stored["attachments"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachments map, got %T", stored["attachments"])
	}

	att, ok := attachments["note.md"].(map[string]any)
	if !ok {
		t.Fatalf("expected attachment entry, got %T", attachments["note.md"])
	}

	gotData, _ := att["data"].(string)
	if gotData != b64 {
		t.Fatalf("expected attachment data to be retained, got %q", gotData)
	}

	if uri, ok := att["uri"].(string); ok && uri != "" {
		t.Fatalf("expected data URI to be stripped when data retained, got %q", uri)
	}
}

type stubSubtaskWrapper struct {
	inner agent.AgentEvent
	meta  agent.SubtaskMetadata
	level agent.AgentLevel
}

func (w *stubSubtaskWrapper) EventType() string {
	return w.inner.EventType()
}

func (w *stubSubtaskWrapper) Timestamp() time.Time {
	return w.inner.Timestamp()
}

func (w *stubSubtaskWrapper) GetAgentLevel() agent.AgentLevel {
	if w.level != "" {
		return w.level
	}
	return agent.LevelSubagent
}

func (w *stubSubtaskWrapper) GetSessionID() string {
	return w.inner.GetSessionID()
}

func (w *stubSubtaskWrapper) GetRunID() string {
	return w.inner.GetRunID()
}

func (w *stubSubtaskWrapper) GetParentRunID() string {
	return w.inner.GetParentRunID()
}

func (w *stubSubtaskWrapper) GetCorrelationID() string {
	return w.inner.GetCorrelationID()
}

func (w *stubSubtaskWrapper) GetCausationID() string {
	return w.inner.GetCausationID()
}

func (w *stubSubtaskWrapper) GetEventID() string {
	return w.inner.GetEventID()
}

func (w *stubSubtaskWrapper) GetSeq() uint64 {
	return w.inner.GetSeq()
}

func (w *stubSubtaskWrapper) SubtaskDetails() agent.SubtaskMetadata {
	return w.meta
}

func (w *stubSubtaskWrapper) WrappedEvent() agent.AgentEvent {
	return w.inner
}

func assertSubtaskEnvelope(t *testing.T, event agent.AgentEvent, meta agent.SubtaskMetadata) {
	t.Helper()

	envelope, ok := event.(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", event)
	}
	if envelope.GetAgentLevel() != agent.LevelSubagent {
		t.Fatalf("expected agent level %q, got %q", agent.LevelSubagent, envelope.GetAgentLevel())
	}
	if !envelope.IsSubtask {
		t.Fatalf("expected IsSubtask=true, got false")
	}
	if envelope.SubtaskIndex != meta.Index {
		t.Fatalf("expected subtask index %d, got %d", meta.Index, envelope.SubtaskIndex)
	}
	if envelope.TotalSubtasks != meta.Total {
		t.Fatalf("expected total subtasks %d, got %d", meta.Total, envelope.TotalSubtasks)
	}
	if envelope.SubtaskPreview != meta.Preview {
		t.Fatalf("expected preview %q, got %q", meta.Preview, envelope.SubtaskPreview)
	}
	if envelope.MaxParallel != meta.MaxParallel {
		t.Fatalf("expected max parallel %d, got %d", meta.MaxParallel, envelope.MaxParallel)
	}
}

func TestRecordFromEventPreservesSubtaskWrapperMetadata(t *testing.T) {
	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "parent", now),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:1",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}

	wrapper := &stubSubtaskWrapper{
		inner: envelope,
		level: agent.LevelSubagent,
		meta: agent.SubtaskMetadata{
			Index:       2,
			Total:       5,
			Preview:     "Inspect output rendering",
			MaxParallel: 3,
		},
	}

	record, err := recordFromEvent(wrapper)
	if err != nil {
		t.Fatalf("recordFromEvent returned error: %v", err)
	}

	if record.agentLevel != string(agent.LevelSubagent) {
		t.Fatalf("expected agent level %q, got %q", agent.LevelSubagent, record.agentLevel)
	}
	if !record.isSubtask {
		t.Fatalf("expected isSubtask=true, got false")
	}
	if record.subtaskIndex != wrapper.meta.Index {
		t.Fatalf("expected subtask index %d, got %d", wrapper.meta.Index, record.subtaskIndex)
	}
	if record.totalSubtasks != wrapper.meta.Total {
		t.Fatalf("expected total subtasks %d, got %d", wrapper.meta.Total, record.totalSubtasks)
	}
	if record.subtaskPrev != wrapper.meta.Preview {
		t.Fatalf("expected preview %q, got %q", wrapper.meta.Preview, record.subtaskPrev)
	}
	if record.maxParallel != wrapper.meta.MaxParallel {
		t.Fatalf("expected max parallel %d, got %d", wrapper.meta.MaxParallel, record.maxParallel)
	}

	rehydrated, err := eventFromRecord(record)
	if err != nil {
		t.Fatalf("eventFromRecord returned error: %v", err)
	}
	env, ok := rehydrated.(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected envelope, got %T", rehydrated)
	}
	if env.GetAgentLevel() != agent.LevelSubagent {
		t.Fatalf("expected rehydrated agent level %q, got %q", agent.LevelSubagent, env.GetAgentLevel())
	}
	if !env.IsSubtask {
		t.Fatalf("expected rehydrated IsSubtask=true, got false")
	}
	if env.SubtaskIndex != wrapper.meta.Index {
		t.Fatalf("expected rehydrated subtask index %d, got %d", wrapper.meta.Index, env.SubtaskIndex)
	}
	if env.TotalSubtasks != wrapper.meta.Total {
		t.Fatalf("expected rehydrated total subtasks %d, got %d", wrapper.meta.Total, env.TotalSubtasks)
	}
	if env.SubtaskPreview != wrapper.meta.Preview {
		t.Fatalf("expected rehydrated preview %q, got %q", wrapper.meta.Preview, env.SubtaskPreview)
	}
	if env.MaxParallel != wrapper.meta.MaxParallel {
		t.Fatalf("expected rehydrated max parallel %d, got %d", wrapper.meta.MaxParallel, env.MaxParallel)
	}
}

func TestPostgresEventHistoryStore_PreservesSubtaskWrapperMetadataOnAppend(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresEventHistoryStore(pool)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess-subtask", "task-1", "parent-1", now),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:1",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}
	meta := agent.SubtaskMetadata{
		Index:       1,
		Total:       2,
		Preview:     "Run unit tests",
		MaxParallel: 1,
	}
	wrapper := &stubSubtaskWrapper{
		inner: envelope,
		level: agent.LevelSubagent,
		meta:  meta,
	}

	if err := store.Append(ctx, wrapper); err != nil {
		t.Fatalf("append event: %v", err)
	}

	var events []agent.AgentEvent
	if err := store.Stream(ctx, EventHistoryFilter{SessionID: "sess-subtask"}, func(evt agent.AgentEvent) error {
		events = append(events, evt)
		return nil
	}); err != nil {
		t.Fatalf("stream history: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	assertSubtaskEnvelope(t, events[0], meta)
}

func TestPostgresEventHistoryStore_EnsureSchemaUpgradesExistingTable(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	_, err := pool.Exec(ctx, `
CREATE TABLE agent_session_events (
    id BIGSERIAL PRIMARY KEY,
    session_id TEXT NOT NULL DEFAULT '',
    event_type TEXT NOT NULL,
    event_ts TIMESTAMPTZ NOT NULL,
    payload JSONB
);`)
	if err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}

	store := NewPostgresEventHistoryStore(pool)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess-upgrade", "task-1", "", time.Now()),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:1",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}

	if err := store.Append(ctx, envelope); err != nil {
		t.Fatalf("append after schema upgrade: %v", err)
	}

	var events []agent.AgentEvent
	if err := store.Stream(ctx, EventHistoryFilter{SessionID: "sess-upgrade"}, func(evt agent.AgentEvent) error {
		events = append(events, evt)
		return nil
	}); err != nil {
		t.Fatalf("stream history: %v", err)
	}
	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
}

func TestPostgresEventHistoryStore_PreservesSubtaskWrapperMetadataOnAppendBatch(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresEventHistoryStore(pool)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess-subtask-batch", "task-2", "parent-2", now),
		Version:   1,
		Event:     "workflow.tool.completed",
		NodeKind:  "tool",
		NodeID:    "bash:2",
		Payload: map[string]any{
			"tool_name": "bash",
			"result":    "ok",
		},
	}
	meta := agent.SubtaskMetadata{
		Index:       0,
		Total:       3,
		Preview:     "Check tool output",
		MaxParallel: 2,
	}
	wrapper := &stubSubtaskWrapper{
		inner: envelope,
		level: agent.LevelSubagent,
		meta:  meta,
	}

	if err := store.AppendBatch(ctx, []agent.AgentEvent{wrapper}); err != nil {
		t.Fatalf("append batch: %v", err)
	}

	var events []agent.AgentEvent
	if err := store.Stream(ctx, EventHistoryFilter{SessionID: "sess-subtask-batch"}, func(evt agent.AgentEvent) error {
		events = append(events, evt)
		return nil
	}); err != nil {
		t.Fatalf("stream history: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 event, got %d", len(events))
	}
	assertSubtaskEnvelope(t, events[0], meta)
}

func TestPostgresEventHistoryStorePrunesOldEvents(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresEventHistoryStore(pool, WithHistoryRetention(24*time.Hour))
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	oldEvent := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", time.Now().Add(-48*time.Hour)),
		Version:   1,
		Event:     "workflow.node.started",
		NodeKind:  "plan",
		NodeID:    "node-old",
	}
	newEvent := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, "sess", "task", "", time.Now()),
		Version:   1,
		Event:     "workflow.node.completed",
		NodeKind:  "plan",
		NodeID:    "node-new",
	}

	if err := store.Append(ctx, oldEvent); err != nil {
		t.Fatalf("append old event: %v", err)
	}
	if err := store.Append(ctx, newEvent); err != nil {
		t.Fatalf("append new event: %v", err)
	}

	if err := store.Prune(ctx); err != nil {
		t.Fatalf("prune history: %v", err)
	}

	var events []agent.AgentEvent
	if err := store.Stream(ctx, EventHistoryFilter{SessionID: "sess"}, func(event agent.AgentEvent) error {
		events = append(events, event)
		return nil
	}); err != nil {
		t.Fatalf("stream history: %v", err)
	}

	if len(events) != 1 {
		t.Fatalf("expected 1 retained event, got %d", len(events))
	}
	envelope, ok := events[0].(*domain.WorkflowEventEnvelope)
	if !ok {
		t.Fatalf("expected workflow envelope, got %T", events[0])
	}
	if envelope.NodeID != "node-new" {
		t.Fatalf("expected new event to remain, got %q", envelope.NodeID)
	}
}

func TestPostgresEventHistoryStore_CrossInstanceReplay(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	storeA := NewPostgresEventHistoryStore(pool)
	storeB := NewPostgresEventHistoryStore(pool)

	if err := storeA.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	event := domain.NewWorkflowInputReceivedEvent(
		agent.LevelCore,
		"session-1",
		"task-1",
		"",
		"hello",
		nil,
		time.Now(),
	)

	if err := storeA.Append(ctx, event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	var got []agent.AgentEvent
	if err := storeB.Stream(ctx, EventHistoryFilter{SessionID: "session-1"}, func(evt agent.AgentEvent) error {
		got = append(got, evt)
		return nil
	}); err != nil {
		t.Fatalf("stream events: %v", err)
	}

	if len(got) != 1 {
		t.Fatalf("expected 1 event, got %d", len(got))
	}
	if got[0].EventType() != event.EventType() {
		t.Fatalf("expected event type %q, got %q", event.EventType(), got[0].EventType())
	}
}

func TestPostgresEventHistoryStore_HasAndDeleteSession(t *testing.T) {
	pool, _, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	ctx := context.Background()
	store := NewPostgresEventHistoryStore(pool)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	sessionID := "session-has-delete"
	has, err := store.HasSessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("check has events: %v", err)
	}
	if has {
		t.Fatal("expected no events before append")
	}

	event := domain.NewWorkflowInputReceivedEvent(
		agent.LevelCore,
		sessionID,
		"task-1",
		"",
		"hello",
		nil,
		time.Now(),
	)
	if err := store.Append(ctx, event); err != nil {
		t.Fatalf("append event: %v", err)
	}

	has, err = store.HasSessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("check has events after append: %v", err)
	}
	if !has {
		t.Fatal("expected events after append")
	}

	if err := store.DeleteSession(ctx, sessionID); err != nil {
		t.Fatalf("delete session events: %v", err)
	}

	has, err = store.HasSessionEvents(ctx, sessionID)
	if err != nil {
		t.Fatalf("check has events after delete: %v", err)
	}
	if has {
		t.Fatal("expected no events after delete")
	}
}

func TestPostgresEventHistoryStore_WithTimeoutUsesDefault(t *testing.T) {
	store := &PostgresEventHistoryStore{}
	start := time.Now()
	ctx, cancel := store.withTimeout(context.Background())
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}
	if deadline.Before(start) {
		t.Fatalf("expected deadline after start, got %v", deadline)
	}
	allowedSkew := 20 * time.Millisecond
	if deadline.Sub(start) > defaultHistoryQueryTimeout+allowedSkew {
		t.Fatalf("expected deadline within %s (skew %s), got %s", defaultHistoryQueryTimeout, allowedSkew, deadline.Sub(start))
	}
}

func TestPostgresEventHistoryStore_WithTimeoutRespectsParentDeadline(t *testing.T) {
	store := &PostgresEventHistoryStore{}
	parent, parentCancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer parentCancel()

	ctx, cancel := store.withTimeout(parent)
	defer cancel()

	deadline, ok := ctx.Deadline()
	if !ok {
		t.Fatal("expected deadline to be set")
	}
	parentDeadline, ok := parent.Deadline()
	if !ok {
		t.Fatal("expected parent deadline to be set")
	}
	if deadline.After(parentDeadline) {
		t.Fatalf("expected deadline <= parent deadline (%v), got %v", parentDeadline, deadline)
	}
}
