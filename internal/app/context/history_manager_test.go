package context

import (
	"context"
	"fmt"
	"runtime"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	sessionstate "alex/internal/infra/session/state_store"
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

func TestHistoryManagerAppendTurnWithExisting(t *testing.T) {
	ctx := context.Background()
	store := sessionstate.NewInMemoryStore()
	manager := NewHistoryManager(store, agent.NoopLogger{}, agent.ClockFunc(time.Now))
	sessionID := "session-existing"

	firstTurn := []ports.Message{
		{Role: "user", Content: "first", Source: ports.MessageSourceUserInput},
		{Role: "assistant", Content: "ack", Source: ports.MessageSourceAssistantReply},
	}
	if err := manager.AppendTurn(ctx, sessionID, firstTurn); err != nil {
		t.Fatalf("append first turn failed: %v", err)
	}

	incoming := append(agent.CloneMessages(firstTurn),
		ports.Message{Role: "user", Content: "second", Source: ports.MessageSourceUserInput},
		ports.Message{Role: "assistant", Content: "done", Source: ports.MessageSourceAssistantReply},
	)
	if err := manager.AppendTurnWithExisting(ctx, sessionID, firstTurn, incoming); err != nil {
		t.Fatalf("AppendTurnWithExisting failed: %v", err)
	}

	history, err := manager.Replay(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}
	if len(history) != len(incoming) {
		t.Fatalf("expected %d messages, got %d", len(incoming), len(history))
	}
	for i := range incoming {
		if history[i].Role != incoming[i].Role || history[i].Content != incoming[i].Content {
			t.Fatalf("message %d mismatch: want %+v got %+v", i, incoming[i], history[i])
		}
	}
}

func TestReplayMaxSnapshotCap(t *testing.T) {
	ctx := context.Background()
	store := sessionstate.NewInMemoryStore()
	manager := NewHistoryManager(store, agent.NoopLogger{}, agent.ClockFunc(time.Now))
	sessionID := "session-cap"

	// Write 200 snapshots (well above maxReplaySnapshots=50).
	var allMessages []ports.Message
	for i := 1; i <= 200; i++ {
		msg := ports.Message{
			Role:    "user",
			Content: fmt.Sprintf("turn %d", i),
			Source:  ports.MessageSourceUserInput,
		}
		allMessages = append(allMessages, msg)

		snapshot := sessionstate.Snapshot{
			SessionID: sessionID,
			TurnID:    i,
			CreatedAt: time.Now(),
			Messages:  []ports.Message{msg},
		}
		if err := store.SaveSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("save snapshot %d: %v", i, err)
		}
	}

	history, err := manager.Replay(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Should be capped at maxReplaySnapshots * 1 message per snapshot = 50 messages.
	if len(history) > maxReplaySnapshots {
		t.Errorf("expected at most %d messages, got %d", maxReplaySnapshots, len(history))
	}

	// First snapshot (turn 1) is always preserved for base context,
	// plus the most recent 49 snapshots (turns 152..200).
	if len(history) > 0 {
		first := history[0]
		if first.Content != "turn 1" {
			t.Errorf("expected first message to be %q, got %q", "turn 1", first.Content)
		}
		second := history[1]
		expectedSecond := fmt.Sprintf("turn %d", 200-maxReplaySnapshots+2)
		if second.Content != expectedSecond {
			t.Errorf("expected second message to be %q, got %q", expectedSecond, second.Content)
		}
		last := history[len(history)-1]
		if last.Content != "turn 200" {
			t.Errorf("expected last message to be 'turn 200', got %q", last.Content)
		}
	}
}

func TestReplayStress10000Turns(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping stress test in short mode")
	}

	ctx := context.Background()
	store := sessionstate.NewInMemoryStore()
	manager := NewHistoryManager(store, agent.NoopLogger{}, agent.ClockFunc(time.Now))
	sessionID := "session-stress"

	const totalTurns = 10000

	// Write 10000 snapshots, each with a message containing some data.
	for i := 1; i <= totalTurns; i++ {
		snapshot := sessionstate.Snapshot{
			SessionID: sessionID,
			TurnID:    i,
			CreatedAt: time.Now(),
			Messages: []ports.Message{
				{
					Role:    "user",
					Content: fmt.Sprintf("turn %d with some reasonable content that simulates a real message payload", i),
					Source:  ports.MessageSourceUserInput,
				},
				{
					Role:    "assistant",
					Content: fmt.Sprintf("response to turn %d with tool output and reasoning", i),
					Source:  ports.MessageSourceAssistantReply,
				},
			},
		}
		if err := store.SaveSnapshot(ctx, snapshot); err != nil {
			t.Fatalf("save snapshot %d: %v", i, err)
		}
	}

	// Measure memory before replay.
	runtime.GC()
	var memBefore runtime.MemStats
	runtime.ReadMemStats(&memBefore)

	history, err := manager.Replay(ctx, sessionID, 0)
	if err != nil {
		t.Fatalf("replay failed: %v", err)
	}

	// Measure memory after replay.
	var memAfter runtime.MemStats
	runtime.ReadMemStats(&memAfter)

	heapGrowth := memAfter.HeapAlloc - memBefore.HeapAlloc
	t.Logf("Results:")
	t.Logf("  Total turns:    %d", totalTurns)
	t.Logf("  Messages loaded: %d", len(history))
	t.Logf("  Heap growth:    %.2f MB", float64(heapGrowth)/1024/1024)

	// With maxReplaySnapshots=50, should load at most 50 * 2 = 100 messages.
	maxExpected := maxReplaySnapshots * 2
	if len(history) > maxExpected {
		t.Errorf("expected at most %d messages, got %d — cap not working", maxExpected, len(history))
	}

	// Memory growth should be minimal (< 50 MB even with overhead).
	if heapGrowth > 50*1024*1024 {
		t.Errorf("heap growth too high: %.2f MB — likely unbounded loading", float64(heapGrowth)/1024/1024)
	}
}
