package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	builtinshared "alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/logging"

)

func TestInjectMessageBasicP2P(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "hello back"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	err := gw.InjectMessage(context.Background(), "oc_chat_1", "p2p", "ou_user_1", "om_msg_1", "hello")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if executor.capturedTask != "hello" {
		t.Fatalf("expected task 'hello', got %q", executor.capturedTask)
	}

	replies := rec.CallsByMethod("ReplyMessage")
	if len(replies) == 0 {
		t.Fatal("expected at least one ReplyMessage call")
	}
	if !strings.Contains(replies[0].Content, "hello back") {
		t.Fatalf("expected reply to contain 'hello back', got %q", replies[0].Content)
	}
}

func TestInjectMessageGroupChat(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "group reply"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowGroups:   true,
	})

	err := gw.InjectMessage(context.Background(), "oc_group_1", "group", "ou_user_1", "om_msg_1", "question")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if executor.capturedTask != "question" {
		t.Fatalf("expected task 'question', got %q", executor.capturedTask)
	}

	replies := rec.CallsByMethod("ReplyMessage")
	if len(replies) == 0 {
		t.Fatal("expected at least one ReplyMessage call")
	}
}

func TestInjectMessageTopicGroupChatTreatedAsGroup(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "topic reply"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowGroups:   true,
		AllowDirect:   false,
	})

	err := gw.InjectMessage(context.Background(), "oc_topic_group_1", "topic_group", "ou_user_1", "om_msg_1", "question")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if executor.capturedTask != "question" {
		t.Fatalf("expected task 'question', got %q", executor.capturedTask)
	}

	replies := rec.CallsByMethod("ReplyMessage")
	if len(replies) == 0 {
		t.Fatal("expected at least one ReplyMessage call")
	}
}

func TestInjectMessageDefaultsChatType(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "ok"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	// Empty chatType should default to "p2p".
	err := gw.InjectMessage(context.Background(), "oc_chat_1", "", "ou_user_1", "om_msg_1", "test")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if executor.capturedTask != "test" {
		t.Fatalf("expected task 'test', got %q", executor.capturedTask)
	}
}

func TestInjectMessageDedup(t *testing.T) {
	rec := NewRecordingMessenger()
	callCount := 0
	executor := &stubExecutorFunc{
		fn: func() {
			callCount++
		},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	// First message should be processed.
	err := gw.InjectMessage(context.Background(), "oc_chat_1", "p2p", "ou_user_1", "om_dup_1", "first")
	if err != nil {
		t.Fatalf("first InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()
	if callCount != 1 {
		t.Fatalf("expected 1 execute call, got %d", callCount)
	}

	// Second message with same ID should be deduped.
	err = gw.InjectMessage(context.Background(), "oc_chat_1", "p2p", "ou_user_1", "om_dup_1", "second")
	if err != nil {
		t.Fatalf("second InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()
	if callCount != 1 {
		t.Fatalf("expected dedup to prevent second execute, got %d calls", callCount)
	}
}

func TestInjectMessageWithReaction(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "done"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})
	err := gw.InjectMessage(context.Background(), "oc_chat_1", "p2p", "ou_user_1", "om_msg_react", "with emoji")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	// Wait briefly for async reactions.
	time.Sleep(50 * time.Millisecond)

	reactions := rec.CallsByMethod("AddReaction")
	if len(reactions) < 1 {
		t.Fatalf("expected at least 1 AddReaction call, got %d", len(reactions))
	}
}

func TestInjectMessageResetCommand(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &resetExecutor{}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	err := gw.InjectMessage(context.Background(), "oc_chat_1", "p2p", "ou_user_1", "om_msg_reset", "/reset")
	if err != nil {
		t.Fatalf("InjectMessage failed: %v", err)
	}
	gw.WaitForTasks()

	if executor.executeCalled {
		t.Fatal("expected ExecuteTask to be skipped on /reset")
	}
	if executor.resetCalled {
		t.Fatal("expected /reset to be deprecated and skip ResetSession")
	}

	replies := rec.CallsByMethod("ReplyMessage")
	if len(replies) == 0 {
		t.Fatal("expected reset deprecation reply")
	}
	if !strings.Contains(replies[0].Content, "/new") {
		t.Fatalf("expected deprecation hint for /new, got %q", replies[0].Content)
	}
}

func TestBuildExecContextInjectsLarkMessenger(t *testing.T) {
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(&capturingExecutor{}, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	msg := &incomingMessage{
		senderID:  "ou_user_1",
		chatID:    "inject-chat-1",
		messageID: "inject_msg_1",
	}
	execCtx, cancel := gw.buildExecContext(context.Background(), msg, "session-1", make(chan agent.UserInput, 1))
	defer cancel()

	messenger := builtinshared.LarkMessengerFromContext(execCtx)
	if messenger == nil {
		t.Fatal("expected lark messenger in execution context")
	}
}

// --- test helpers ---

func newTestGatewayWithMessenger(exec AgentExecutor, messenger LarkMessenger, baseCfg channels.BaseConfig) *Gateway {
		return &Gateway{
		cfg:         Config{BaseConfig: baseCfg, AppID: "test", AppSecret: "secret"},
		agent:       exec,
		logger:      logging.OrNop(nil),
		messenger:   wrapInjectCaptureHub(messenger),
		dedup: newEventDedup(nil),
		now:         time.Now,
	}
}

type stubExecutorFunc struct {
	stubExecutor
	fn func()
}

func (s *stubExecutorFunc) ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error) {
	if s.fn != nil {
		s.fn()
	}
	return s.stubExecutor.ExecuteTask(ctx, task, sessionID, listener)
}
