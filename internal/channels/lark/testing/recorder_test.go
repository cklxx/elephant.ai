package larktesting

import (
	"context"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
	larkgw "alex/internal/channels/lark"
)

func TestConversationTraceAppendAndEntries(t *testing.T) {
	trace := NewConversationTrace()

	trace.Append(TraceEntry{Direction: "inbound", Type: "message", Content: "hello"})
	trace.Append(TraceEntry{Direction: "outbound", Type: "reply", Content: "world"})

	entries := trace.Entries()
	if len(entries) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(entries))
	}
	if entries[0].Content != "hello" {
		t.Fatalf("expected first entry 'hello', got %q", entries[0].Content)
	}
	if entries[1].Content != "world" {
		t.Fatalf("expected second entry 'world', got %q", entries[1].Content)
	}

	// Entries should be a snapshot — appending after shouldn't affect returned slice.
	trace.Append(TraceEntry{Direction: "event", Type: "event"})
	if len(entries) != 2 {
		t.Fatal("entries snapshot should not change after additional appends")
	}
}

func TestConversationTraceReset(t *testing.T) {
	trace := NewConversationTrace()
	trace.Append(TraceEntry{Direction: "inbound"})
	trace.Reset()
	if len(trace.Entries()) != 0 {
		t.Fatal("expected empty after reset")
	}
}

func TestTracingMessengerRecordsAllCalls(t *testing.T) {
	rec := larkgw.NewRecordingMessenger()
	trace := NewConversationTrace()
	tracing := NewTracingMessenger(rec, trace)

	ctx := context.Background()

	_, _ = tracing.SendMessage(ctx, "chat_1", "text", `{"text":"hi"}`)
	_, _ = tracing.ReplyMessage(ctx, "om_1", "text", `{"text":"reply"}`)
	_ = tracing.UpdateMessage(ctx, "om_2", "text", `{"text":"updated"}`)
	_ = tracing.AddReaction(ctx, "om_3", "SMILE")
	_, _ = tracing.UploadImage(ctx, []byte("img"))
	_, _ = tracing.UploadFile(ctx, []byte("pdf"), "doc.pdf", "pdf")
	_, _ = tracing.ListMessages(ctx, "chat_1", 10)

	entries := trace.Entries()
	if len(entries) != 7 {
		t.Fatalf("expected 7 trace entries, got %d", len(entries))
	}

	expectedMethods := []string{
		"SendMessage", "ReplyMessage", "UpdateMessage",
		"AddReaction", "UploadImage", "UploadFile", "ListMessages",
	}
	for i, expected := range expectedMethods {
		if entries[i].Method != expected {
			t.Errorf("entry %d: expected method %q, got %q", i, expected, entries[i].Method)
		}
		if entries[i].Direction != "outbound" {
			t.Errorf("entry %d: expected direction 'outbound', got %q", i, entries[i].Direction)
		}
	}

	// Verify inner messenger also received the calls.
	innerCalls := rec.Calls()
	if len(innerCalls) != 7 {
		t.Fatalf("expected inner messenger to have 7 calls, got %d", len(innerCalls))
	}
}

func TestRecordingEventListenerRecordsEvents(t *testing.T) {
	trace := NewConversationTrace()
	inner := agent.NoopEventListener{}
	listener := NewRecordingEventListener(inner, trace)

	// Use a simple event type for testing.
	listener.OnEvent(&testAgentEvent{})

	entries := trace.Entries()
	if len(entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(entries))
	}
	if entries[0].Direction != "event" {
		t.Fatalf("expected direction 'event', got %q", entries[0].Direction)
	}
	if entries[0].EventType == "" {
		t.Fatal("expected non-empty event type")
	}
}

// testAgentEvent is a minimal AgentEvent implementation for testing.
type testAgentEvent struct{}

func (e *testAgentEvent) EventType() string            { return "test_event" }
func (e *testAgentEvent) Timestamp() time.Time         { return time.Now() }
func (e *testAgentEvent) GetAgentLevel() agent.AgentLevel { return agent.LevelCore }
func (e *testAgentEvent) GetSessionID() string         { return "session-1" }
func (e *testAgentEvent) GetRunID() string             { return "run-1" }
func (e *testAgentEvent) GetParentRunID() string       { return "" }
func (e *testAgentEvent) GetCorrelationID() string     { return "" }
func (e *testAgentEvent) GetCausationID() string       { return "" }
func (e *testAgentEvent) GetEventID() string           { return "evt-1" }
func (e *testAgentEvent) GetSeq() uint64               { return 0 }
