package lark

import (
	"context"
	"strings"
	"sync"
	"testing"

	"alex/internal/delivery/channels"
	agent "alex/internal/domain/agent/ports/agent"
	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/shared/logging"
)

// promptRecordingExecutor captures prompts passed to ExecuteTask.
type promptRecordingExecutor struct {
	mu      sync.Mutex
	prompts []string
}

func (e *promptRecordingExecutor) EnsureSession(_ context.Context, sessionID string) (*storage.Session, error) {
	return &storage.Session{ID: sessionID, Metadata: map[string]string{}}, nil
}

func (e *promptRecordingExecutor) ExecuteTask(_ context.Context, task string, _ string, _ agent.EventListener) (*agent.TaskResult, error) {
	e.mu.Lock()
	e.prompts = append(e.prompts, task)
	e.mu.Unlock()
	return &agent.TaskResult{Answer: "Claude Code hooks 配置完成。"}, nil
}

func TestRunCCHooksSetup(t *testing.T) {
	executor := &promptRecordingExecutor{}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true},
			AppID:      "test",
			AppSecret:  "secret",
			CCHooksAutoConfig: &CCHooksAutoConfig{
				ServerURL: "http://localhost:8080",
				Token:     "tok123",
			},
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
	}

	msg := &incomingMessage{
		chatID:    "oc_hooks_chat",
		messageID: "om_hooks_msg",
		senderID:  "ou_hooks_sender",
		content:   "/notice",
		isGroup:   true,
	}

	gw.runCCHooksSetup(msg)

	executor.mu.Lock()
	prompts := append([]string{}, executor.prompts...)
	executor.mu.Unlock()

	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	prompt := prompts[0]
	if !strings.Contains(prompt, "cc-hooks-setup") {
		t.Errorf("expected prompt to mention cc-hooks-setup, got: %s", prompt)
	}
	if !strings.Contains(prompt, "http://localhost:8080") {
		t.Errorf("expected prompt to contain server_url, got: %s", prompt)
	}
	if !strings.Contains(prompt, "tok123") {
		t.Errorf("expected prompt to contain token, got: %s", prompt)
	}

	// Verify reply was sent to chat.
	calls := recorder.CallsByMethod("SendMessage")
	if len(calls) == 0 {
		t.Fatal("expected a reply message to be sent")
	}
	replyText := extractTextContent(calls[0].Content, nil)
	if !strings.Contains(replyText, "配置完成") {
		t.Errorf("unexpected reply: %q", replyText)
	}
}

func TestRunCCHooksSetupNoToken(t *testing.T) {
	executor := &promptRecordingExecutor{}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true},
			AppID:      "test",
			AppSecret:  "secret",
			CCHooksAutoConfig: &CCHooksAutoConfig{
				ServerURL: "http://localhost:9090",
			},
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
	}

	msg := &incomingMessage{
		chatID:    "oc_hooks_chat2",
		messageID: "om_hooks_msg2",
		senderID:  "ou_hooks_sender2",
		content:   "/notice",
		isGroup:   true,
	}

	gw.runCCHooksSetup(msg)

	executor.mu.Lock()
	prompts := append([]string{}, executor.prompts...)
	executor.mu.Unlock()

	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if strings.Contains(prompts[0], "token") {
		t.Errorf("expected no token in prompt args, got: %s", prompts[0])
	}
}

func TestRunCCHooksRemove(t *testing.T) {
	executor := &promptRecordingExecutor{}
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{SessionPrefix: "lark", AllowGroups: true},
			AppID:      "test",
			AppSecret:  "secret",
			CCHooksAutoConfig: &CCHooksAutoConfig{
				ServerURL: "http://localhost:8080",
			},
		},
		agent:     executor,
		logger:    logging.OrNop(nil),
		messenger: recorder,
	}

	msg := &incomingMessage{
		chatID:    "oc_hooks_chat3",
		messageID: "om_hooks_msg3",
		senderID:  "ou_hooks_sender3",
		content:   "/notice off",
		isGroup:   true,
	}

	gw.runCCHooksRemove(msg)

	executor.mu.Lock()
	prompts := append([]string{}, executor.prompts...)
	executor.mu.Unlock()

	if len(prompts) != 1 {
		t.Fatalf("expected 1 prompt, got %d", len(prompts))
	}
	if !strings.Contains(prompts[0], "remove") {
		t.Errorf("expected prompt to contain remove action, got: %s", prompts[0])
	}
}

func TestRunCCHooksSetupNilConfig(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{
		cfg: Config{
			BaseConfig: channels.BaseConfig{SessionPrefix: "lark"},
			AppID:      "test",
			AppSecret:  "secret",
		},
		logger:    logging.OrNop(nil),
		messenger: recorder,
	}

	msg := &incomingMessage{
		chatID:    "oc_chat",
		messageID: "om_msg",
		senderID:  "ou_sender",
		isGroup:   true,
	}

	// Should be a no-op.
	gw.runCCHooksSetup(msg)

	calls := recorder.CallsByMethod("SendMessage")
	if len(calls) != 0 {
		t.Fatalf("expected no messages when CCHooksAutoConfig is nil, got %d", len(calls))
	}
}

func TestCCHooksSetupArgs(t *testing.T) {
	tests := []struct {
		serverURL, token string
		wantContains     []string
		wantNotContains  []string
	}{
		{
			serverURL:    "http://localhost:8080",
			token:        "tok",
			wantContains: []string{`"action":"setup"`, `"server_url":"http://localhost:8080"`, `"token":"tok"`},
		},
		{
			serverURL:       "http://example.com",
			token:           "",
			wantContains:    []string{`"action":"setup"`, `"server_url":"http://example.com"`},
			wantNotContains: []string{`"token"`},
		},
	}

	for _, tt := range tests {
		result := ccHooksSetupArgs(tt.serverURL, tt.token)
		for _, want := range tt.wantContains {
			if !strings.Contains(result, want) {
				t.Errorf("ccHooksSetupArgs(%q, %q) = %q, expected to contain %q", tt.serverURL, tt.token, result, want)
			}
		}
		for _, notWant := range tt.wantNotContains {
			if strings.Contains(result, notWant) {
				t.Errorf("ccHooksSetupArgs(%q, %q) = %q, expected NOT to contain %q", tt.serverURL, tt.token, result, notWant)
			}
		}
	}
}
