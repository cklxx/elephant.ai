package app

import (
	"context"
	"testing"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
)

func TestEventBroadcaster_RegisterUnregister(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	sessionID := "test-session"
	ch := make(chan ports.AgentEvent, 10)

	// Register client
	broadcaster.RegisterClient(sessionID, ch)

	// Check client count
	if count := broadcaster.GetClientCount(sessionID); count != 1 {
		t.Errorf("Expected 1 client, got %d", count)
	}

	// Unregister client
	broadcaster.UnregisterClient(sessionID, ch)

	// Check client count after unregistration
	if count := broadcaster.GetClientCount(sessionID); count != 0 {
		t.Errorf("Expected 0 clients after unregister, got %d", count)
	}
}

func TestEventBroadcaster_BroadcastEvent(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	sessionID := "test-session"
	ch1 := make(chan ports.AgentEvent, 10)
	ch2 := make(chan ports.AgentEvent, 10)

	// Register two clients
	broadcaster.RegisterClient(sessionID, ch1)
	broadcaster.RegisterClient(sessionID, ch2)

	// Create and broadcast an event
	event := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"task-broadcast",
		"",
		&ports.TaskAnalysis{ActionName: "Test Action", Goal: "Test Goal"},
		time.Now(),
	)
	broadcaster.OnEvent(event)

	// Give some time for event to be delivered
	time.Sleep(100 * time.Millisecond)

	// Check if both clients received the event
	select {
	case receivedEvent := <-ch1:
		if receivedEvent.EventType() != "task_analysis" {
			t.Errorf("Client 1 received wrong event type: %s", receivedEvent.EventType())
		}
	default:
		t.Error("Client 1 did not receive event")
	}

	select {
	case receivedEvent := <-ch2:
		if receivedEvent.EventType() != "task_analysis" {
			t.Errorf("Client 2 received wrong event type: %s", receivedEvent.EventType())
		}
	default:
		t.Error("Client 2 did not receive event")
	}

	// Cleanup
	broadcaster.UnregisterClient(sessionID, ch1)
	broadcaster.UnregisterClient(sessionID, ch2)
}

func TestEventBroadcaster_MultipleSessionsIsolation(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	session1 := "session-1"
	session2 := "session-2"

	ch1 := make(chan ports.AgentEvent, 10)
	ch2 := make(chan ports.AgentEvent, 10)

	// Register clients to different sessions
	broadcaster.RegisterClient(session1, ch1)
	broadcaster.RegisterClient(session2, ch2)

	// Broadcast event for session1 - should only go to session1
	event := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		session1,
		"task-session1",
		"",
		&ports.TaskAnalysis{ActionName: "Test", Goal: "Test"},
		time.Now(),
	)
	broadcaster.OnEvent(event)

	time.Sleep(100 * time.Millisecond)

	// Session 1 should receive the event
	if len(ch1) == 0 {
		t.Error("Session 1 client should have received event")
	}
	// Session 2 should NOT receive the event (isolation)
	if len(ch2) != 0 {
		t.Error("Session 2 client should NOT have received event (isolation)")
	}

	// Cleanup
	broadcaster.UnregisterClient(session1, ch1)
	broadcaster.UnregisterClient(session2, ch2)
}

func TestEventBroadcaster_BufferFull(t *testing.T) {
	broadcaster := NewEventBroadcaster()

	sessionID := "test-session"
	// Create a small buffer channel
	ch := make(chan ports.AgentEvent, 2)

	broadcaster.RegisterClient(sessionID, ch)

	// Fill the buffer
	for i := 0; i < 5; i++ {
		event := domain.NewTaskAnalysisEvent(
			types.LevelCore,
			sessionID,
			"task-buffer",
			"",
			&ports.TaskAnalysis{ActionName: "Test", Goal: "Test"},
			time.Now(),
		)
		broadcaster.OnEvent(event)
	}

	time.Sleep(100 * time.Millisecond)

	// Should have at most 2 events (buffer size)
	eventCount := len(ch)
	if eventCount > 2 {
		t.Errorf("Expected at most 2 events in buffer, got %d", eventCount)
	}

	// Cleanup
	broadcaster.UnregisterClient(sessionID, ch)
}

func TestEventBroadcaster_AttachmentArchiver(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	stub := &stubAttachmentArchiver{calls: make(chan stubAttachmentCall, 1)}
	broadcaster.SetAttachmentArchiver(stub)

	sessionID := "session-attachments"
	base := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"task-attachments",
		"",
		&ports.TaskAnalysis{ActionName: "Action", Goal: "Goal"},
		time.Now(),
	).BaseEvent
	event := &domain.ToolCallCompleteEvent{
		BaseEvent: base,
		Attachments: map[string]ports.Attachment{
			"image.png": {
				Name:      "image.png",
				Data:      "ZGF0YQ==",
				MediaType: "image/png",
				Source:    "seedream",
			},
		},
	}

	broadcaster.OnEvent(event)

	select {
	case call := <-stub.calls:
		if call.sessionID != sessionID {
			t.Fatalf("expected sessionID %s, got %s", sessionID, call.sessionID)
		}
		if len(call.attachments) != 1 {
			t.Fatalf("expected 1 attachment, got %d", len(call.attachments))
		}
	case <-time.After(1 * time.Second):
		t.Fatal("attachment archiver was not invoked")
	}
}

func TestEventBroadcaster_ArchivesUserTaskAttachments(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	stub := &stubAttachmentArchiver{calls: make(chan stubAttachmentCall, 1)}
	broadcaster.SetAttachmentArchiver(stub)

	sessionID := "session-user"
	event := domain.NewUserTaskEvent(
		types.LevelCore,
		sessionID,
		"task-user",
		"",
		"write doc",
		map[string]ports.Attachment{
			"notes.txt": {
				Name:      "notes.txt",
				MediaType: "text/plain",
				Data:      "ZGF0YQ==",
				Source:    "user_upload",
			},
		},
		time.Now(),
	)

	broadcaster.OnEvent(event)

	select {
	case call := <-stub.calls:
		if call.sessionID != sessionID {
			t.Fatalf("expected sessionID %s, got %s", sessionID, call.sessionID)
		}
		if _, ok := call.attachments["notes.txt"]; !ok {
			t.Fatal("expected notes.txt to be archived")
		}
	case <-time.After(1 * time.Second):
		t.Fatal("user_task attachments were not archived")
	}
}

func TestEventBroadcaster_ExportsAttachmentsWhenLastClientLeaves(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	exporter := &stubAttachmentExporter{calls: make(chan stubAttachmentCall, 1)}
	exporter.result = AttachmentExportResult{
		ExporterKind:      "stub",
		Endpoint:          "stub://export",
		Exported:          true,
		AttachmentUpdates: map[string]ports.Attachment{"diagram.png": {URI: "https://cdn.example/diagram.png"}},
	}
	broadcaster.SetAttachmentExporter(exporter)

	sessionID := "session-export"
	ch := make(chan ports.AgentEvent, 1)
	broadcaster.RegisterClient(sessionID, ch)

	base := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"task-export",
		"",
		&ports.TaskAnalysis{ActionName: "Action", Goal: "Goal"},
		time.Now(),
	).BaseEvent
	event := &domain.ToolCallCompleteEvent{
		BaseEvent: base,
		Attachments: map[string]ports.Attachment{
			"diagram.png": {
				Name:      "diagram.png",
				MediaType: "image/png",
				Data:      "ZGF0YQ==",
			},
		},
	}

	broadcaster.OnEvent(event)
	broadcaster.UnregisterClient(sessionID, ch)

	select {
	case call := <-exporter.calls:
		if call.sessionID != sessionID {
			t.Fatalf("expected session %s, got %s", sessionID, call.sessionID)
		}
		if _, ok := call.attachments["diagram.png"]; !ok {
			t.Fatalf("expected diagram.png to be exported: %+v", call.attachments)
		}
	case <-time.After(1 * time.Second):
		t.Fatal("attachment exporter was not invoked when session closed")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		history := broadcaster.GetEventHistory(sessionID)
		for _, evt := range history {
			if exportEvt, ok := evt.(*AttachmentExportEvent); ok {
				updates := exportEvt.AttachmentUpdates()
				if uri := updates["diagram.png"].URI; uri == "https://cdn.example/diagram.png" {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected attachment export event with CDN updates")
}

func TestEventBroadcaster_EmitsExportStatusEvent(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	exporter := &stubAttachmentExporter{
		calls: make(chan stubAttachmentCall, 1),
		result: AttachmentExportResult{
			ExporterKind: "stub",
			Endpoint:     "stub://export",
			Exported:     true,
			Attempts:     2,
		},
	}
	broadcaster.SetAttachmentExporter(exporter)

	sessionID := "session-status"
	ch := make(chan ports.AgentEvent, 1)
	broadcaster.RegisterClient(sessionID, ch)

	base := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"task-status",
		"",
		&ports.TaskAnalysis{ActionName: "Action", Goal: "Goal"},
		time.Now(),
	).BaseEvent
	event := &domain.ToolCallCompleteEvent{
		BaseEvent: base,
		Attachments: map[string]ports.Attachment{
			"diagram.png": {Name: "diagram.png"},
		},
	}

	broadcaster.OnEvent(event)
	broadcaster.UnregisterClient(sessionID, ch)

	select {
	case <-exporter.calls:
	case <-time.After(1 * time.Second):
		t.Fatal("attachment exporter was not invoked")
	}

	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		history := broadcaster.GetEventHistory(sessionID)
		for _, evt := range history {
			if exportEvt, ok := evt.(*AttachmentExportEvent); ok {
				if exportEvt.Status() != AttachmentExportStatusSucceeded {
					t.Fatalf("expected success status, got %s", exportEvt.Status())
				}
				if exportEvt.AttachmentCount() != 1 {
					t.Fatalf("expected attachment count 1, got %d", exportEvt.AttachmentCount())
				}
				if exportEvt.Attempts() != exporter.result.Attempts {
					t.Fatalf("expected attempts %d, got %d", exporter.result.Attempts, exportEvt.Attempts())
				}
				return
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected attachment export event to be recorded")
}

func TestEventBroadcaster_ReportAttachmentScanStoresEvent(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	broadcaster.ReportAttachmentScan(
		"session-scan",
		"task-123",
		"blocked.png",
		ports.Attachment{Name: "blocked.png"},
		AttachmentScanResult{Verdict: AttachmentScanVerdictInfected, Details: "virus"},
	)
	history := broadcaster.GetEventHistory("session-scan")
	if len(history) != 1 {
		t.Fatalf("expected one scan event in history, got %d", len(history))
	}
	event, ok := history[0].(*AttachmentScanEvent)
	if !ok {
		t.Fatalf("expected AttachmentScanEvent, got %T", history[0])
	}
	if event.Placeholder() != "blocked.png" {
		t.Fatalf("unexpected placeholder: %s", event.Placeholder())
	}
	if event.GetTaskID() != "task-123" {
		t.Fatalf("unexpected task id: %s", event.GetTaskID())
	}
	if event.Details() != "virus" {
		t.Fatalf("unexpected details: %s", event.Details())
	}
}

func TestEventBroadcaster_AppliesAttachmentUpdatesToHistory(t *testing.T) {
	broadcaster := NewEventBroadcaster()
	exporter := &stubAttachmentExporter{
		calls: make(chan stubAttachmentCall, 1),
		result: AttachmentExportResult{
			Exported: true,
			AttachmentUpdates: map[string]ports.Attachment{
				"notes.txt": {URI: "https://cdn.example/notes.txt"},
			},
		},
	}
	broadcaster.SetAttachmentExporter(exporter)
	sessionID := "session-history"
	ch := make(chan ports.AgentEvent, 1)
	broadcaster.RegisterClient(sessionID, ch)
	base := domain.NewTaskAnalysisEvent(
		types.LevelCore,
		sessionID,
		"task-history",
		"",
		&ports.TaskAnalysis{ActionName: "Action", Goal: "Goal"},
		time.Now(),
	).BaseEvent
	event := &domain.UserTaskEvent{
		BaseEvent: base,
		Task:      "document",
		Attachments: map[string]ports.Attachment{
			"notes.txt": {
				Name:      "notes.txt",
				MediaType: "text/plain",
				Data:      "ZGF0YQ==",
			},
		},
	}
	broadcaster.OnEvent(event)
	broadcaster.UnregisterClient(sessionID, ch)
	select {
	case <-exporter.calls:
	case <-time.After(time.Second):
		t.Fatal("attachment exporter was not invoked")
	}
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		history := broadcaster.GetEventHistory(sessionID)
		for _, evt := range history {
			if userEvt, ok := evt.(*domain.UserTaskEvent); ok {
				if att := userEvt.Attachments["notes.txt"]; att.URI == "https://cdn.example/notes.txt" {
					return
				}
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("expected attachment URI to be updated in history")
}

type stubAttachmentCall struct {
	sessionID   string
	taskID      string
	attachments map[string]ports.Attachment
}

type stubAttachmentArchiver struct {
	calls chan stubAttachmentCall
}

func (s *stubAttachmentArchiver) Persist(ctx context.Context, sessionID, taskID string, attachments map[string]ports.Attachment) {
	s.calls <- stubAttachmentCall{
		sessionID:   sessionID,
		taskID:      taskID,
		attachments: attachments,
	}
}

type stubAttachmentExporter struct {
	calls  chan stubAttachmentCall
	result AttachmentExportResult
}

func (s *stubAttachmentExporter) ExportSession(ctx context.Context, sessionID string, attachments map[string]ports.Attachment) AttachmentExportResult {
	s.calls <- stubAttachmentCall{
		sessionID:   sessionID,
		attachments: attachments,
	}
	result := s.result
	if result.AttachmentCount == 0 {
		result.AttachmentCount = len(attachments)
	}
	if result.Attempts == 0 {
		result.Attempts = 1
	}
	return result
}
