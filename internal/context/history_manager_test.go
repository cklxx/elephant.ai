package context

import (
	"context"
	"testing"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	sessionstate "alex/internal/session/state_store"
)

func TestHistoryManagerAppendAndReplayOrder(t *testing.T) {
	ctx := context.Background()
	store := sessionstate.NewInMemoryStore()
	manager := NewHistoryManager(store, agent.NoopLogger{}, agent.ClockFunc(time.Now))
	sessionID := "session-order"

	firstTurn := []ports.Message{
		{Role: "user", Content: "场景：小猪在草地上休息", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "收到，正在生成素材。", Source: ports.MessageSourceAssistantReply},
	}
	if err := manager.AppendTurn(ctx, sessionID, firstTurn); err != nil {
		t.Fatalf("append first turn failed: %v", err)
	}

	secondTurn := append(agent.CloneMessages(firstTurn),
		ports.Message{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{ID: "vision:1", Name: "vision_analyze"},
				{ID: "image:2", Name: "text_to_image"},
			},
			Source: ports.MessageSourceAssistantReply,
		},
		ports.Message{Role: "tool", ToolCallID: "vision:1", Content: "vision ok", Source: ports.MessageSourceToolResult},
		ports.Message{Role: "tool", ToolCallID: "image:2", Content: "image ok", Source: ports.MessageSourceToolResult},
		ports.Message{Role: "assistant", Content: "生成完成", Source: ports.MessageSourceAssistantReply},
	)
	if err := manager.AppendTurn(ctx, sessionID, secondTurn); err != nil {
		t.Fatalf("append second turn failed: %v", err)
	}

	history, err := manager.Replay(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(history) != len(secondTurn) {
		t.Fatalf("expected %d messages, got %d", len(secondTurn), len(history))
	}
	for i := range secondTurn {
		if history[i].Role != secondTurn[i].Role || history[i].Content != secondTurn[i].Content || history[i].ToolCallID != secondTurn[i].ToolCallID {
			t.Fatalf("message %d mismatch: want %+v, got %+v", i, secondTurn[i], history[i])
		}
	}
}

func TestHistoryManagerResetsOnPrefixMismatch(t *testing.T) {
	ctx := context.Background()
	store := sessionstate.NewInMemoryStore()
	manager := NewHistoryManager(store, agent.NoopLogger{}, agent.ClockFunc(time.Now))
	sessionID := "session-reset"

	first := []ports.Message{{Role: "user", Content: "first", Source: ports.MessageSourceUserInput}}
	if err := manager.AppendTurn(ctx, sessionID, first); err != nil {
		t.Fatalf("append first turn failed: %v", err)
	}

	// Introduce divergence in the historical prefix.
	divergent := []ports.Message{{Role: "user", Content: "changed", Source: ports.MessageSourceUserInput}}
	if err := manager.AppendTurn(ctx, sessionID, divergent); err != nil {
		t.Fatalf("append divergent turn failed: %v", err)
	}

	history, err := manager.Replay(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(history) != len(divergent) {
		t.Fatalf("expected reset history to contain divergent turn only")
	}
	if history[0].Content != "changed" {
		t.Fatalf("unexpected history content after reset: %+v", history[0])
	}
}

func TestHistoryManagerClearSession(t *testing.T) {
	ctx := context.Background()
	store := sessionstate.NewInMemoryStore()
	manager := NewHistoryManager(store, agent.NoopLogger{}, agent.ClockFunc(time.Now))
	sessionID := "session-clear"

	first := []ports.Message{{Role: "user", Content: "first", Source: ports.MessageSourceUserInput}}
	if err := manager.AppendTurn(ctx, sessionID, first); err != nil {
		t.Fatalf("append turn failed: %v", err)
	}

	if err := manager.ClearSession(ctx, sessionID); err != nil {
		t.Fatalf("clear session failed: %v", err)
	}

	history, err := manager.Replay(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(history) != 0 {
		t.Fatalf("expected empty history after clear, got %d messages", len(history))
	}
}
