package lark

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
)

func TestTeeMessengerCapturesTargetChat(t *testing.T) {
	inner := NewRecordingMessenger()
	tee := newTeeMessenger(inner, "target-chat")

	ctx := context.Background()

	// Send to target chat — should be captured.
	if _, err := tee.SendMessage(ctx, "target-chat", "text", `{"text":"hello"}`); err != nil {
		t.Fatal(err)
	}
	// Send to other chat — should NOT be captured.
	if _, err := tee.SendMessage(ctx, "other-chat", "text", `{"text":"bye"}`); err != nil {
		t.Fatal(err)
	}

	captured := tee.captured()
	if len(captured) != 1 {
		t.Fatalf("expected 1 captured call, got %d", len(captured))
	}
	if captured[0].ChatID != "target-chat" {
		t.Fatalf("expected captured chat_id='target-chat', got %q", captured[0].ChatID)
	}

	// Inner should have received both calls.
	innerCalls := inner.Calls()
	if len(innerCalls) != 2 {
		t.Fatalf("expected inner to receive 2 calls, got %d", len(innerCalls))
	}
}

func TestTeeMessengerCapturesReplyAndReaction(t *testing.T) {
	inner := NewRecordingMessenger()
	tee := newTeeMessenger(inner, "chat-1")

	ctx := context.Background()

	if _, err := tee.ReplyMessage(ctx, "om_msg_1", "text", `{"text":"reply"}`); err != nil {
		t.Fatal(err)
	}
	if err := tee.AddReaction(ctx, "om_msg_1", "DONE"); err != nil {
		t.Fatal(err)
	}

	captured := tee.captured()
	if len(captured) != 2 {
		t.Fatalf("expected 2 captured calls, got %d", len(captured))
	}
	if captured[0].Method != "ReplyMessage" {
		t.Fatalf("expected ReplyMessage, got %q", captured[0].Method)
	}
	if captured[1].Method != "AddReaction" {
		t.Fatalf("expected AddReaction, got %q", captured[1].Method)
	}
}

func TestTeeMessengerDoesNotCaptureUploads(t *testing.T) {
	inner := NewRecordingMessenger()
	tee := newTeeMessenger(inner, "chat-1")

	ctx := context.Background()

	if _, err := tee.UploadImage(ctx, []byte("img")); err != nil {
		t.Fatal(err)
	}
	if _, err := tee.UploadFile(ctx, []byte("file"), "test.txt", "text"); err != nil {
		t.Fatal(err)
	}

	captured := tee.captured()
	if len(captured) != 0 {
		t.Fatalf("expected 0 captured calls for upload ops, got %d", len(captured))
	}
}

func TestInjectMessageSyncBasic(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "injected reply"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:  "inject-test-1",
		Text:    "hello from inject",
		Timeout: 10 * time.Second,
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.Duration <= 0 {
		t.Fatal("expected positive duration")
	}
	if len(resp.Replies) == 0 {
		t.Fatal("expected at least one reply")
	}

	// Verify the reply contains the expected answer.
	found := false
	for _, r := range resp.Replies {
		if strings.Contains(r.Content, "injected reply") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reply containing 'injected reply', got %v", resp.Replies)
	}

	// Verify executor received the task.
	if executor.capturedTask != "hello from inject" {
		t.Fatalf("expected task 'hello from inject', got %q", executor.capturedTask)
	}
}

func TestInjectMessageSyncDefaults(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "ok"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		Text:    "defaults test",
		Timeout: 10 * time.Second,
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	// ChatID should have been auto-generated with "inject-" prefix.
	// No easy way to check the generated chatID from here, but no error means it worked.
}

func TestInjectMessageSyncTimeout(t *testing.T) {
	rec := NewRecordingMessenger()
	// Executor that blocks for a while.
	executor := &slowExecutor{
		delay:  2 * time.Second,
		result: &agent.TaskResult{Answer: "slow"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:  "inject-timeout",
		Text:    "slow task",
		Timeout: 500 * time.Millisecond,
	})

	if resp.Error == "" {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(resp.Error, "timeout") {
		t.Fatalf("expected timeout error, got: %s", resp.Error)
	}

	// Clean up: wait for the slow executor to finish.
	gw.WaitForTasks()
}

func TestInjectMessageSyncContextCancelled(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &slowExecutor{
		delay:  5 * time.Second,
		result: &agent.TaskResult{Answer: "slow"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(300 * time.Millisecond)
		cancel()
	}()

	resp := gw.InjectMessageSync(ctx, InjectSyncRequest{
		ChatID:  "inject-cancel",
		Text:    "cancelled task",
		Timeout: 30 * time.Second,
	})

	if resp.Error == "" {
		t.Fatal("expected context cancelled error")
	}

	// Clean up.
	gw.WaitForTasks()
}

func TestInjectMessageSyncRestoresMessenger(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "ok"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	originalMessenger := gw.messenger

	_ = gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:  "inject-restore",
		Text:    "restore test",
		Timeout: 10 * time.Second,
	})

	if gw.messenger != originalMessenger {
		t.Fatal("expected messenger to be restored after InjectMessageSync")
	}
}

// --- test helpers ---

type slowExecutor struct {
	stubExecutor
	delay  time.Duration
	result *agent.TaskResult
}

func (s *slowExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	select {
	case <-time.After(s.delay):
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	return s.result, nil
}
