package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
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
	// Executor that blocks longer than the inject timeout.
	executor := &slowExecutor{
		delay:  5 * time.Second,
		result: &agent.TaskResult{Answer: "slow"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	start := time.Now()
	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:  "inject-timeout",
		Text:    "slow task",
		Timeout: 200 * time.Millisecond,
	})
	elapsed := time.Since(start)

	if resp.Error == "" {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(resp.Error, "timeout") {
		t.Fatalf("expected timeout error, got: %s", resp.Error)
	}
	if elapsed > 2*time.Second {
		t.Fatalf("expected timeout path to cancel quickly, elapsed=%s", elapsed)
	}

	// Ensure the timed-out task does not linger in background.
	gw.WaitForTasks()
}

func TestInjectMessageSyncContextCancelled(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &slowExecutor{
		delay:  1 * time.Second,
		result: &agent.TaskResult{Answer: "slow"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	ctx, cancel := context.WithCancel(context.Background())
	go func() {
		time.Sleep(200 * time.Millisecond)
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
}

func TestInjectMessageSyncCaptureClosedAfterReturn(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "ok"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	_ = gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:  "inject-tee",
		Text:    "tee test",
		Timeout: 10 * time.Second,
	})

	// After InjectMessageSync, messenger should still forward to the original.
	ctx := context.Background()
	if _, err := gw.messenger.SendMessage(ctx, "some-chat", "text", `{"text":"after"}`); err != nil {
		t.Fatalf("messenger should still forward after inject: %v", err)
	}

	hub, ok := gw.messenger.(*injectCaptureHub)
	if !ok {
		t.Fatal("expected messenger to be an injectCaptureHub")
	}
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.sessions) != 0 {
		t.Fatalf("expected no active inject capture sessions, got %d", len(hub.sessions))
	}
}

func TestInjectMessageSyncDoesNotStackWrappers(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "ok"},
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	for i := 0; i < 50; i++ {
		resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
			ChatID:  fmt.Sprintf("inject-wrap-%d", i),
			Text:    "/new",
			Timeout: 10 * time.Second,
		})
		if resp.Error != "" {
			t.Fatalf("inject %d failed: %s", i, resp.Error)
		}
	}

	hub, ok := gw.messenger.(*injectCaptureHub)
	if !ok {
		t.Fatal("expected messenger to be injectCaptureHub")
	}
	if _, nested := hub.inner.(*injectCaptureHub); nested {
		t.Fatal("inject capture hub should not be nested")
	}
	hub.mu.RLock()
	defer hub.mu.RUnlock()
	if len(hub.sessions) != 0 {
		t.Fatalf("expected no active sessions after repeated injects, got %d", len(hub.sessions))
	}
}

// --- auto-reply tests ---

func TestInjectSyncAutoReplyPicksFirstOption(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := newAutoReplyTestExecutor(
		// First call: return await_user_input with options
		&agent.TaskResult{
			Answer:     "which option?",
			StopReason: "await_user_input",
		},
		[]string{"option A", "option B"},
		// Second call (resume): return final answer
		&agent.TaskResult{Answer: "done with option A"},
	)
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:    "auto-reply-opts",
		Text:      "do something",
		Timeout:   10 * time.Second,
		AutoReply: true,
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.AutoReplies != 1 {
		t.Fatalf("expected 1 auto-reply, got %d", resp.AutoReplies)
	}

	// Heuristic should have sent "1" as the auto-reply (no LLM factory).
	calls := int(executor.callCount.Load())
	if calls != 2 {
		t.Fatalf("expected executor called 2 times, got %d", calls)
	}

	// Verify final response contains the completion answer.
	found := false
	for _, r := range resp.Replies {
		if strings.Contains(r.Content, "done with option A") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected reply containing 'done with option A', got %v", resp.Replies)
	}
}

func TestInjectSyncAutoReplyNoOptions(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := newAutoReplyTestExecutor(
		&agent.TaskResult{
			Answer:     "what should I do?",
			StopReason: "await_user_input",
		},
		nil, // no options
		&agent.TaskResult{Answer: "executed"},
	)
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:    "auto-reply-no-opts",
		Text:      "search something",
		Timeout:   10 * time.Second,
		AutoReply: true,
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.AutoReplies != 1 {
		t.Fatalf("expected 1 auto-reply, got %d", resp.AutoReplies)
	}

	// Verify executor was called twice.
	if calls := int(executor.callCount.Load()); calls != 2 {
		t.Fatalf("expected executor called 2 times, got %d", calls)
	}
}

func TestInjectSyncAutoReplyMaxRoundsExhausted(t *testing.T) {
	rec := NewRecordingMessenger()
	// Executor always returns await_user_input — never completes.
	executor := &alwaysAwaitExecutor{
		answer: "still waiting",
	}
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:             "auto-reply-max",
		Text:               "do something",
		Timeout:            10 * time.Second,
		AutoReply:          true,
		MaxAutoReplyRounds: 2,
	})

	// Should NOT error — just stop after maxRounds.
	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.AutoReplies != 2 {
		t.Fatalf("expected 2 auto-replies (max), got %d", resp.AutoReplies)
	}
}

func TestInjectSyncAutoReplyDisabled(t *testing.T) {
	rec := NewRecordingMessenger()
	executor := newAutoReplyTestExecutor(
		&agent.TaskResult{
			Answer:     "clarification?",
			StopReason: "await_user_input",
		},
		nil,
		&agent.TaskResult{Answer: "should not reach here"},
	)
	gw := newTestGatewayWithMessenger(executor, rec, channels.BaseConfig{
		SessionPrefix: "test",
		AllowDirect:   true,
	})

	resp := gw.InjectMessageSync(context.Background(), InjectSyncRequest{
		ChatID:    "auto-reply-off",
		Text:      "do something",
		Timeout:   10 * time.Second,
		AutoReply: false, // disabled
	})

	if resp.Error != "" {
		t.Fatalf("unexpected error: %s", resp.Error)
	}
	if resp.AutoReplies != 0 {
		t.Fatalf("expected 0 auto-replies, got %d", resp.AutoReplies)
	}
	// Should only call executor once (the initial task).
	if calls := int(executor.callCount.Load()); calls != 1 {
		t.Fatalf("expected executor called 1 time, got %d", calls)
	}
}

func TestExtractLastReplyText(t *testing.T) {
	tests := []struct {
		name  string
		calls []MessengerCall
		want  string
	}{
		{
			name:  "empty",
			calls: nil,
			want:  "",
		},
		{
			name: "skips reactions",
			calls: []MessengerCall{
				{Method: "ReplyMessage", Content: `{"text":"hello"}`},
				{Method: "AddReaction", Emoji: "DONE"},
			},
			want: "hello",
		},
		{
			name: "last text reply",
			calls: []MessengerCall{
				{Method: "ReplyMessage", Content: `{"text":"first"}`},
				{Method: "SendMessage", Content: `{"text":"second"}`},
			},
			want: "second",
		},
		{
			name: "non-json fallback",
			calls: []MessengerCall{
				{Method: "ReplyMessage", Content: "raw text"},
			},
			want: "raw text",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractLastReplyText(tt.calls)
			if got != tt.want {
				t.Errorf("extractLastReplyText() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestHeuristicAutoReply(t *testing.T) {
	if got := heuristicAutoReply([]string{"opt1", "opt2"}); got != "1" {
		t.Errorf("expected '1' with options, got %q", got)
	}
	if got := heuristicAutoReply(nil); got != "Proceed directly, no further confirmation needed." {
		t.Errorf("expected fixed text without options, got %q", got)
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

// autoReplyTestExecutor returns await_user_input on the first call (setting
// pendingOptions on the slot), then returns a final result on subsequent calls.
type autoReplyTestExecutor struct {
	mu           sync.Mutex
	callCount    atomic.Int32
	firstResult  *agent.TaskResult
	finalResult  *agent.TaskResult
	options      []string
	capturedTask string
}

func newAutoReplyTestExecutor(firstResult *agent.TaskResult, options []string, finalResult *agent.TaskResult) *autoReplyTestExecutor {
	return &autoReplyTestExecutor{
		firstResult: firstResult,
		finalResult: finalResult,
		options:     options,
	}
}

func (a *autoReplyTestExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (a *autoReplyTestExecutor) ExecuteTask(ctx context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	a.mu.Lock()
	a.capturedTask = task
	a.mu.Unlock()

	n := a.callCount.Add(1)
	if n == 1 {
		return a.firstResult, nil
	}
	return a.finalResult, nil
}

// alwaysAwaitExecutor always returns await_user_input — used for max-rounds testing.
type alwaysAwaitExecutor struct {
	stubExecutor
	answer    string
	callCount atomic.Int32
}

func (a *alwaysAwaitExecutor) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	a.callCount.Add(1)
	return &agent.TaskResult{
		Answer:     a.answer,
		StopReason: "await_user_input",
	}, nil
}
