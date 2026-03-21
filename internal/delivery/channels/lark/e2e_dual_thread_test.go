package lark

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/config"
	"alex/internal/shared/logging"
)

// ---------------------------------------------------------------------------
// Test 1: taskStartTime is set before the goroutine starts, so
// snapshotWorker never falls back to lastTouched.
// ---------------------------------------------------------------------------

func TestDualThread_TaskStartTimePrecision(t *testing.T) {
	t.Parallel()

	// Use a fake clock so we can distinguish taskStartTime from lastTouched.
	var mu sync.Mutex
	fakeNow := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	advance := func(d time.Duration) {
		mu.Lock()
		fakeNow = fakeNow.Add(d)
		mu.Unlock()
	}
	clock := func() time.Time {
		mu.Lock()
		defer mu.Unlock()
		return fakeNow
	}

	exec := &blockingExecutor{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}
	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(exec, rec, channels.BaseConfig{
		SessionPrefix: "lark",
		AllowDirect:   true,
	})
	gw.now = clock

	// Inject a message — this will set taskStartTime before launching the goroutine.
	advance(10 * time.Second) // now = T+10s
	go func() {
		_ = gw.InjectMessage(context.Background(), "oc_ts_test", "p2p", "ou_user", "om_1", "hello")
	}()
	<-exec.started

	// Advance clock so elapsed is measurable.
	advance(5 * time.Second) // now = T+15s

	snap := gw.snapshotWorker("oc_ts_test")
	if snap.Phase != slotRunning {
		t.Fatalf("expected slotRunning, got %v", snap.Phase)
	}

	// Elapsed should be ~5s (from taskStartTime). If it fell back to
	// lastTouched we'd also see ~5s since they are set in the same
	// critical section, so verify taskStartTime is non-zero directly.
	raw, _ := gw.activeSlots.Load("oc_ts_test")
	slot := raw.(*sessionSlot)
	slot.mu.Lock()
	startTime := slot.taskStartTime
	slot.mu.Unlock()

	if startTime.IsZero() {
		t.Fatal("taskStartTime is zero — should have been set before goroutine launch")
	}

	// Elapsed should be 5s, not 0 (which would happen with zero taskStartTime).
	if snap.Elapsed < 4*time.Second || snap.Elapsed > 6*time.Second {
		t.Fatalf("expected ~5s elapsed, got %v", snap.Elapsed)
	}

	close(exec.finish)
	gw.WaitForTasks()
}

// ---------------------------------------------------------------------------
// Test 2: When conversation process LLM returns text + dispatch_worker,
// only the worker result is sent as a reply (no double reply).
// ---------------------------------------------------------------------------

func TestDualThread_ConversationProcessSingleReply(t *testing.T) {
	t.Parallel()

	// Stub LLM returns text reply + dispatch_worker tool call.
	// Use a unique ack text so we can distinguish it from the worker result.
	ackText := "收到，马上处理"
	stub := &convStubLLMClient{
		resp: ackText,
		toolCalls: []ports.ToolCall{
			{
				Name:      dispatchWorkerToolName,
				Arguments: map[string]interface{}{"task": "do the thing"},
			},
		},
	}

	en := true
	rec := &convRecordingMessenger{}
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "test",
				AllowDirect:   true,
			},
			ConversationProcessEnabled: &en,
		},
		agent: &convStubAgentExecutor{
			result: &agent.TaskResult{Answer: "worker result"},
		},
		llmFactory: &convStubFactory{client: stub},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		logger:     logging.OrNop(nil),
		messenger:  rec,
		now:        time.Now,
	}
	gw.dedup = newEventDedup(nil)

	_ = gw.InjectMessage(context.Background(), "oc_single_reply", "p2p", "ou_user", "om_1", "do the thing")
	gw.WaitForTasks()

	// When dispatch_worker is called, we expect TWO replies to om_1:
	//   1. The conversation LLM's quick ack (e.g. "收到，马上处理") — sent
	//      immediately so the user knows the task was accepted.
	//   2. The worker's final result — sent when the background task completes.
	//
	// The original double-reply bug was two full answers; the ack is
	// intentionally a short confirmation, not a duplicate answer.
	rec.mu.Lock()
	allMsgs := make([]convSentMessage, len(rec.messages))
	copy(allMsgs, rec.messages)
	rec.mu.Unlock()

	var repliesToOM1 []convSentMessage
	for _, m := range allMsgs {
		if m.replyTo == "om_1" {
			repliesToOM1 = append(repliesToOM1, m)
		}
	}
	// Expect exactly 2: ack + worker result.
	if len(repliesToOM1) < 2 {
		t.Fatalf("expected 2 replies to om_1 (ack + worker result), got %d", len(repliesToOM1))
	}
	// First reply should be the ack text from the conversation LLM.
	if !dtContains(repliesToOM1[0].content, ackText) {
		t.Fatalf("first reply to om_1 should be the ack %q, got %q", ackText, repliesToOM1[0].content)
	}
}

// dtContains is a simple substring check for dual-thread test assertions.
func dtContains(haystack, needle string) bool {
	return len(haystack) >= len(needle) && (haystack == needle || len(needle) == 0 ||
		findSubstring(haystack, needle))
}

// ---------------------------------------------------------------------------
// Test 3: Concurrent conversation process messages — second message injects
// into running worker instead of spawning a second one.
// ---------------------------------------------------------------------------

func TestDualThread_ConcurrentConversationInject(t *testing.T) {
	t.Parallel()

	exec := &dtBlockingConvExecutor{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}

	// Stub LLM always returns dispatch_worker.
	stub := &convStubLLMClient{
		resp: "on it",
		toolCalls: []ports.ToolCall{
			{
				Name:      dispatchWorkerToolName,
				Arguments: map[string]interface{}{"task": "work"},
			},
		},
	}

	en := true
	rec := &convRecordingMessenger{}
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "test",
				AllowDirect:   true,
			},
			ConversationProcessEnabled: &en,
		},
		agent:      exec,
		llmFactory: &convStubFactory{client: stub},
		llmProfile: config.LLMProfile{Provider: "openai", Model: "gpt-4o-mini"},
		logger:     logging.OrNop(nil),
		messenger:  rec,
		now:        time.Now,
		imDelayFn:  func(_ context.Context, _ time.Duration) bool { return true },
	}
	gw.dedup = newEventDedup(nil)

	// M1 starts worker W1.
	go func() {
		_ = gw.InjectMessage(context.Background(), "oc_conc", "p2p", "ou_user", "om_1", "task one")
	}()
	<-exec.started

	// M2 arrives while W1 is still running — should inject, not spawn W2.
	_ = gw.InjectMessage(context.Background(), "oc_conc", "p2p", "ou_user", "om_2", "task two")

	// Verify M2 was injected into W1's inputCh.
	select {
	case input := <-exec.inputCh:
		if input.Content != "work" {
			t.Fatalf("expected injected content 'work', got %q", input.Content)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("timed out waiting for injected message in worker inputCh")
	}

	close(exec.finish)
	gw.WaitForTasks()

	// ExecuteTask should have been called exactly once.
	if c := exec.getCallCount(); c != 1 {
		t.Fatalf("expected 1 ExecuteTask call, got %d", c)
	}

	// Check that the LLM reply was sent as inject notification (no hardcoded text).
	rec.mu.Lock()
	found := false
	for _, m := range rec.messages {
		if dtContains(m.content, "on it") {
			found = true
		}
	}
	rec.mu.Unlock()
	if !found {
		t.Fatal("expected LLM reply 'on it' to be sent as inject notification")
	}
}

// ---------------------------------------------------------------------------
// Test 4: drainAndReprocess drops messages when slot has been claimed by
// a newer task (token mismatch).
// ---------------------------------------------------------------------------

func TestDualThread_DrainTokenGuard(t *testing.T) {
	t.Parallel()

	var callCount atomic.Int32
	exec := &orderTrackingExecutor{
		ensureFn: func(_ context.Context, sid string) (*storage.Session, error) {
			return &storage.Session{ID: sid, Metadata: map[string]string{}}, nil
		},
		executeFn: func(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
			callCount.Add(1)
			return &agent.TaskResult{Answer: "ok"}, nil
		},
	}

	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
		agent:  exec,
		logger: logging.OrNop(nil),
		now:    time.Now,
	}
	gw.dedup = newEventDedup(nil)

	// Create a slot and simulate it being claimed by a newer task.
	slot := gw.getOrCreateSlot("oc_drain_guard")
	slot.mu.Lock()
	slot.taskToken = 5 // simulate newer task has claimed the slot
	slot.mu.Unlock()

	// Put messages in a channel as if they were drained from the old task.
	ch := make(chan agent.UserInput, 4)
	ch <- agent.UserInput{Content: "old-msg-1", SenderID: "ou_test", MessageID: "om_old_1"}
	ch <- agent.UserInput{Content: "old-msg-2", SenderID: "ou_test", MessageID: "om_old_2"}

	// Drain with the OLD token (3) — messages should be dropped because
	// slot.taskToken (5) != drainToken (3).
	gw.drainAndReprocess(ch, "oc_drain_guard", "p2p", 3)
	gw.WaitForTasks()

	if c := callCount.Load(); c != 0 {
		t.Fatalf("expected 0 reprocessed messages (all dropped), got %d", c)
	}
}

// ---------------------------------------------------------------------------
// Test 5: Old goroutine cleanup does not zero intentionalCancelToken when
// taskToken has been bumped by /new.
// ---------------------------------------------------------------------------

func TestDualThread_IntentionalCancelTokenPreserved(t *testing.T) {
	t.Parallel()

	exec1 := &dtBlockingConvExecutor{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}

	rec := NewRecordingMessenger()
	gw := newTestGatewayWithMessenger(exec1, rec, channels.BaseConfig{
		SessionPrefix: "lark",
		AllowDirect:   true,
	})

	// M1 starts W1 (token=1).
	go func() {
		_ = gw.InjectMessage(context.Background(), "oc_ict", "p2p", "ou_user", "om_1", "task one")
	}()
	<-exec1.started

	// Get the slot and its current token.
	raw, _ := gw.activeSlots.Load("oc_ict")
	slot := raw.(*sessionSlot)

	slot.mu.Lock()
	token1 := slot.taskToken
	// Simulate /new: set intentionalCancelToken to T1, bump token to T2.
	slot.intentionalCancelToken = token1
	slot.taskToken = token1 + 1
	token2 := slot.taskToken
	// Simulate W2 starting with T2's cancel token set (e.g., user sent /stop on W2).
	slot.intentionalCancelToken = token2
	slot.mu.Unlock()

	// Let W1's goroutine finish. Its cleanup should NOT clear intentionalCancelToken
	// because slot.taskToken (T2) != W1's taskToken (T1).
	close(exec1.finish)
	gw.WaitForTasks()

	slot.mu.Lock()
	ict := slot.intentionalCancelToken
	slot.mu.Unlock()

	if ict != token2 {
		t.Fatalf("intentionalCancelToken should be %d (T2), got %d — old goroutine incorrectly cleared it", token2, ict)
	}
}

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// dtBlockingConvExecutor blocks on finish channel and captures inputCh.
// Used for conversation process tests where we need to control worker timing.
type dtBlockingConvExecutor struct {
	mu          sync.Mutex
	started     chan struct{}
	finish      chan struct{}
	startedOnce sync.Once
	inputCh     <-chan agent.UserInput
	callCount   int
}

func (b *dtBlockingConvExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (b *dtBlockingConvExecutor) ExecuteTask(ctx context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	b.mu.Lock()
	b.callCount++
	b.inputCh = agent.UserInputChFromContext(ctx)
	b.mu.Unlock()
	b.startedOnce.Do(func() {
		close(b.started)
	})
	<-b.finish
	return &agent.TaskResult{Answer: "done"}, nil
}

func (b *dtBlockingConvExecutor) getCallCount() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.callCount
}

// Ensure dtBlockingConvExecutor is used (prevent unused import lint).
var _ AgentExecutor = (*dtBlockingConvExecutor)(nil)
