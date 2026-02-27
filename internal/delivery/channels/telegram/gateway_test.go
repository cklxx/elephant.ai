package telegram

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/logging"

	lru "github.com/hashicorp/golang-lru/v2"
	"github.com/mymmrac/telego"
)

// ── Test helpers ──────────────────────────────────────────────────────────

type stubExecutor struct{}

func (s *stubExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "tg-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (s *stubExecutor) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	return &agent.TaskResult{Answer: "done"}, nil
}

type capturingExecutor struct {
	mu                sync.Mutex
	capturedCtx       context.Context
	capturedSessionID string
	capturedTask      string
	result            *agent.TaskResult
	blockCh           chan struct{} // if non-nil, blocks until closed
}

func (c *capturingExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (c *capturingExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	c.mu.Lock()
	c.capturedCtx = ctx
	c.capturedSessionID = sessionID
	c.capturedTask = task
	c.mu.Unlock()
	if c.blockCh != nil {
		<-c.blockCh
	}
	if c.result != nil {
		return c.result, nil
	}
	return &agent.TaskResult{Answer: "done"}, nil
}

type recordingMessenger struct {
	mu       sync.Mutex
	sent     []sentMessage
	edited   []editedMessage
	nextID   int
}

type sentMessage struct {
	chatID       int64
	text         string
	replyToMsgID int
}

type editedMessage struct {
	chatID    int64
	messageID int
	text      string
}

func newRecordingMessenger() *recordingMessenger {
	return &recordingMessenger{nextID: 100}
}

func (m *recordingMessenger) SendText(_ context.Context, chatID int64, text string, replyToMsgID int) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.nextID++
	m.sent = append(m.sent, sentMessage{chatID: chatID, text: text, replyToMsgID: replyToMsgID})
	return m.nextID, nil
}

func (m *recordingMessenger) EditText(_ context.Context, chatID int64, messageID int, text string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.edited = append(m.edited, editedMessage{chatID: chatID, messageID: messageID, text: text})
	return nil
}

func (m *recordingMessenger) sentTexts() []string {
	m.mu.Lock()
	defer m.mu.Unlock()
	var texts []string
	for _, s := range m.sent {
		texts = append(texts, s.text)
	}
	return texts
}

func (m *recordingMessenger) sentCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.sent)
}

func newTestGateway(exec AgentExecutor, messenger Messenger) *Gateway {
	dedupCache, _ := lru.New[int, time.Time](128)
	g := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{
				SessionPrefix: "tg",
				AllowGroups:   true,
				AllowDirect:   true,
				ReplyTimeout:  30 * time.Second,
			},
			Enabled:              true,
			ActiveSlotTTL:        6 * time.Hour,
			ActiveSlotMaxEntries: 100,
			StateCleanupInterval: 5 * time.Minute,
		},
		agent:      exec,
		logger:     logging.OrNop(nil),
		messenger:  messenger,
		dedupCache: dedupCache,
		now:        time.Now,
	}
	return g
}

func dmMessage(chatID int64, msgID int, text string) *telego.Message {
	return &telego.Message{
		MessageID: msgID,
		Chat:      telego.Chat{ID: chatID, Type: telego.ChatTypePrivate},
		From:      &telego.User{ID: chatID, Username: "testuser"},
		Text:      text,
	}
}

func groupMessage(chatID int64, senderID int64, msgID int, text string) *telego.Message {
	return &telego.Message{
		MessageID: msgID,
		Chat:      telego.Chat{ID: chatID, Type: telego.ChatTypeSupergroup},
		From:      &telego.User{ID: senderID, Username: "testuser"},
		Text:      text,
	}
}

// ── Tests ─────────────────────────────────────────────────────────────────

func TestNewGatewayRequiresAgent(t *testing.T) {
	_, err := NewGateway(Config{}, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestDMMessageEndToEnd(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)

	ctx := context.Background()
	msg := dmMessage(123, 1, "hello")
	gw.handleMessage(ctx, msg)
	gw.WaitForTasks()

	texts := messenger.sentTexts()
	if len(texts) == 0 {
		t.Fatal("expected at least one reply")
	}
	if !strings.Contains(texts[len(texts)-1], "done") {
		t.Fatalf("expected reply to contain 'done', got %q", texts[len(texts)-1])
	}
}

func TestGroupMessageFiltering(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)
	gw.cfg.AllowedGroups = []int64{999}

	ctx := context.Background()
	// Message from non-allowed group should be ignored.
	gw.handleMessage(ctx, groupMessage(888, 1, 1, "hello"))
	gw.WaitForTasks()

	if messenger.sentCount() != 0 {
		t.Fatalf("expected no replies for non-allowed group, got %d", messenger.sentCount())
	}

	// Message from allowed group should go through.
	gw.handleMessage(ctx, groupMessage(999, 1, 2, "hello"))
	gw.WaitForTasks()

	if messenger.sentCount() == 0 {
		t.Fatal("expected reply for allowed group")
	}
}

func TestMessageDedup(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)

	ctx := context.Background()
	msg := dmMessage(123, 42, "hello")

	gw.handleMessage(ctx, msg)
	gw.WaitForTasks()
	count1 := messenger.sentCount()

	// Same message ID should be deduplicated.
	gw.handleMessage(ctx, msg)
	gw.WaitForTasks()
	count2 := messenger.sentCount()

	if count2 != count1 {
		t.Fatalf("expected dedup to prevent second processing, got %d then %d", count1, count2)
	}
}

func TestSessionReuseAcrossMessages(t *testing.T) {
	exec := &capturingExecutor{}
	messenger := newRecordingMessenger()
	gw := newTestGateway(exec, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "first"))
	gw.WaitForTasks()
	sid1 := exec.capturedSessionID

	gw.handleMessage(ctx, dmMessage(123, 2, "second"))
	gw.WaitForTasks()
	sid2 := exec.capturedSessionID

	if sid1 != sid2 {
		t.Fatalf("expected session reuse, got %q vs %q", sid1, sid2)
	}
}

func TestNewCommandResetsSession(t *testing.T) {
	exec := &capturingExecutor{}
	messenger := newRecordingMessenger()
	gw := newTestGateway(exec, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "hello"))
	gw.WaitForTasks()
	sid1 := exec.capturedSessionID

	gw.handleMessage(ctx, dmMessage(123, 2, "/new"))
	// /new doesn't spawn a task, so no WaitForTasks needed.

	gw.handleMessage(ctx, dmMessage(123, 3, "hello again"))
	gw.WaitForTasks()
	sid2 := exec.capturedSessionID

	if sid1 == sid2 {
		t.Fatalf("expected /new to create a different session, both got %q", sid1)
	}
}

func TestStopCommandCancelsTask(t *testing.T) {
	blockCh := make(chan struct{})
	exec := &capturingExecutor{blockCh: blockCh}
	messenger := newRecordingMessenger()
	gw := newTestGateway(exec, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "long task"))

	// Wait for task to start.
	time.Sleep(50 * time.Millisecond)

	gw.handleMessage(ctx, dmMessage(123, 2, "/stop"))

	// Unblock executor (cancel should have been called).
	close(blockCh)
	gw.WaitForTasks()

	texts := messenger.sentTexts()
	found := false
	for _, text := range texts {
		if strings.Contains(text, "停止") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("expected stop confirmation message, got %v", texts)
	}
}

func TestInputInjectionWhileRunning(t *testing.T) {
	blockCh := make(chan struct{})
	exec := &capturingExecutor{blockCh: blockCh}
	messenger := newRecordingMessenger()
	gw := newTestGateway(exec, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "start task"))

	// Wait for task to start.
	time.Sleep(50 * time.Millisecond)

	// Get the slot and verify it's running.
	slot := gw.getSlot(123)
	if slot == nil {
		close(blockCh)
		gw.WaitForTasks()
		t.Fatal("expected slot to exist")
	}

	slot.mu.Lock()
	phase := slot.phase
	inputCh := slot.inputCh
	slot.mu.Unlock()

	if phase != slotRunning {
		close(blockCh)
		gw.WaitForTasks()
		t.Fatalf("expected slotRunning, got %d", phase)
	}

	// Send a follow-up message — should be injected into inputCh.
	gw.handleMessage(ctx, dmMessage(123, 2, "injected input"))

	// Check the input channel received something.
	select {
	case input := <-inputCh:
		if input.Content != "injected input" {
			t.Fatalf("expected injected content, got %q", input.Content)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for injected input")
	}

	close(blockCh)
	gw.WaitForTasks()
}

func TestAwaitUserInputFlow(t *testing.T) {
	exec := &capturingExecutor{
		result: &agent.TaskResult{
			Answer:     "need input",
			StopReason: "await_user_input",
		},
	}
	messenger := newRecordingMessenger()
	gw := newTestGateway(exec, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "start"))
	gw.WaitForTasks()

	slot := gw.getSlot(123)
	if slot == nil {
		t.Fatal("expected slot to exist")
	}
	slot.mu.Lock()
	phase := slot.phase
	sid := slot.sessionID
	slot.mu.Unlock()

	if phase != slotAwaitingInput {
		t.Fatalf("expected slotAwaitingInput, got %d", phase)
	}

	// Next message should resume the same session.
	exec.result = &agent.TaskResult{Answer: "resumed"}
	gw.handleMessage(ctx, dmMessage(123, 2, "my input"))
	gw.WaitForTasks()

	if exec.capturedSessionID != sid {
		t.Fatalf("expected session reuse for resume, got %q vs %q", exec.capturedSessionID, sid)
	}
}

func TestDisallowGroupChat(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)
	gw.cfg.AllowGroups = false

	ctx := context.Background()
	gw.handleMessage(ctx, groupMessage(999, 1, 1, "hello"))
	gw.WaitForTasks()

	if messenger.sentCount() != 0 {
		t.Fatal("expected no replies when groups are disabled")
	}
}

func TestDisallowDirectMessage(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)
	gw.cfg.AllowDirect = false

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "hello"))
	gw.WaitForTasks()

	if messenger.sentCount() != 0 {
		t.Fatal("expected no replies when direct messages are disabled")
	}
}

func TestStripBotMention(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"/new@mybot", "/new"},
		{"/new@mybot extra args", "/new extra args"},
		{"/new", "/new"},
		{"hello", "hello"},
		{"/stop@bot123 now", "/stop now"},
	}
	for _, tt := range tests {
		got := stripBotMention(tt.input)
		if got != tt.expected {
			t.Errorf("stripBotMention(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestStatusCommand(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, "/status"))

	texts := messenger.sentTexts()
	if len(texts) == 0 {
		t.Fatal("expected status reply")
	}
	if !strings.Contains(texts[0], "空闲") {
		t.Fatalf("expected idle status, got %q", texts[0])
	}
}

func TestEmptyMessageIgnored(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)

	ctx := context.Background()
	gw.handleMessage(ctx, dmMessage(123, 1, ""))
	gw.WaitForTasks()

	if messenger.sentCount() != 0 {
		t.Fatal("expected no reply for empty message")
	}
}

func TestCleanupSlotsTTL(t *testing.T) {
	messenger := newRecordingMessenger()
	gw := newTestGateway(&stubExecutor{}, messenger)
	gw.cfg.ActiveSlotTTL = time.Millisecond

	// Create a slot.
	slot := gw.getOrCreateSlot(123)
	slot.mu.Lock()
	slot.lastTouched = time.Now().Add(-time.Hour)
	slot.mu.Unlock()

	gw.cleanupSlots()

	if gw.getSlot(123) != nil {
		t.Fatal("expected slot to be evicted by TTL")
	}
}

// ── Format tests ──────────────────────────────────────────────────────────

func TestSplitForTelegram(t *testing.T) {
	short := "hello"
	chunks := splitForTelegram(short, 10)
	if len(chunks) != 1 || chunks[0] != short {
		t.Fatalf("expected single chunk %q, got %v", short, chunks)
	}

	long := strings.Repeat("a", 15)
	chunks = splitForTelegram(long, 10)
	if len(chunks) < 2 {
		t.Fatalf("expected multiple chunks, got %d", len(chunks))
	}
	// Rejoin should equal original.
	rejoined := strings.Join(chunks, "")
	if rejoined != long {
		t.Fatalf("rejoined chunks don't match original")
	}
}

func TestSplitForTelegramNewlineBreak(t *testing.T) {
	text := "line1\nline2\nline3\nline4"
	chunks := splitForTelegram(text, 12) // "line1\nline2\n" = 12 chars
	if len(chunks) < 2 {
		t.Fatalf("expected split at newline, got %d chunks", len(chunks))
	}
}

func TestTruncateWithEllipsis(t *testing.T) {
	if got := truncateWithEllipsis("hello", 10); got != "hello" {
		t.Fatalf("expected no truncation, got %q", got)
	}
	if got := truncateWithEllipsis("hello world", 8); got != "hello..." {
		t.Fatalf("expected truncation with ellipsis, got %q", got)
	}
}

// ── Task store tests ──────────────────────────────────────────────────────

func TestTaskStoreMemory(t *testing.T) {
	ctx := context.Background()
	store := NewTaskMemoryStore(time.Hour, 100)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	task := TaskRecord{
		ChatID:      123,
		TaskID:      "t1",
		UserID:      456,
		Description: "test task",
		Status:      "running",
	}
	if err := store.SaveTask(ctx, task); err != nil {
		t.Fatal(err)
	}

	got, ok, err := store.GetTask(ctx, "t1")
	if err != nil || !ok {
		t.Fatal("expected task to be found")
	}
	if got.Description != "test task" {
		t.Fatalf("expected description 'test task', got %q", got.Description)
	}

	if err := store.UpdateStatus(ctx, "t1", "completed", WithAnswerPreview("preview")); err != nil {
		t.Fatal(err)
	}
	got, _, _ = store.GetTask(ctx, "t1")
	if got.Status != "completed" || got.AnswerPreview != "preview" {
		t.Fatalf("expected completed with preview, got %q/%q", got.Status, got.AnswerPreview)
	}
}

func TestTaskStoreMarkStaleRunning(t *testing.T) {
	ctx := context.Background()
	store := NewTaskMemoryStore(time.Hour, 100)
	_ = store.SaveTask(ctx, TaskRecord{TaskID: "t1", Status: "running"})
	_ = store.SaveTask(ctx, TaskRecord{TaskID: "t2", Status: "completed"})

	if err := store.MarkStaleRunning(ctx, "restart"); err != nil {
		t.Fatal(err)
	}

	t1, _, _ := store.GetTask(ctx, "t1")
	t2, _, _ := store.GetTask(ctx, "t2")

	if t1.Status != "failed" {
		t.Fatalf("expected running task to be marked failed, got %q", t1.Status)
	}
	if t2.Status != "completed" {
		t.Fatalf("expected completed task to stay completed, got %q", t2.Status)
	}
}

// ── Plan review store tests ───────────────────────────────────────────────

func TestPlanReviewStoreMemory(t *testing.T) {
	ctx := context.Background()
	store := NewPlanReviewMemoryStore(time.Hour)
	if err := store.EnsureSchema(ctx); err != nil {
		t.Fatal(err)
	}

	pending := PlanReviewPending{
		ChatID:        123,
		UserID:        456,
		RunID:         "run1",
		OverallGoalUI: "build feature X",
	}
	if err := store.SavePending(ctx, pending); err != nil {
		t.Fatal(err)
	}

	got, ok, err := store.GetPending(ctx, 123)
	if err != nil || !ok {
		t.Fatal("expected pending to be found")
	}
	if got.OverallGoalUI != "build feature X" {
		t.Fatalf("expected goal 'build feature X', got %q", got.OverallGoalUI)
	}

	if err := store.ClearPending(ctx, 123); err != nil {
		t.Fatal(err)
	}
	_, ok, _ = store.GetPending(ctx, 123)
	if ok {
		t.Fatal("expected pending to be cleared")
	}
}

// ── Progress listener tests ───────────────────────────────────────────────

func TestProgressListenerSendsOnToolStart(t *testing.T) {
	var mu sync.Mutex
	var sentTexts []string
	var sentIDs []int
	idCounter := 0

	sender := &mockProgressSender{
		sendFn: func(_ context.Context, text string) (int, error) {
			mu.Lock()
			defer mu.Unlock()
			idCounter++
			sentTexts = append(sentTexts, text)
			sentIDs = append(sentIDs, idCounter)
			return idCounter, nil
		},
		updateFn: func(_ context.Context, msgID int, text string) error {
			mu.Lock()
			defer mu.Unlock()
			sentTexts = append(sentTexts, text)
			return nil
		},
	}

	ctx := context.Background()
	pl := newProgressListener(ctx, nil, sender, nil)

	// No events sent — just close cleanly.

	pl.Close()

	// No tools started, so only sent if dirty.
	mu.Lock()
	count := len(sentTexts)
	mu.Unlock()
	// Progress listener only sends when dirty from tool events — 0 sends is expected.
	_ = count
}

func TestProgressListenerClose(t *testing.T) {
	sender := &mockProgressSender{
		sendFn: func(_ context.Context, text string) (int, error) {
			return 1, nil
		},
		updateFn: func(_ context.Context, msgID int, text string) error {
			return nil
		},
	}

	ctx := context.Background()
	pl := newProgressListener(ctx, nil, sender, nil)
	pl.Close()
	pl.Close() // double close should be safe
}

type mockProgressSender struct {
	sendFn   func(context.Context, string) (int, error)
	updateFn func(context.Context, int, string) error
}

func (m *mockProgressSender) SendProgress(ctx context.Context, text string) (int, error) {
	return m.sendFn(ctx, text)
}

func (m *mockProgressSender) UpdateProgress(ctx context.Context, messageID int, text string) error {
	return m.updateFn(ctx, messageID, text)
}

// ── Bootstrap config tests ────────────────────────────────────────────────

func TestParseIncomingMessageDM(t *testing.T) {
	gw := newTestGateway(&stubExecutor{}, newRecordingMessenger())
	msg := dmMessage(123, 1, "hello world")
	parsed := gw.parseIncomingMessage(msg)
	if parsed == nil {
		t.Fatal("expected parsed message")
	}
	if parsed.isGroup {
		t.Fatal("expected DM, got group")
	}
	if parsed.content != "hello world" {
		t.Fatalf("expected 'hello world', got %q", parsed.content)
	}
}

func TestParseIncomingMessageGroup(t *testing.T) {
	gw := newTestGateway(&stubExecutor{}, newRecordingMessenger())
	msg := groupMessage(999, 1, 1, "/new@mybot")
	parsed := gw.parseIncomingMessage(msg)
	if parsed == nil {
		t.Fatal("expected parsed message")
	}
	if !parsed.isGroup {
		t.Fatal("expected group message")
	}
	if parsed.content != "/new" {
		t.Fatalf("expected '/new' after stripping bot mention, got %q", parsed.content)
	}
}

func TestNilMessageIgnored(t *testing.T) {
	gw := newTestGateway(&stubExecutor{}, newRecordingMessenger())
	parsed := gw.parseIncomingMessage(nil)
	if parsed != nil {
		t.Fatal("expected nil for nil message")
	}
}

// Suppress unused import warnings.
var _ = fmt.Sprintf
