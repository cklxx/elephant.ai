package lark

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	appcontext "alex/internal/agent/app/context"
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

func TestNewGatewayDefaultsToolPreset(t *testing.T) {
	cfg := Config{AppID: "cli_test", AppSecret: "secret"}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw.cfg.ToolPreset != "full" {
		t.Fatalf("expected default tool preset 'full', got %q", gw.cfg.ToolPreset)
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

func TestExtractTextContent(t *testing.T) {
	tests := []struct {
		name     string
		raw      string
		expected string
	}{
		{"valid text", `{"text":"hello world"}`, "hello world"},
		{"empty text", `{"text":""}`, ""},
		{"whitespace text", `{"text":"  "}`, ""},
		{"empty raw", "", ""},
		{"invalid json", "not json", "not json"},
		{"no text field", `{"other":"value"}`, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractTextContent(tt.raw, nil)
			if got != tt.expected {
				t.Fatalf("extractTextContent(%q) = %q, want %q", tt.raw, got, tt.expected)
			}
		})
	}
}

func TestExtractTextContent_MentionPlaceholderFromEventMentions(t *testing.T) {
	raw := `{"text":"hi @_user_1"}`
	key := "@_user_1"
	name := "Bob"
	openID := "ou_123"
	mentions := []*larkim.MentionEvent{
		{
			Key:  &key,
			Name: &name,
			Id:   &larkim.UserId{OpenId: &openID},
		},
	}
	got := extractTextContent(raw, mentions)
	if !strings.Contains(got, "@Bob(ou_123)") {
		t.Fatalf("expected placeholder mention to resolve, got %q", got)
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

func TestTextContent_RendersOutgoingAtTag(t *testing.T) {
	got := textContent("hi @Bob(ou_123)")
	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal([]byte(got), &parsed); err != nil {
		t.Fatalf("failed to parse content json: %v", err)
	}
	if !strings.Contains(parsed.Text, `<at user_id="ou_123">Bob</at>`) {
		t.Fatalf("expected outgoing mention tag, got %q", parsed.Text)
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

func TestReplyTarget(t *testing.T) {
	tests := []struct {
		name      string
		messageID string
		allow     bool
		want      string
	}{
		{"allowed with message", "om_group", true, "om_group"},
		{"allowed empty message", "", true, ""},
		{"disallowed with message", "om_group", false, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := replyTarget(tt.messageID, tt.allow); got != tt.want {
				t.Fatalf("replyTarget(%q, %v) = %q, want %q", tt.messageID, tt.allow, got, tt.want)
			}
		})
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

func TestFilterNonA2UIAttachmentsDoesNotMutateInput(t *testing.T) {
	input := map[string]ports.Attachment{
		"ui.json": {
			Name:      "ui.json",
			MediaType: "application/a2ui+json",
			Format:    "a2ui",
		},
		"report.pdf": {
			Name:      "report.pdf",
			MediaType: "application/pdf",
		},
	}
	clone := ports.CloneAttachmentMap(input)

	filtered := filterNonA2UIAttachments(input)
	if len(filtered) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(filtered))
	}
	if _, ok := filtered["report.pdf"]; !ok {
		t.Fatalf("expected report.pdf attachment, got %#v", filtered)
	}
	if !reflect.DeepEqual(input, clone) {
		t.Fatalf("expected input attachments to remain unchanged")
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
	if !appcontext.SessionHistoryEnabled(executor.capturedCtx) {
		t.Fatalf("expected session history to be enabled for lark")
	}
}

func TestHandleMessageSessionHistoryEnabled(t *testing.T) {
	openID := "ou_sender_history"
	chatID := "oc_chat_history"
	msgID := "om_msg_history"
	content := `{"text":"hello history"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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

	if executor.capturedCtx == nil {
		t.Fatal("expected ExecuteTask to be called")
	}

	if !appcontext.SessionHistoryEnabled(executor.capturedCtx) {
		t.Fatalf("expected session history to be enabled for lark")
	}
}

func TestHandleMessagePostContent(t *testing.T) {
	openID := "ou_sender_post"
	chatID := "oc_chat_post"
	msgID := "om_msg_post"
	content := `{"title":"Weekly Update","content":[[{"tag":"text","text":"Line 1 "},{"tag":"at","user_name":"alex"}],[{"tag":"text","text":"Line 2"}]]}`
	msgType := "post"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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

	if executor.capturedTask == "" {
		t.Fatal("expected post content to be extracted")
	}
	if !strings.Contains(executor.capturedTask, "Weekly Update") {
		t.Fatalf("expected title to be included, got %q", executor.capturedTask)
	}
	if !strings.Contains(executor.capturedTask, "Line 1 @alex") {
		t.Fatalf("expected text + mention to be included, got %q", executor.capturedTask)
	}
	if !strings.Contains(executor.capturedTask, "Line 2") {
		t.Fatalf("expected second line to be included, got %q", executor.capturedTask)
	}
}

func TestExtractTextContent_MentionRendersUserID(t *testing.T) {
	raw := `{"text":"Hi <at user_id=\"ou_123\">Bob</at>"}`
	got := extractTextContent(raw, nil)
	if !strings.Contains(got, "@Bob(ou_123)") {
		t.Fatalf("expected mention to include user id, got %q", got)
	}
}

func TestExtractPostContent_MentionRendersUserID(t *testing.T) {
	raw := `{"title":"t","content":[[{"tag":"text","text":"Hi "},{"tag":"at","user_id":"ou_123","user_name":"Bob"}]]}`
	got := extractPostContent(raw, nil)
	if !strings.Contains(got, "@Bob(ou_123)") {
		t.Fatalf("expected mention to include user id, got %q", got)
	}
}

func TestExtractPostContent_MentionPlaceholderFromEventMentions(t *testing.T) {
	raw := `{"title":"t","content":[[{"tag":"text","text":"Hi "},{"tag":"at","user_id":"@_user_1","user_name":""}]]}`
	key := "@_user_1"
	name := "Bob"
	openID := "ou_123"
	mentions := []*larkim.MentionEvent{
		{
			Key:  &key,
			Name: &name,
			Id:   &larkim.UserId{OpenId: &openID},
		},
	}

	got := extractPostContent(raw, mentions)
	if strings.Contains(got, "@@_user_1") {
		t.Fatalf("expected no double @ placeholder, got %q", got)
	}
	if strings.Contains(got, "@_user_1") {
		t.Fatalf("expected placeholder to be resolved, got %q", got)
	}
	if !strings.Contains(got, "@Bob(ou_123)") {
		t.Fatalf("expected mention to resolve, got %q", got)
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
	if !policy.Enabled || !policy.AutoRecall || !policy.AutoCapture || !policy.CaptureMessages || !policy.RefreshEnabled {
		t.Fatalf("expected memory policy enabled, got %+v", policy)
	}
}

func TestHandleMessageCreatesNewSessionPerMessage(t *testing.T) {
	openID := "ou_sender_stable"
	chatID := "oc_chat_stable"
	msgID := "om_msg_stable"
	msgID2 := "om_msg_stable_2"
	content := `{"text":"hello stable"}`
	content2 := `{"text":"hello stable again"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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

	firstSessionID := executor.capturedSessionID
	if firstSessionID == "" || !strings.HasPrefix(firstSessionID, "lark-") {
		t.Fatalf("expected lark session id, got %q", firstSessionID)
	}

	event2 := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID2,
				Content:     &content2,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}

	if err := gw.handleMessage(context.Background(), event2); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}

	secondSessionID := executor.capturedSessionID
	if secondSessionID == "" || !strings.HasPrefix(secondSessionID, "lark-") {
		t.Fatalf("expected lark session id, got %q", secondSessionID)
	}
	if secondSessionID == firstSessionID {
		t.Fatalf("expected new session id, got same id %q", secondSessionID)
	}
	if got := id.SessionIDFromContext(executor.capturedCtx); got != secondSessionID {
		t.Fatalf("expected context sessionID %q, got %q", secondSessionID, got)
	}
}

func TestHandleMessageReusesAwaitingSession(t *testing.T) {
	openID := "ou_sender_await"
	chatID := "oc_chat_await"
	msgID := "om_msg_await"
	msgID2 := "om_msg_await_2"
	content := `{"text":"first"}`
	content2 := `{"text":"second"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{
		result: &agent.TaskResult{StopReason: "await_user_input"},
	}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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
	firstSessionID := executor.capturedSessionID
	if firstSessionID == "" {
		t.Fatal("expected first session id")
	}

	executor.result = &agent.TaskResult{Answer: "done"}
	event2 := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID2,
				Content:     &content2,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}
	if err := gw.handleMessage(context.Background(), event2); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	secondSessionID := executor.capturedSessionID
	if secondSessionID != firstSessionID {
		t.Fatalf("expected awaiting session reuse (%q), got %q", firstSessionID, secondSessionID)
	}
}

func TestHandleMessageReusesInFlightSession(t *testing.T) {
	openID := "ou_sender_inflight"
	chatID := "oc_chat_inflight"
	msgID := "om_msg_inflight"
	msgID2 := "om_msg_inflight_2"
	content := `{"text":"first"}`
	content2 := `{"text":"second"}`
	msgType := "text"
	chatType := "p2p"

	executor := &blockingExecutor{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := gw.handleMessage(context.Background(), event); err != nil {
			t.Errorf("handleMessage failed: %v", err)
		}
	}()

	<-executor.started

	event2 := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID2,
				Content:     &content2,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}
	if err := gw.handleMessage(context.Background(), event2); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	executor.mu.Lock()
	callCount := executor.callCount
	executor.mu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected ExecuteTask once, got %d", callCount)
	}

	close(executor.finish)
	wg.Wait()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		executor.mu.Lock()
		callCount := executor.callCount
		executor.mu.Unlock()
		if callCount >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected ExecuteTask to be called again for reprocessed message, got %d", callCount)
		case <-ticker.C:
		}
	}
}

func TestHandleMessageReprocessPreservesGroupChatType(t *testing.T) {
	openID := "ou_sender_inflight_group"
	chatID := "oc_chat_inflight_group"
	msgID := "om_msg_inflight_group"
	msgID2 := "om_msg_inflight_group_2"
	content := `{"text":"first"}`
	content2 := `{"text":"second"}`
	msgType := "text"
	chatType := "group"

	executor := &blockingExecutor{
		started: make(chan struct{}),
		finish:  make(chan struct{}),
	}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true, AllowDirect: false}, AppID: "test", AppSecret: "secret"},
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

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := gw.handleMessage(context.Background(), event); err != nil {
			t.Errorf("handleMessage failed: %v", err)
		}
	}()

	<-executor.started

	event2 := &larkim.P2MessageReceiveV1{
		Event: &larkim.P2MessageReceiveV1Data{
			Message: &larkim.EventMessage{
				MessageType: &msgType,
				ChatType:    &chatType,
				ChatId:      &chatID,
				MessageId:   &msgID2,
				Content:     &content2,
			},
			Sender: &larkim.EventSender{
				SenderId: &larkim.UserId{
					OpenId: &openID,
				},
			},
		},
	}
	if err := gw.handleMessage(context.Background(), event2); err != nil {
		t.Fatalf("handleMessage failed: %v", err)
	}
	executor.mu.Lock()
	callCount := executor.callCount
	executor.mu.Unlock()
	if callCount != 1 {
		t.Fatalf("expected ExecuteTask once, got %d", callCount)
	}

	close(executor.finish)
	wg.Wait()

	deadline := time.After(2 * time.Second)
	ticker := time.NewTicker(10 * time.Millisecond)
	defer ticker.Stop()
	for {
		executor.mu.Lock()
		callCount := executor.callCount
		executor.mu.Unlock()
		if callCount >= 2 {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("expected ExecuteTask to be called again for reprocessed group message, got %d", callCount)
		case <-ticker.C:
		}
	}
}

func TestHandleMessageDefaultsToolPresetFull(t *testing.T) {
	openID := "ou_sender_preset"
	chatID := "oc_chat_preset"
	msgID := "om_msg_preset"
	content := `{"text":"hello preset"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	gw, err := NewGateway(Config{
		BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true},
		AppID:      "test",
		AppSecret:  "secret",
	}, executor, logging.OrNop(nil))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

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

	presetRaw := executor.capturedCtx.Value(appcontext.PresetContextKey{})
	preset, ok := presetRaw.(appcontext.PresetConfig)
	if !ok {
		t.Fatal("expected preset config on context")
	}
	if preset.ToolPreset != "full" {
		t.Fatalf("expected tool preset 'full', got %q", preset.ToolPreset)
	}
}

func TestHandleMessageInjectsPlanFeedback(t *testing.T) {
	openID := "ou_sender_plan"
	chatID := "oc_chat_plan"
	msgID := "om_msg_plan"
	content := `{"text":"请加一步验收"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{}
	store := &stubPlanReviewStore{
		has: true,
		pending: PlanReviewPending{
			UserID:        openID,
			ChatID:        chatID,
			RunID:         "run-1",
			OverallGoalUI: "ship feature",
			InternalPlan:  map[string]any{"steps": []any{"a", "b"}},
		},
	}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret", PlanReviewEnabled: true},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}
	gw.SetPlanReviewStore(store)
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
	if executor.capturedTask == "" {
		t.Fatal("expected ExecuteTask to be called")
	}
	if !strings.Contains(executor.capturedTask, "<plan_feedback>") {
		t.Fatalf("expected plan feedback block, got %q", executor.capturedTask)
	}
	if !store.cleared {
		t.Fatal("expected pending store to be cleared")
	}
}

func TestHandleMessageSavesPlanReviewPendingOnAwaitUserInput(t *testing.T) {
	openID := "ou_sender_pending"
	chatID := "oc_chat_pending"
	msgID := "om_msg_pending"
	content := `{"text":"继续"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{
		result: &agent.TaskResult{
			StopReason: "await_user_input",
			Messages: []ports.Message{{
				Role:    "system",
				Content: "<plan_review_pending>\nrun_id: run-9\noverall_goal_ui: goal-9\ninternal_plan: {\"steps\":[\"x\"]}\n</plan_review_pending>",
			}},
		},
	}
	store := &stubPlanReviewStore{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret", PlanReviewEnabled: true},
		agent:  executor,
		logger: logging.OrNop(nil),
		now:    func() time.Time { return time.Now() },
	}
	gw.SetPlanReviewStore(store)
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
	if len(store.saved) != 1 {
		t.Fatalf("expected pending save, got %d", len(store.saved))
	}
	if store.saved[0].RunID != "run-9" || store.saved[0].OverallGoalUI != "goal-9" {
		t.Fatalf("unexpected pending record: %+v", store.saved[0])
	}
}

func TestHandleMessageAwaitUserInputRepliesWithQuestion(t *testing.T) {
	openID := "ou_sender_question"
	chatID := "oc_chat_question"
	msgID := "om_msg_question"
	content := `{"text":"继续"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{
		result: &agent.TaskResult{
			StopReason: "await_user_input",
			Messages: []ports.Message{{
				Role: "tool",
				ToolResults: []ports.ToolResult{{
					CallID:  "call-1",
					Content: "goal\nWhich env?",
					Metadata: map[string]any{
						"needs_user_input": true,
						"question_to_user": "Which env?",
					},
				}},
			}},
		},
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg:       Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
		now:       func() time.Time { return time.Now() },
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

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		calls = recorder.CallsByMethod("SendMessage")
	}
	if len(calls) == 0 {
		t.Fatal("expected a reply message")
	}
	if got := extractTextContent(calls[0].Content, nil); got != "Which env?" {
		t.Fatalf("expected question reply, got %q", got)
	}
}

func TestHandleMessageSeedsPendingUserInput(t *testing.T) {
	openID := "ou_sender_pending_input"
	chatID := "oc_chat_pending_input"
	msgID := "om_msg_pending_input"
	content := `{"text":"next step"}`
	msgType := "text"
	chatType := "p2p"

	executor := &awaitInputExecutor{}
	gw := &Gateway{
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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

	if executor.capturedTask != "" {
		t.Fatalf("expected empty task when pending user input, got %q", executor.capturedTask)
	}
	if executor.capturedInput == nil {
		t.Fatal("expected pending user input to be seeded")
	}
	if executor.capturedInput.Content != "next step" {
		t.Fatalf("expected pending input content, got %q", executor.capturedInput.Content)
	}
}

func TestHandleMessageSendsPlanReviewCardWhenEnabled(t *testing.T) {
	openID := "ou_sender_card"
	chatID := "oc_chat_card"
	msgID := "om_msg_card"
	content := `{"text":"继续"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{
		result: &agent.TaskResult{
			StopReason: "await_user_input",
			Messages: []ports.Message{{
				Role:    "system",
				Content: "<plan_review_pending>\nrun_id: run-9\noverall_goal_ui: goal-9\ninternal_plan: {\"steps\":[\"x\"]}\n</plan_review_pending>",
			}},
		},
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig:        channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true},
			AppID:             "test",
			AppSecret:         "secret",
			PlanReviewEnabled: true,
			CardsEnabled:      true,
			CardsPlanReview:   true,
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
		now:       func() time.Time { return time.Now() },
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

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		calls = recorder.CallsByMethod("SendMessage")
	}
	if len(calls) == 0 {
		t.Fatal("expected a reply message")
	}
	if calls[0].MsgType != "interactive" {
		t.Fatalf("expected interactive card reply, got %q", calls[0].MsgType)
	}
}

func TestHandleMessageSendsResultCardWhenEnabled(t *testing.T) {
	openID := "ou_sender_result"
	chatID := "oc_chat_result"
	msgID := "om_msg_result"
	content := `{"text":"hello"}`
	msgType := "text"
	chatType := "p2p"

	executor := &capturingExecutor{
		result: &agent.TaskResult{
			Answer: "done",
		},
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig:   channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true},
			AppID:        "test",
			AppSecret:    "secret",
			CardsEnabled: true,
			CardsResults: true,
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
		now:       func() time.Time { return time.Now() },
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

	calls := recorder.CallsByMethod("ReplyMessage")
	if len(calls) == 0 {
		calls = recorder.CallsByMethod("SendMessage")
	}
	if len(calls) == 0 {
		t.Fatal("expected a reply message")
	}
	if calls[0].MsgType != "interactive" {
		t.Fatalf("expected interactive card reply, got %q", calls[0].MsgType)
	}
}

func TestHandleMessageSendsAttachmentCardWhenAttachmentsPresent(t *testing.T) {
	openID := "ou_sender_attach"
	chatID := "oc_chat_attach"
	msgID := "om_msg_attach"
	content := `{"text":"hello"}`
	msgType := "text"
	chatType := "p2p"

	imagePayload := base64.StdEncoding.EncodeToString([]byte("fake-image"))
	filePayload := base64.StdEncoding.EncodeToString([]byte("fake-file"))

	executor := &capturingExecutor{
		result: &agent.TaskResult{
			Answer: "done",
			Attachments: map[string]ports.Attachment{
				"photo.png":  {Name: "photo.png", MediaType: "image/png", Data: imagePayload},
				"report.pdf": {Name: "report.pdf", MediaType: "application/pdf", Data: filePayload},
			},
		},
	}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig:      channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true},
			AppID:           "test",
			AppSecret:       "secret",
			CardsEnabled:    true,
			CardsResults:    true,
			AutoUploadFiles: true,
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
		now:       func() time.Time { return time.Now() },
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

	msgCalls := 0
	for _, call := range recorder.Calls() {
		if call.Method != "ReplyMessage" && call.Method != "SendMessage" {
			continue
		}
		msgCalls++
		if call.MsgType != "interactive" {
			t.Fatalf("expected interactive card reply, got %q", call.MsgType)
		}
	}
	if msgCalls != 1 {
		t.Fatalf("expected 1 reply message, got %d", msgCalls)
	}
	for _, call := range recorder.Calls() {
		if call.MsgType == "image" || call.MsgType == "file" {
			t.Fatalf("expected no image/file message dispatches, got %q", call.MsgType)
		}
	}
	if len(recorder.CallsByMethod("UploadImage")) != 1 {
		t.Fatalf("expected image upload, got %#v", recorder.CallsByMethod("UploadImage"))
	}
	if len(recorder.CallsByMethod("UploadFile")) != 1 {
		t.Fatalf("expected file upload, got %#v", recorder.CallsByMethod("UploadFile"))
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
		cfg:    Config{BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowDirect: true}, AppID: "test", AppSecret: "secret"},
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

type awaitInputExecutor struct {
	capturedTask  string
	capturedInput *agent.UserInput
}

func (a *awaitInputExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{
		ID:       sessionID,
		Metadata: map[string]string{"await_user_input": "true"},
	}, nil
}

func (a *awaitInputExecutor) ExecuteTask(ctx context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	a.capturedTask = task
	if ch := agent.UserInputChFromContext(ctx); ch != nil {
		select {
		case input := <-ch:
			a.capturedInput = &input
		default:
		}
	}
	return &agent.TaskResult{Answer: "ok"}, nil
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

type blockingExecutor struct {
	mu          sync.Mutex
	started     chan struct{}
	finish      chan struct{}
	startedOnce sync.Once
	inputCh     <-chan agent.UserInput
	sessionID   string
	callCount   int
}

func (b *blockingExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	if sessionID == "" {
		sessionID = "lark-session"
	}
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (b *blockingExecutor) ExecuteTask(ctx context.Context, _ string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	b.mu.Lock()
	b.callCount++
	b.sessionID = sessionID
	b.inputCh = agent.UserInputChFromContext(ctx)
	b.mu.Unlock()
	b.startedOnce.Do(func() {
		close(b.started)
	})
	<-b.finish
	return &agent.TaskResult{Answer: "done"}, nil
}

type stubPlanReviewStore struct {
	pending PlanReviewPending
	has     bool
	loaded  int
	cleared bool
	saved   []PlanReviewPending
	err     error
}

func (s *stubPlanReviewStore) EnsureSchema(_ context.Context) error { return nil }

func (s *stubPlanReviewStore) SavePending(_ context.Context, pending PlanReviewPending) error {
	s.saved = append(s.saved, pending)
	return s.err
}

func (s *stubPlanReviewStore) GetPending(_ context.Context, _, _ string) (PlanReviewPending, bool, error) {
	s.loaded++
	if s.err != nil {
		return PlanReviewPending{}, false, s.err
	}
	return s.pending, s.has, nil
}

func (s *stubPlanReviewStore) ClearPending(_ context.Context, _, _ string) error {
	s.cleared = true
	return s.err
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

func TestAutoChatContextSizeConfig(t *testing.T) {
	cfg := Config{
		AppID:               "cli_test",
		AppSecret:           "secret",
		AutoChatContextSize: 30,
	}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw.cfg.AutoChatContextSize != 30 {
		t.Fatalf("expected AutoChatContextSize 30, got %d", gw.cfg.AutoChatContextSize)
	}
}

func TestBuildAttachmentSummaryNil(t *testing.T) {
	if s := buildAttachmentSummary(nil); s != "" {
		t.Fatalf("expected empty for nil result, got %q", s)
	}
}

func TestBuildAttachmentSummaryEmpty(t *testing.T) {
	result := &agent.TaskResult{}
	if s := buildAttachmentSummary(result); s != "" {
		t.Fatalf("expected empty for no attachments, got %q", s)
	}
}

func TestBuildAttachmentSummaryWithURIs(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"report.pdf": {
				Name: "report.pdf",
				URI:  "https://cdn.example.com/report.pdf",
			},
			"chart.png": {
				Name: "chart.png",
				URI:  "https://cdn.example.com/chart.png",
			},
		},
	}
	summary := buildAttachmentSummary(result)
	if summary == "" {
		t.Fatal("expected non-empty summary")
	}
	if !strings.Contains(summary, "chart.png: https://cdn.example.com/chart.png") {
		t.Fatalf("expected chart.png URI in summary, got %q", summary)
	}
	if !strings.Contains(summary, "report.pdf: https://cdn.example.com/report.pdf") {
		t.Fatalf("expected report.pdf URI in summary, got %q", summary)
	}
	if !strings.Contains(summary, "[Attachments]") {
		t.Fatalf("expected [Attachments] header, got %q", summary)
	}
}

func TestBuildAttachmentSummaryFiltersA2UI(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"report.pdf": {
				Name: "report.pdf",
				URI:  "https://cdn.example.com/report.pdf",
			},
			"ui-widget": {
				Name:   "ui-widget",
				Format: "a2ui",
			},
		},
	}
	summary := buildAttachmentSummary(result)
	if strings.Contains(summary, "ui-widget") {
		t.Fatalf("expected a2ui attachment to be filtered, got %q", summary)
	}
	if !strings.Contains(summary, "report.pdf") {
		t.Fatalf("expected report.pdf in summary, got %q", summary)
	}
}

func TestBuildAttachmentSummaryNoURIFallback(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"draft.txt": {
				Name: "draft.txt",
				Data: "aGVsbG8=",
			},
		},
	}
	summary := buildAttachmentSummary(result)
	if !strings.Contains(summary, "- draft.txt") {
		t.Fatalf("expected name-only entry, got %q", summary)
	}
	if strings.Contains(summary, "http") {
		t.Fatalf("expected no URL for data-only attachment, got %q", summary)
	}
}

func TestBuildAttachmentSummarySkipsDataURI(t *testing.T) {
	result := &agent.TaskResult{
		Attachments: map[string]ports.Attachment{
			"inline.png": {
				Name: "inline.png",
				URI:  "data:image/png;base64,abc",
			},
		},
	}
	summary := buildAttachmentSummary(result)
	if strings.Contains(summary, "data:") {
		t.Fatalf("expected data: URI to be omitted, got %q", summary)
	}
	if !strings.Contains(summary, "- inline.png") {
		t.Fatalf("expected name-only entry, got %q", summary)
	}
}
