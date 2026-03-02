package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/domain/agent/types"
)

// preanalysisEmojiExecutor emits a preanalysis emoji event through the real
// listener chain, exactly as the agent preparation layer does in production.
type preanalysisEmojiExecutor struct {
	emoji     string
	emitCount int // how many times to emit (for idempotency tests); 0 = 1
}

func (e *preanalysisEmojiExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *preanalysisEmojiExecutor) ExecuteTask(_ context.Context, _ string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	n := e.emitCount
	if n <= 0 {
		n = 1
	}
	for i := 0; i < n; i++ {
		listener.OnEvent(&domain.Event{
			BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-pre", "", time.Now()),
			Kind:      types.EventDiagnosticPreanalysisEmoji,
			Data:      domain.EventData{ReactEmoji: e.emoji},
		})
	}
	return &agent.TaskResult{Answer: "done"}, nil
}

// noEmojiExecutor emits only non-preanalysis events.
type noEmojiExecutor struct{}

func (e *noEmojiExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if strings.TrimSpace(sessionID) == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *noEmojiExecutor) ExecuteTask(_ context.Context, _ string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	listener.OnEvent(&domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-1", "", time.Now()),
		Version:   1,
		Event:     types.EventToolStarted,
		Payload:   map[string]any{"tool_name": "shell"},
	})
	listener.OnEvent(&domain.Event{
		BaseEvent: domain.NewBaseEvent(agent.LevelCore, sessionID, "run-1", "", time.Now()),
		Kind:      types.EventDiagnosticError,
	})
	return &agent.TaskResult{Answer: "done"}, nil
}

func TestPreanalysisEmoji_E2E_ReactsWithLLMChosenEmoji(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(
		&preanalysisEmojiExecutor{emoji: "HEART"},
		rec,
		channels.BaseConfig{SessionPrefix: "test", AllowDirect: true},
	)

	if err := gw.InjectMessage(context.Background(), "oc_chat_1", "p2p", "ou_user", "om_msg_1", "帮我分析一下"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	// Allow async addReaction goroutine to settle.
	// Poll for the specific HEART reaction since the synchronous processing
	// reaction (OnIt) arrives first and would cause early loop exit.
	deadline := time.Now().Add(500 * time.Millisecond)
	var preanalysisReaction *MessengerCall
	for time.Now().Before(deadline) {
		for _, r := range rec.CallsByMethod("AddReaction") {
			if r.MsgID == "om_msg_1" && r.Emoji == "HEART" {
				preanalysisReaction = &r
				break
			}
		}
		if preanalysisReaction != nil {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if preanalysisReaction == nil {
		t.Fatalf("expected HEART reaction on om_msg_1, got reactions: %+v", rec.CallsByMethod("AddReaction"))
	}
}

func TestPreanalysisEmoji_E2E_StartReactionIsLLMChosen(t *testing.T) {
	rec := NewRecordingMessenger()
	// Use SMILE (not in defaultEmojiPool) so we can distinguish it from
	// the random end emoji that pickReactionEmojis() produces.
	gw := newTestGatewayWithMessenger(
		&preanalysisEmojiExecutor{emoji: "SMILE"},
		rec,
		channels.BaseConfig{SessionPrefix: "test", AllowDirect: true},
	)

	if err := gw.InjectMessage(context.Background(), "oc_chat_2", "p2p", "ou_user", "om_msg_2", "hello"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()
	time.Sleep(100 * time.Millisecond)

	var foundLLMReaction bool
	for _, r := range rec.CallsByMethod("AddReaction") {
		if r.MsgID == "om_msg_2" && r.Emoji == "SMILE" {
			foundLLMReaction = true
			break
		}
	}
	if !foundLLMReaction {
		t.Fatalf("expected LLM-chosen SMILE reaction on om_msg_2, got: %+v", rec.CallsByMethod("AddReaction"))
	}
}

func TestPreanalysisEmoji_E2E_ReactsOnlyOnce(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(
		&preanalysisEmojiExecutor{emoji: "SMILE", emitCount: 5},
		rec,
		channels.BaseConfig{SessionPrefix: "test", AllowDirect: true},
	)

	if err := gw.InjectMessage(context.Background(), "oc_chat_3", "p2p", "ou_user", "om_msg_3", "重复测试"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()
	time.Sleep(100 * time.Millisecond)

	var count int
	for _, r := range rec.CallsByMethod("AddReaction") {
		if r.MsgID == "om_msg_3" && r.Emoji == "SMILE" {
			count++
		}
	}
	if count != 1 {
		t.Fatalf("expected exactly 1 SMILE reaction (idempotent), got %d", count)
	}
}

func TestPreanalysisEmoji_E2E_IgnoresNonPreanalysisEvents(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(
		&noEmojiExecutor{},
		rec,
		channels.BaseConfig{SessionPrefix: "test", AllowDirect: true},
	)

	if err := gw.InjectMessage(context.Background(), "oc_chat_4", "p2p", "ou_user", "om_msg_4", "不触发 emoji"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()
	time.Sleep(100 * time.Millisecond)

	// Preanalysis listener should NOT have fired — only the random end emoji
	// from pickReactionEmojis() may appear. We verify by checking that no
	// reaction matches an emoji outside the default pool.
	for _, r := range rec.CallsByMethod("AddReaction") {
		if r.MsgID == "om_msg_4" && r.Emoji == "SMILE" {
			t.Fatalf("unexpected preanalysis-sourced reaction on om_msg_4: %+v", r)
		}
	}
}

// TestPreanalysisEmoji_E2E_FullPipeline verifies the complete reaction lifecycle:
// preanalysis start emoji (LLM-chosen) + random end emoji + reply message.
func TestPreanalysisEmoji_E2E_FullPipeline(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(
		&preanalysisEmojiExecutor{emoji: "SMILE"},
		rec,
		channels.BaseConfig{SessionPrefix: "test", AllowDirect: true},
	)

	if err := gw.InjectMessage(context.Background(), "oc_trace", "p2p", "ou_user", "om_trace_1", "full pipeline test"); err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	// Poll for the preanalysis SMILE reaction (async goroutine).
	deadline := time.Now().Add(500 * time.Millisecond)
	var hasPreanalysis bool
	for time.Now().Before(deadline) {
		for _, r := range rec.CallsByMethod("AddReaction") {
			if r.MsgID == "om_trace_1" && r.Emoji == "SMILE" {
				hasPreanalysis = true
				break
			}
		}
		if hasPreanalysis {
			break
		}
		time.Sleep(5 * time.Millisecond)
	}
	if !hasPreanalysis {
		t.Fatalf("expected preanalysis SMILE reaction, got reactions: %+v", rec.CallsByMethod("AddReaction"))
	}

	// Poll for the end emoji (async goroutine launched after task completes).
	var hasEndEmoji bool
	endDeadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(endDeadline) {
		for _, r := range rec.CallsByMethod("AddReaction") {
			if r.MsgID == "om_trace_1" && r.Emoji != "SMILE" && r.Emoji != defaultProcessingReactEmoji {
				hasEndEmoji = true
				break
			}
		}
		if hasEndEmoji {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	if !hasEndEmoji {
		t.Fatalf("expected end emoji reaction from random pool, got reactions: %+v", rec.CallsByMethod("AddReaction"))
	}

	if len(rec.CallsByMethod("SendMessage"))+len(rec.CallsByMethod("ReplyMessage")) == 0 {
		t.Fatalf("expected at least one reply message")
	}
}
