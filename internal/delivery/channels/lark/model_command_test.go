package lark

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func TestBuildModelListIncludesLlamaServerWhenAvailable(t *testing.T) {
	t.Parallel()

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/v1/models" {
			t.Fatalf("expected /v1/models path, got %q", got)
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"data":[{"id":"llama-3.2-local"}]}`))
	}))
	defer srv.Close()

	gw := &Gateway{
		logger: logging.OrNop(nil),
		cliCredsLoader: func() runtimeconfig.CLICredentials {
			return runtimeconfig.CLICredentials{}
		},
		llamaResolver: func(context.Context) (subscription.LlamaServerTarget, bool) {
			return subscription.LlamaServerTarget{
				BaseURL: srv.URL,
				Source:  "llama_server",
			}, true
		},
	}

	out := gw.buildModelList(context.Background(), &incomingMessage{chatID: "oc_test", senderID: "ou_test"})
	if !strings.Contains(out, "- llama_server (llama_server)") {
		t.Fatalf("expected llama_server provider in output, got:\n%s", out)
	}
	if !strings.Contains(out, "llama-3.2-local") {
		t.Fatalf("expected llama model in output, got:\n%s", out)
	}
}

func TestBuildModelListSkipsProvidersWithoutCredentials(t *testing.T) {
	t.Parallel()

	gw := &Gateway{
		logger: logging.OrNop(nil),
		cliCredsLoader: func() runtimeconfig.CLICredentials {
			return runtimeconfig.CLICredentials{}
		},
		llamaResolver: func(context.Context) (subscription.LlamaServerTarget, bool) {
			return subscription.LlamaServerTarget{}, false
		},
	}

	out := gw.buildModelList(context.Background(), &incomingMessage{chatID: "oc_test", senderID: "ou_test"})
	if !strings.Contains(out, "未发现可用的订阅模型") {
		t.Fatalf("expected no usable providers notice, got:\n%s", out)
	}
}

func TestResolveLlamaServerTarget(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		lookup  runtimeconfig.EnvLookup
		wantURL string
		wantSrc string
	}{
		{
			name: "base url from env",
			lookup: func(key string) (string, bool) {
				if key == "LLAMA_SERVER_BASE_URL" {
					return "http://127.0.0.1:8082", true
				}
				return "", false
			},
			wantURL: "http://127.0.0.1:8082",
			wantSrc: string(runtimeconfig.SourceEnv),
		},
		{
			name: "host from env without scheme",
			lookup: func(key string) (string, bool) {
				if key == "LLAMA_SERVER_HOST" {
					return "127.0.0.1:8082", true
				}
				return "", false
			},
			wantURL: "http://127.0.0.1:8082",
			wantSrc: string(runtimeconfig.SourceEnv),
		},
		{
			name: "host from env with scheme",
			lookup: func(key string) (string, bool) {
				if key == "LLAMA_SERVER_HOST" {
					return "https://llama.local:8082", true
				}
				return "", false
			},
			wantURL: "https://llama.local:8082",
			wantSrc: string(runtimeconfig.SourceEnv),
		},
		{
			name: "fallback source",
			lookup: func(string) (string, bool) {
				return "", false
			},
			wantURL: "",
			wantSrc: "llama_server",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got, ok := resolveLlamaServerTarget(tt.lookup)
			if !ok {
				t.Fatalf("expected resolver to return enabled")
			}
			if got.BaseURL != tt.wantURL {
				t.Fatalf("expected base url %q, got %q", tt.wantURL, got.BaseURL)
			}
			if got.Source != tt.wantSrc {
				t.Fatalf("expected source %q, got %q", tt.wantSrc, got.Source)
			}
		})
	}
}

func TestBuildModelListReplyReturnsText(t *testing.T) {
	t.Parallel()

	gw := &Gateway{
		logger: logging.OrNop(nil),
		cliCredsLoader: func() runtimeconfig.CLICredentials {
			return runtimeconfig.CLICredentials{
				Codex: runtimeconfig.CLICredential{
					Provider: "codex",
					APIKey:   "tok",
					BaseURL:  "https://chatgpt.com/backend-api/codex",
					Model:    "gpt-5.2-codex",
					Source:   runtimeconfig.SourceCodexCLI,
				},
			}
		},
		llamaResolver: func(context.Context) (subscription.LlamaServerTarget, bool) {
			return subscription.LlamaServerTarget{}, false
		},
	}

	msgType, content := gw.buildModelListReply(context.Background(), &incomingMessage{chatID: "oc_test", senderID: "ou_test"})
	if msgType != "text" {
		t.Fatalf("expected text fallback, got %q", msgType)
	}
	if !strings.Contains(content, "可用的订阅模型") {
		t.Fatalf("expected text model list payload, got:\n%s", content)
	}
}

// newTestGatewayWithStore creates a Gateway with a real SelectionStore for scope tests.
func newTestGatewayWithStore(t *testing.T) *Gateway {
	t.Helper()
	storePath := filepath.Join(t.TempDir(), "llm_selection.json")
	return &Gateway{
		logger:         logging.OrNop(nil),
		llmSelections:  subscription.NewSelectionStore(storePath),
		llmResolver:    subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} }),
		cliCredsLoader: func() runtimeconfig.CLICredentials { return runtimeconfig.CLICredentials{} },
	}
}

func TestApplyPinnedFallsBackToChannelLevel(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()

	// Set at channel level (global).
	sel := subscription.Selection{Mode: "cli", Provider: "llama_server", Model: "llama3:latest", Source: "llama_server"}
	if err := gw.llmSelections.Set(ctx, channelScope(), sel); err != nil {
		t.Fatalf("Set: %v", err)
	}

	// Apply from a different chat — should fall back to channel scope.
	msg := &incomingMessage{chatID: "oc_other", senderID: "ou_user"}
	newCtx := gw.applyPinnedLarkLLMSelection(ctx, msg)
	if newCtx == ctx {
		t.Fatal("expected context to be enriched with LLM selection")
	}
}

func TestApplyPinnedPrefersChatSpecific(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()

	// Set channel-level.
	channelSel := subscription.Selection{Mode: "cli", Provider: "llama_server", Model: "channel-model", Source: "llama_server"}
	if err := gw.llmSelections.Set(ctx, channelScope(), channelSel); err != nil {
		t.Fatalf("Set channel: %v", err)
	}
	// Set chat-specific.
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}
	chatSel := subscription.Selection{Mode: "cli", Provider: "llama_server", Model: "chat-model", Source: "llama_server"}
	if err := gw.llmSelections.Set(ctx, chatScope(msg), chatSel); err != nil {
		t.Fatalf("Set chat: %v", err)
	}

	newCtx := gw.applyPinnedLarkLLMSelection(ctx, msg)
	if newCtx == ctx {
		t.Fatal("expected context to be enriched")
	}
	// The chat-specific model should win. We verify via buildModelStatus.
	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "chat-model") {
		t.Fatalf("expected chat-model in status, got: %s", status)
	}
	if !strings.Contains(status, "[当前会话]") {
		t.Fatalf("expected [当前会话] scope label, got: %s", status)
	}
}

func TestSetModelDefaultsToChannelScope(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}

	if err := gw.setModelSelection(ctx, msg, "llama_server/llama3:latest", false); err != nil {
		t.Fatalf("setModelSelection: %v", err)
	}

	// Should be visible from a different chat.
	otherMsg := &incomingMessage{chatID: "oc_other", senderID: "ou_user"}
	status := gw.buildModelStatus(ctx, otherMsg)
	if !strings.Contains(status, "llama3:latest") {
		t.Fatalf("expected model in status from other chat, got: %s", status)
	}
	if !strings.Contains(status, "[全局]") {
		t.Fatalf("expected [全局] scope label, got: %s", status)
	}
}

func TestSetModelWithChatFlag(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}

	if err := gw.setModelSelection(ctx, msg, "llama_server/chat-only-model", true); err != nil {
		t.Fatalf("setModelSelection: %v", err)
	}

	// Should be visible in the same chat with [当前会话] label.
	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "chat-only-model") {
		t.Fatalf("expected chat-only-model in status, got: %s", status)
	}
	if !strings.Contains(status, "[当前会话]") {
		t.Fatalf("expected [当前会话], got: %s", status)
	}

	// Should be visible for another sender in the same chat (group-chat semantics).
	sameChatOtherSender := &incomingMessage{chatID: "oc_chat", senderID: "ou_other"}
	sameChatStatus := gw.buildModelStatus(ctx, sameChatOtherSender)
	if !strings.Contains(sameChatStatus, "chat-only-model") {
		t.Fatalf("expected chat-only-model for same chat other sender, got: %s", sameChatStatus)
	}

	// Should NOT be visible from a different chat.
	otherMsg := &incomingMessage{chatID: "oc_other", senderID: "ou_user"}
	otherStatus := gw.buildModelStatus(ctx, otherMsg)
	if strings.Contains(otherStatus, "chat-only-model") {
		t.Fatalf("chat-specific model should not leak to other chats, got: %s", otherStatus)
	}
}

func TestApplyPinnedSupportsLegacyChatUserScope(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}

	legacyScope := subscription.SelectionScope{Channel: "lark", ChatID: "oc_chat", UserID: "ou_user"}
	legacySel := subscription.Selection{Mode: "cli", Provider: "llama_server", Model: "legacy-model", Source: "llama_server"}
	if err := gw.llmSelections.Set(ctx, legacyScope, legacySel); err != nil {
		t.Fatalf("set legacy scope: %v", err)
	}

	newCtx := gw.applyPinnedLarkLLMSelection(ctx, msg)
	if newCtx == ctx {
		t.Fatal("expected context to include legacy pinned selection")
	}
	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "legacy-model") {
		t.Fatalf("expected legacy-model in status, got: %s", status)
	}
}

func TestApplyPinnedGroupChatIgnoresLegacyChatUserScope(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_group", senderID: "ou_user", isGroup: true}

	channelSel := subscription.Selection{
		Mode:     "cli",
		Provider: "llama_server",
		Model:    "channel-model",
		Source:   "llama_server",
	}
	if err := gw.llmSelections.Set(ctx, channelScope(), channelSel); err != nil {
		t.Fatalf("set channel scope: %v", err)
	}

	legacyScope := subscription.SelectionScope{Channel: "lark", ChatID: "oc_group", UserID: "ou_user"}
	legacySel := subscription.Selection{
		Mode:     "cli",
		Provider: "llama_server",
		Model:    "legacy-model",
		Source:   "llama_server",
	}
	if err := gw.llmSelections.Set(ctx, legacyScope, legacySel); err != nil {
		t.Fatalf("set legacy scope: %v", err)
	}

	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "channel-model") {
		t.Fatalf("expected channel-model in status, got: %s", status)
	}
	if strings.Contains(status, "legacy-model") {
		t.Fatalf("expected legacy scope to be ignored in group chat, got: %s", status)
	}
}

func TestBuildModelStatusShowsScopeLabel(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}

	// No selection → default message.
	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "当前未设置") {
		t.Fatalf("expected default message, got: %s", status)
	}

	// Set global.
	if err := gw.setModelSelection(ctx, msg, "llama_server/global-model", false); err != nil {
		t.Fatalf("set: %v", err)
	}
	status = gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "[全局]") {
		t.Fatalf("expected [全局], got: %s", status)
	}

	// Override per-chat.
	if err := gw.setModelSelection(ctx, msg, "llama_server/override-model", true); err != nil {
		t.Fatalf("set chat: %v", err)
	}
	status = gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "[当前会话]") {
		t.Fatalf("expected [当前会话] after override, got: %s", status)
	}
	if !strings.Contains(status, "override-model") {
		t.Fatalf("expected override-model, got: %s", status)
	}
}

func TestClearChannelLevel(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}

	if err := gw.setModelSelection(ctx, msg, "llama_server/model-x", false); err != nil {
		t.Fatalf("set: %v", err)
	}
	if err := gw.clearModelSelection(ctx, msg, false); err != nil {
		t.Fatalf("clear: %v", err)
	}
	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "当前未设置") {
		t.Fatalf("expected cleared status, got: %s", status)
	}
}

func TestClearChatOnly(t *testing.T) {
	t.Parallel()
	gw := newTestGatewayWithStore(t)
	ctx := context.Background()
	msg := &incomingMessage{chatID: "oc_chat", senderID: "ou_user"}

	// Set both levels.
	if err := gw.setModelSelection(ctx, msg, "llama_server/global-model", false); err != nil {
		t.Fatalf("set global: %v", err)
	}
	if err := gw.setModelSelection(ctx, msg, "llama_server/chat-model", true); err != nil {
		t.Fatalf("set chat: %v", err)
	}

	// Clear only the chat override.
	if err := gw.clearModelSelection(ctx, msg, true); err != nil {
		t.Fatalf("clear chat: %v", err)
	}

	// Should fall back to global.
	status := gw.buildModelStatus(ctx, msg)
	if !strings.Contains(status, "global-model") {
		t.Fatalf("expected global model after chat clear, got: %s", status)
	}
	if !strings.Contains(status, "[全局]") {
		t.Fatalf("expected [全局] after chat clear, got: %s", status)
	}
}

func TestHasFlag(t *testing.T) {
	t.Parallel()
	if !hasFlag([]string{"/model", "use", "llama_server/llama3", "--chat"}, "--chat") {
		t.Fatal("expected --chat to be found")
	}
	if hasFlag([]string{"/model", "use", "llama_server/llama3"}, "--chat") {
		t.Fatal("expected --chat NOT to be found")
	}
}

func TestFirstNonFlag(t *testing.T) {
	t.Parallel()
	if got := firstNonFlag([]string{"llama_server/llama3", "--chat"}); got != "llama_server/llama3" {
		t.Fatalf("expected llama_server/llama3, got %q", got)
	}
	if got := firstNonFlag([]string{"--chat", "llama_server/llama3"}); got != "llama_server/llama3" {
		t.Fatalf("expected llama_server/llama3, got %q", got)
	}
	if got := firstNonFlag([]string{"--chat"}); got != "" {
		t.Fatalf("expected empty, got %q", got)
	}
}
