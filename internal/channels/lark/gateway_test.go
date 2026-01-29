package lark

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	ports "alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	storage "alex/internal/agent/ports/storage"
	"alex/internal/logging"

	lru "github.com/hashicorp/golang-lru/v2"
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
	cfg := Config{AppID: "cli_test", AppSecret: "secret", SessionPrefix: "custom"}
	gw, err := NewGateway(cfg, &stubExecutor{}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if gw.cfg.SessionPrefix != "custom" {
		t.Fatalf("expected custom session prefix, got %q", gw.cfg.SessionPrefix)
	}
}

func TestSessionIDForChatDeterministic(t *testing.T) {
	gw := &Gateway{cfg: Config{SessionPrefix: "lark"}, logger: logging.OrNop(nil)}
	first := gw.sessionIDForChat("oc_abc123")
	second := gw.sessionIDForChat("oc_abc123")
	if first != second {
		t.Fatalf("expected deterministic session id, got %q vs %q", first, second)
	}
	if !strings.HasPrefix(first, "lark-") {
		t.Fatalf("expected prefix 'lark-', got %q", first)
	}
}

func TestSessionIDForChatDistinct(t *testing.T) {
	gw := &Gateway{cfg: Config{SessionPrefix: "lark"}, logger: logging.OrNop(nil)}
	a := gw.sessionIDForChat("oc_chat_a")
	b := gw.sessionIDForChat("oc_chat_b")
	if a == b {
		t.Fatalf("expected different session ids for different chats, both got %q", a)
	}
}

func TestExtractText(t *testing.T) {
	gw := &Gateway{cfg: Config{SessionPrefix: "lark"}, logger: logging.OrNop(nil)}
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
	gw := &Gateway{cfg: Config{SessionPrefix: "lark"}, logger: logging.OrNop(nil)}

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
		gwPrefix := &Gateway{cfg: Config{SessionPrefix: "lark", ReplyPrefix: "[Bot] "}, logger: logging.OrNop(nil)}
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

func TestGatewayMessageDedup(t *testing.T) {
	now := time.Date(2026, 1, 29, 11, 0, 0, 0, time.UTC)
	cache, err := lru.New[string, time.Time](2)
	if err != nil {
		t.Fatalf("failed to create dedup cache: %v", err)
	}
	gw := &Gateway{
		cfg:        Config{SessionPrefix: "lark"},
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
