package http

import (
	"testing"
	"time"

	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func BenchmarkSSESerializeEventToolProgress(b *testing.B) {
	handler := NewSSEHandler(nil)
	sentAttachments := newStringLRU(sseSentAttachmentCacheSize)
	finalAnswerCache := newStringLRU(sseFinalAnswerCacheSize)
	event := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agentports.LevelCore, "bench-session", "run-1", "", time.Now()),
		Version:   1,
		Event:     types.EventToolProgress,
		NodeID:    "tool:bash",
		NodeKind:  "tool",
		Payload: map[string]any{
			"call_id":   "call-1",
			"tool_name": "bash",
			"chunk":     "benchmark tool progress chunk payload",
		},
	}

	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if _, err := handler.serializeEvent(event, sentAttachments, finalAnswerCache); err != nil {
			b.Fatalf("serializeEvent error: %v", err)
		}
	}
}
