package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	appcontext "alex/internal/agent/app/context"
	"alex/internal/agent/domain"
	ports "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/channels"
	"alex/internal/logging"
	id "alex/internal/utils/id"

	lru "github.com/hashicorp/golang-lru/v2"
	larkim "github.com/larksuite/oapi-sdk-go/v3/service/im/v1"
)

func TestNewGatewayRequiresAgent(t *testing.T) {
	cfg := Config{AppID: "cli_test", AppSecret: "secret"}
	_, err := NewGateway(cfg, nil, nil)
	if err == nil {
		t.Fatal("expected error for nil agent")
	}
}

func TestNewGatewayRequiresCredentials(t *testing.T) {
	cfg := Config{}
	_, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err == nil {
		t.Fatal("expected error for empty credentials")
	}
}

func TestNewGatewayDefaultsSessionPrefix(t *testing.T) {
	cfg := Config{AppID: "cli_test", AppSecret: "secret"}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw.cfg.SessionPrefix != "lark" {
		t.Fatalf("expected default session prefix 'lark', got %q", gw.cfg.SessionPrefix)
	}
}

func TestNewGatewayPreservesCustomPrefix(t *testing.T) {
	cfg := Config{BaseConfig: channels.BaseConfig{SessionPrefix: "custom"}, AppID: "cli_test", AppSecret: "secret"}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw.cfg.SessionPrefix != "custom" {
		t.Fatalf("expected custom session prefix, got %q", gw.cfg.SessionPrefix)
	}
}

func TestMemoryIDForChatDeterministic(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}
	first := gw.memoryIDForChat("oc_abc123")
	second := gw.memoryIDForChat("oc_abc123")
	if first != second {
		t.Fatalf("expected deterministic memory id, got %q vs %q", first, second)
	}
	if !strings.HasPrefix(first, "lark-") {
		t.Fatalf("expected prefix 'lark-', got %q", first)
	}
}

func TestMemoryIDForChatDistinct(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}
	a := gw.memoryIDForChat("oc_chat_a")
	b := gw.memoryIDForChat("oc_chat_b")
	if a == b {
		t.Fatalf("expected different memory ids for different chats, both got %q", a)
	}
}

func TestExtractText(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}
	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{"valid text", `{"text":"hello world"}`, "hello world"},
		{"empty text", `{"text":""}`, ""},
		{"whitespace text", `{"text":"  "}`, ""},
		{"empty raw", "", ""},
		{"invalid json", "not json", ""},
		{"no text field", `{"other":"value"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := gw.extractText(tt.raw)
			if got != tt.expected {
				t.Fatalf("extractText(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}

func TestBuildReply(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}

	t.Run("with error", func(t *testing.T) {
		reply := gw.buildReply(nil, errTest)
		if !strings.Contains(reply, "执行失败") {
			t.Fatalf("expected error reply, got %q", reply)
		}
	})

	t.Run("with empty answer", func(t *testing.T) {
		reply := gw.buildReply(nil, nil)
		if reply != "" {
			t.Fatalf("expected empty reply, got %q", reply)
		}
	})

	t.Run("with prefix", func(t *testing.T) {
		gwPrefix := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", ReplyPrefix: "[Bot] "}}, logger: logging.OrNop(nil)}
		result := &agent.TaskResult{Answer: "hello"}
		reply := gwPrefix.buildReply(result, nil)
		if !strings.HasPrefix(reply, "[Bot] ") {
			t.Fatalf("expected prefixed reply, got %q", reply)
		}
	})
}

func TestTextContent(t *testing.T) {
	got := textContent("hello")
	if !strings.Contains(got, `"text":"hello"`) {
		t.Fatalf("expected json text content, got %q", got)
	}
}

func TestAttachmentContentPayloads(t *testing.T) {
	image := imageContent("img_123")
	if !strings.Contains(image, `"image_key":"img_123"`) {
		t.Fatalf("expected image payload, got %q", image)
	}
	file := fileContent("file_456")
	if !strings.Contains(file, `"file_key":"file_456"`) {
		t.Fatalf("expected file payload, got %q", file)
	}
}

func TestCollectAttachmentsFromResult(t *testing.T) {
	result := &agent.TaskResult{
		Messages: []ports.Message{
			{
				Attachments: map[string]ports.Attachment{
					"": {Name: "photo.png", MediaType: "image/png"},
				},
			},
			{
				ToolResults: []ports.ToolResult{
					{Attachments: map[string]ports.Attachment{
						"report.pdf": {Name: "report.pdf", MediaType: "application/pdf"},
					}},
				},
			},
			{
				Attachments: map[string]ports.Attachment{
					"photo.png": {Name: "photo.png", MediaType: "image/png"},
				},
			},
		},
	}

	attachments := collectAttachmentsFromResult(result)
	if len(attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(attachments))
	}
	if _, ok := attachments["photo.png"]; !ok {
		t.Fatalf("expected photo.png attachment, got %#v", attachments)
	}
	if _, ok := attachments["report.pdf"]; !ok {
		t.Fatalf("expected report.pdf attachment, got %#v", attachments)
	}
}

func TestIsImageAttachment(t *testing.T) {
	if !isImageAttachment(ports.Attachment{}, "image/png", "file.bin") {
		t.Fatal("expected image by media type")
	}
	if !isImageAttachment(ports.Attachment{MediaType: "image/jpeg"}, "", "file.bin") {
		t.Fatal("expected image by attachment media type")
	}
	if !isImageAttachment(ports.Attachment{}, "", "photo.png") {
		t.Fatal("expected image by extension")
	}
	if isImageAttachment(ports.Attachment{}, "", "report.pdf") {
		t.Fatal("expected non-image for pdf")
	}
}

func TestFileNameAndTypeForAttachment(t *testing.T) {
	name := fileNameForAttachment(ports.Attachment{MediaType: "image/png"}, "image")
	if !strings.HasSuffix(name, ".png") {
		t.Fatalf("expected png extension, got %q", name)
	}
	if fileType := fileTypeForAttachment("video.mp4", ""); fileType != "mp4" {
		t.Fatalf("expected mp4 file type, got %q", fileType)
	}
	if fileType := fileTypeForAttachment("document", "application/pdf"); fileType != "pdf" {
		t.Fatalf("expected pdf file type, got %q", fileType)
	}
}

func TestLarkFileType(t *testing.T) {
	tests := []struct {
		ext  string
		want string
	}{
		{"pdf", "pdf"},
		{"mp4", "mp4"},
		{"doc", "doc"},
		{"xls", "xls"},
		{"ppt", "ppt"},
		{"opus", "opus"},
		{"stream", "stream"},
		{"md", "stream"},
		{"txt", "stream"},
		{"csv", "stream"},
		{"json", "stream"},
		{"bin", "stream"},
		{"PDF", "pdf"},
		{"", "stream"},
	}
	for _, tt := range tests {
		t.Run(tt.ext, func(t *testing.T) {
			got := larkFileType(tt.ext)
			if got != tt.want {
				t.Fatalf("larkFileType(%q) = %q, want %q", tt.ext, got, tt.want)
			}
		})
	}
}

func TestDeref(t *testing.T) {
	if deref(nil) != "" {
		t.Fatal("expected empty string for nil")
	}
	s := "hello"
	if deref(&s) != "hello" {
		t.Fatal("expected 'hello'")
	}
}

func TestStartReturnsNilWhenDisabled(t *testing.T) {
	gw := &Gateway{cfg: Config{Enabled: false}, logger: logging.OrNop(nil)}
	if err := gw.Start(context.Background()); err != nil {
		t.Fatalf("expected nil error for disabled gateway, got %v", err)
	}
}

func TestAddReactionSkipsWhenClientNil(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}, ReactEmoji: "SMILE"}, logger: logging.OrNop(nil)}
	// Should not panic when client is nil.
	gw.addReaction(context.Background(), "om_test_msg", "SMILE")
}

func TestAddReactionSkipsEmptyInputs(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}
	// Should not panic with empty messageID or emojiType.
	gw.addReaction(context.Background(), "", "SMILE")
	gw.addReaction(context.Background(), "om_test_msg", "")
}

func TestGatewayMessageDedup(t *testing.T) {
	now := time.Date(2026, 1, 29, 11, 0, 0, 0, time.UTC)
	cache, err := lru.New[string, time.Time](2)
	if err != nil {
		t.Fatalf("failed to create dedup cache: %v", err)
	}
	gw := &Gateway{
		cfg:        Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}},
		logger:     logging.OrNop(nil),
		dedupCache: cache,
		now: func() time.Time {
			return now
		},
	}

	if gw.isDuplicateMessage("msg-1") {
		t.Fatalf("expected first message not to be duplicate")
	}
	if !gw.isDuplicateMessage("msg-1") {
		t.Fatalf("expected second message to be duplicate")
	}

	now = now.Add(messageDedupTTL + time.Second)
	if gw.isDuplicateMessage("msg-1") {
		t.Fatalf("expected message to expire from dedupe window")
	}
}

func TestHandleMessageSetsUserIDOnContext(t *testing.T) {
	openID := "ou_sender_abc"
	chatID := "oc_chat_123"
	msgID := "om_msg_001"
	content := `{"text":"hello"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}
	// Initialize dedup cache.
	cache, _ := lru.New[string, time.Time](16)
	gw.dedupCache = cache

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID,
				Content:     &content,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if err := gw.handleMessage(context.Background(), event); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	if executor.capturedCtx == nil {
		t.Fatal("expected ExecuteTask to be called")
	}

	gotUserID := id.UserIDFromContext(executor.capturedCtx)
	if gotUserID != "ou_sender_abc" {
		t.Fatalf("expected user_id 'ou_sender_abc' on context, got %q", gotUserID)
	}
	if appcontext.SessionHistoryEnabled(executor.capturedCtx) {
		t.Fatalf("expected session history to be disabled for lark")
	}
}

func TestHandleMessageSetsMemoryPolicy(t *testing.T) {
	openID := "ou_sender_xyz"
	chatID := "oc_chat_456"
	msgID := "om_msg_002"
	content := `{"text":"remember this"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{
		result: &agent.TaskResult{Answer: "noted"},
	}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true, MemoryEnabled: true}, AppID: "test", AppSecret: "secret"},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}

	cache, _ := lru.New[string, time.Time](16)
	gw.dedupCache = cache

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID,
				Content:     &content,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if err := gw.handleMessage(context.Background(), event); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	policy, ok := appcontext.MemoryPolicyFromContext(executor.capturedCtx)
	if !ok {
		t.Fatal("expected memory policy on context")
	}
	if !policy.Enabled || !policy.AutoRecall || !policy.AutoCapture {
		t.Fatalf("expected memory policy enabled, got %+v", policy)
	}
}

func TestHandleMessageSessionModeStable(t *testing.T) {
	openID := "ou_sender_stable"
	chatID := "oc_chat_stable"
	msgID := "om_msg_stable"
	content := `{"text":"hello stable"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret", SessionMode: "stable"},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}
	cache, _ := lru.New[string, time.Time](16)
	gw.dedupCache = cache

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID,
				Content:     &content,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if err := gw.handleMessage(context.Background(), event); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	expectedSessionID := gw.memoryIDForChat(chatID)
	if executor.capturedSessionID != expectedSessionID {
		t.Fatalf("expected sessionID %q, got %q", expectedSessionID, executor.capturedSessionID)
	}
	if got := id.SessionIDFromContext(executor.capturedCtx); got != expectedSessionID {
		t.Fatalf("expected context sessionID %q, got %q", expectedSessionID, got)
	}
}

func TestHandleMessageSessionModeFresh(t *testing.T) {
	openID := "ou_sender_fresh"
	chatID := "oc_chat_fresh"
	msgID := "om_msg_fresh"
	content := `{"text":"hello fresh"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret", SessionMode: "fresh"},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}
	cache, _ := lru.New[string, time.Time](16)
	gw.dedupCache = cache

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID,
				Content:     &content,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if err := gw.handleMessage(context.Background(), event); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	memoryID := gw.memoryIDForChat(chatID)
	if executor.capturedSessionID == memoryID {
		t.Fatalf("expected fresh sessionID to differ from memoryID %q", memoryID)
	}
	if !strings.HasPrefix(executor.capturedSessionID, "lark-") {
		t.Fatalf("expected fresh sessionID to have lark- prefix, got %q", executor.capturedSessionID)
	}
	if got := id.SessionIDFromContext(executor.capturedCtx); got != executor.capturedSessionID {
		t.Fatalf("expected context sessionID %q, got %q", executor.capturedSessionID, got)
	}
}

func TestHandleMessageResetCommand(t *testing.T) {
	openID := "ou_sender_reset"
	chatID := "oc_chat_reset"
	msgID := "om_msg_reset"
	content := `{"text":"/reset"}`
	msgType := "text"
	chatType := "p2p"

	executor := &resetExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret", SessionMode: "stable"},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}
	cache, _ := lru.New[string, time.Time](16)
	gw.dedupCache = cache

	event := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID,
				Content:     &content,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if err := gw.handleMessage(context.Background(), event); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	if !executor.resetCalled {
		t.Fatal("expected ResetSession to be called")
	}
	if executor.executeCalled {
		t.Fatal("expected ExecuteTask to be skipped on /reset")
	}
	expectedSessionID := gw.memoryIDForChat(chatID)
	if executor.resetSessionID != expectedSessionID {
		t.Fatalf("expected reset sessionID %q, got %q", expectedSessionID, executor.resetSessionID)
	}
}

// --- test helpers ---

var errTest = fmt.Errorf("test error")

type stubExecutor struct{}

func (s *stubExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (s *stubExecutor) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	return nil, nil
}

// capturingExecutor records the context and returns a configurable result.
type capturingExecutor struct {
	capturedCtx       context.Context
	capturedSessionID string
	capturedTask      string
	result            *agent.TaskResult
}

func (c *capturingExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (c *capturingExecutor) ExecuteTask(ctx context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	c.capturedCtx = ctx
	c.capturedTask = task
	c.capturedSessionID = sessionID
	return c.result, nil
}

type resetExecutor struct {
	resetCalled    bool
	resetSessionID string
	executeCalled  bool
}

func (r *resetExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (r *resetExecutor) ExecuteTask(_ context.Context, _ string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	r.executeCalled = true
	return nil, nil
}

func (r *resetExecutor) ResetSession(_ context.Context, sessionID string) error {
	r.resetCalled = true
	r.resetSessionID = sessionID
	return nil
}

func TestBuildReplyThinkingFallback(t *testing.T) {
	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}

	t.Run("fallback to thinking when answer empty", func(t *testing.T) {
		result := &agent.TaskResult{
			Answer: "",
			Messages: []ports.Message{
				{Role: "user", Content: "hello"},
				{
					Role: "assistant",
					Thinking: ports.Thinking{
						Parts: []ports.ThinkingPart{
							{Kind: "reasoning", Text: "I should greet the user"},
						},
					},
				},
			},
		}
		reply := gw.buildReply(result, nil)
		if reply != "I should greet the user" {
			t.Fatalf("expected thinking fallback, got %q", reply)
		}
	})

	t.Run("no fallback when answer present", func(t *testing.T) {
		result := &agent.TaskResult{
			Answer: "Hello!",
			Messages: []ports.Message{
				{
					Role: "assistant",
					Thinking: ports.Thinking{
						Parts: []ports.ThinkingPart{
							{Kind: "reasoning", Text: "thinking content"},
						},
					},
				},
			},
		}
		reply := gw.buildReply(result, nil)
		if reply != "Hello!" {
			t.Fatalf("expected answer, got %q", reply)
		}
	})

	t.Run("empty when no thinking and no answer", func(t *testing.T) {
		result := &agent.TaskResult{
			Answer:   "",
			Messages: []ports.Message{{Role: "assistant"}},
		}
		reply := gw.buildReply(result, nil)
		if reply != "" {
			t.Fatalf("expected empty reply, got %q", reply)
		}
	})

	t.Run("last assistant message thinking used", func(t *testing.T) {
		result := &agent.TaskResult{
			Answer: "",
			Messages: []ports.Message{
				{
					Role: "assistant",
					Thinking: ports.Thinking{
						Parts: []ports.ThinkingPart{
							{Kind: "reasoning", Text: "first thought"},
						},
					},
				},
				{Role: "user", Content: "follow up"},
				{
					Role: "assistant",
					Thinking: ports.Thinking{
						Parts: []ports.ThinkingPart{
							{Kind: "reasoning", Text: "second thought"},
						},
					},
				},
			},
		}
		reply := gw.buildReply(result, nil)
		if reply != "second thought" {
			t.Fatalf("expected last assistant thinking, got %q", reply)
		}
	})
}

func TestExtractThinkingFallback(t *testing.T) {
	t.Run("nil messages", func(t *testing.T) {
		if got := extractThinkingFallback(nil); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("no assistant messages", func(t *testing.T) {
		msgs := []ports.Message{{Role: "user", Content: "hi"}}
		if got := extractThinkingFallback(msgs); got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("assistant with empty thinking", func(t *testing.T) {
		msgs := []ports.Message{{
			Role:     "assistant",
			Thinking: ports.Thinking{Parts: []ports.ThinkingPart{{Text: "  "}}},
		}}
		if got := extractThinkingFallback(msgs); got != "" {
			t.Fatalf("expected empty for whitespace-only thinking, got %q", got)
		}
	})
}

func TestEmojiReactionInterceptorDelegatesAndReactsOnce(t *testing.T) {
	delegate := &recordingGatewayListener{}

	gw := &Gateway{cfg: Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}}, logger: logging.OrNop(nil)}

	interceptor := &emojiReactionInterceptor{
		delegate:  delegate,
		gateway:   gw,
		messageID: "om_test_123",
		ctx:       context.Background(),
	}

	// First emoji event should trigger (but addReaction won't do anything since client is nil).
	emojiEvent := domain.NewWorkflowPreAnalysisEmojiEvent(
		agent.LevelCore, "sess", "run", "", "WAVE", time.Now(),
	)
	interceptor.OnEvent(emojiEvent)
	interceptor.OnEvent(emojiEvent) // Second call should not react again (sync.Once).

	// Non-emoji events should still be delegated.
	otherEvent := &stubAgentEvent{eventType: "workflow.node.started"}
	interceptor.OnEvent(otherEvent)

	events := delegate.EventTypes()
	if len(events) != 3 {
		t.Fatalf("expected 3 delegated events, got %d: %v", len(events), events)
	}
	if events[0] != "workflow.diagnostic.preanalysis_emoji" {
		t.Fatalf("expected first event to be emoji, got %q", events[0])
	}
	if events[2] != "workflow.node.started" {
		t.Fatalf("expected third event to be node.started, got %q", events[2])
	}
}

type recordingGatewayListener struct {
	mu     sync.Mutex
	events []agent.AgentEvent
}

func (l *recordingGatewayListener) OnEvent(event agent.AgentEvent) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.events = append(l.events, event)
}

func (l *recordingGatewayListener) EventTypes() []string {
	l.mu.Lock()
	defer l.mu.Unlock()
	types := make([]string, len(l.events))
	for i, e := range l.events {
		types[i] = e.EventType()
	}
	return types
}

func TestExtractSenderID(t *testing.T) {
	t.Run("nil event", func(t *testing.T) {
		got := extractSenderID(nil)
		if got != "" {
			t.Fatalf("expected empty, got %q", got)
		}
	})

	t.Run("nil sender", func(t *testing.T) {
		event := &larkim.P2MessageReceiveV1{}
		got := extractSenderID(event)
		if got != "" {
			t.Fatalf("expected empty for nil event body, got %q", got)
		}
	})

	t.Run("valid sender", func(t *testing.T) {
		openID := "ou_user123"
		event := &larkim.P2MessageReceiveV1{
			Event: &larkim.P2MessageReceiveV1Data{
				Sender: &larkim.EventSender{
					SenderId: &larkim.UserId{
						OpenId: &openID,
					},
				},
			},
		}
		got := extractSenderID(event)
		if got != "ou_user123" {
			t.Fatalf("expected 'ou_user123', got %q", got)
		}
	})
}

func TestAutoChatContextConfigDefaults(t *testing.T) {
	cfg := Config{
		AppID:     "cli_test",
		AppSecret: "secret",
	}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// AutoChatContext and AutoChatContextSize should be zero-value (not set by NewGateway).
	if gw.cfg.AutoChatContext {
		t.Fatal("expected AutoChatContext to default to false in Config")
	}
	if gw.cfg.AutoChatContextSize != 0 {
		t.Fatalf("expected AutoChatContextSize to default to 0, got %d", gw.cfg.AutoChatContextSize)
	}
}

func TestAutoChatContextConfigEnabled(t *testing.T) {
	cfg := Config{
		AppID:               "cli_test",
		AppSecret:           "secret",
		AutoChatContext:     true,
		AutoChatContextSize: 30,
	}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !gw.cfg.AutoChatContext {
		t.Fatal("expected AutoChatContext to be true")
	}
	if gw.cfg.AutoChatContextSize != 30 {
		t.Fatalf("expected AutoChatContextSize 30, got %d", gw.cfg.AutoChatContextSize)
	}
}

type stubAgentEvent struct {
	eventType string
}

func (e *stubAgentEvent) EventType() string               { return e.eventType }
func (e *stubAgentEvent) Timestamp() time.Time            { return time.Time{} }
func (e *stubAgentEvent) GetAgentLevel() agent.AgentLevel { return "" }
func (e *stubAgentEvent) GetSessionID() string            { return "" }
func (e *stubAgentEvent) GetRunID() string                { return "" }
func (e *stubAgentEvent) GetParentRunID() string          { return "" }
func (e *stubAgentEvent) GetCorrelationID() string        { return "" }
func (e *stubAgentEvent) GetCausationID() string          { return "" }
func (e *stubAgentEvent) GetEventID() string              { return "" }
func (e *stubAgentEvent) GetSeq() uint64                  { return 0 }

func TestEmojiReactionInterceptorFallbackWhenNoEvent(t *testing.T) {
	delegate := &recordingGatewayListener{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}, ReactEmoji: "SMILE"},
		logger: logging.OrNop(nil),
	}

	interceptor := &emojiReactionInterceptor{
		delegate:  delegate,
		gateway:   gw,
		messageID: "om_test_fallback",
		ctx:       context.Background(),
	}

	// Send a non-emoji event — interceptor should not fire.
	interceptor.OnEvent(&stubAgentEvent{eventType: "workflow.node.started"})
	if interceptor.fired {
		t.Fatal("expected interceptor not to have fired yet")
	}

	// Fallback should send the config emoji.
	interceptor.sendFallback()
	if !interceptor.fired {
		t.Fatal("expected interceptor to have fired after fallback")
	}

	// Calling fallback again should be idempotent (sync.Once).
	interceptor.sendFallback()
}

func TestEmojiReactionInterceptorNoFallbackAfterDynamic(t *testing.T) {
	delegate := &recordingGatewayListener{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark"}, ReactEmoji: "SMILE"},
		logger: logging.OrNop(nil),
	}

	interceptor := &emojiReactionInterceptor{
		delegate:  delegate,
		gateway:   gw,
		messageID: "om_test_no_fallback",
		ctx:       context.Background(),
	}

	// Dynamic emoji event fires first.
	emojiEvent := domain.NewWorkflowPreAnalysisEmojiEvent(
		agent.LevelCore, "sess", "run", "", "WAVE", time.Now(),
	)
	interceptor.OnEvent(emojiEvent)
	if !interceptor.fired {
		t.Fatal("expected interceptor to have fired on dynamic emoji")
	}

	// Fallback should be a no-op since dynamic already fired.
	interceptor.sendFallback() // should not panic or send again
}
