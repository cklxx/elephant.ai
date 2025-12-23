package app

import (
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
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
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "", time.Now()),
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
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "", time.Now()),
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
	inner ports.AgentEvent
	meta  ports.SubtaskMetadata
	level ports.AgentLevel
}

func (w *stubSubtaskWrapper) EventType() string {
	if w == nil || w.inner == nil {
		return "subtask"
	}
	return w.inner.EventType()
}

func (w *stubSubtaskWrapper) Timestamp() time.Time {
	if w == nil || w.inner == nil {
		return time.Time{}
	}
	return w.inner.Timestamp()
}

func (w *stubSubtaskWrapper) GetAgentLevel() ports.AgentLevel {
	if w == nil {
		return ports.LevelSubagent
	}
	if w.level != "" {
		return w.level
	}
	return ports.LevelSubagent
}

func (w *stubSubtaskWrapper) GetSessionID() string {
	if w == nil || w.inner == nil {
		return ""
	}
	return w.inner.GetSessionID()
}

func (w *stubSubtaskWrapper) GetTaskID() string {
	if w == nil || w.inner == nil {
		return ""
	}
	return w.inner.GetTaskID()
}

func (w *stubSubtaskWrapper) GetParentTaskID() string {
	if w == nil || w.inner == nil {
		return ""
	}
	return w.inner.GetParentTaskID()
}

func (w *stubSubtaskWrapper) SubtaskDetails() ports.SubtaskMetadata {
	if w == nil {
		return ports.SubtaskMetadata{}
	}
	return w.meta
}

func (w *stubSubtaskWrapper) WrappedEvent() ports.AgentEvent {
	if w == nil {
		return nil
	}
	return w.inner
}

func TestRecordFromEventPreservesSubtaskWrapperMetadata(t *testing.T) {
	now := time.Now()
	envelope := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(ports.LevelCore, "sess", "task", "parent", now),
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
		level: ports.LevelSubagent,
		meta: ports.SubtaskMetadata{
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

	if record.agentLevel != string(ports.LevelSubagent) {
		t.Fatalf("expected agent level %q, got %q", ports.LevelSubagent, record.agentLevel)
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
	if env.GetAgentLevel() != ports.LevelSubagent {
		t.Fatalf("expected rehydrated agent level %q, got %q", ports.LevelSubagent, env.GetAgentLevel())
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
