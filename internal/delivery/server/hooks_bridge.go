package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"path/filepath"
	"strings"

	"alex/internal/shared/logging"
	"alex/internal/shared/uxphrases"
)

// HooksBridgeConfig configures the Claude Code hooks bridge endpoint.
type HooksBridgeConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

// NoticeLoader loads the notice binding to determine which chat receives
// hook notifications. It mirrors the read-only subset of noticeStateStore.
type NoticeLoader interface {
	Load() (chatID string, ok bool, err error)
}

// NoticeLoaderFunc adapts a plain function to the NoticeLoader interface.
type NoticeLoaderFunc func() (string, bool, error)

func (f NoticeLoaderFunc) Load() (string, bool, error) { return f() }

// HooksBridge receives Claude Code hook events and forwards them to Lark.
type HooksBridge struct {
	gateway       LarkNotifier
	token         string
	logger        logging.Logger
	defaultChatID string
	noticeLoader  NoticeLoader
}

// NewHooksBridge constructs a hooks bridge.
// LarkNotifier is the subset of lark.Gateway used by the hooks bridge.
type LarkNotifier interface {
	SendNotification(ctx context.Context, chatID, text string) error
}

func NewHooksBridge(gateway LarkNotifier, noticeLoader NoticeLoader, token, defaultChatID string, logger logging.Logger) *HooksBridge {
	return &HooksBridge{
		gateway:       gateway,
		token:         token,
		logger:        logging.OrNop(logger),
		defaultChatID: defaultChatID,
		noticeLoader:  noticeLoader,
	}
}

// hookPayload represents the JSON payload from a Claude Code hook event.
type hookPayload struct {
	Event     string          `json:"event"`      // e.g. "PostToolUse", "Stop", "PreToolUse"
	SessionID string          `json:"session_id"`
	ToolName  string          `json:"tool_name"`
	ToolInput json.RawMessage `json:"tool_input"`
	Output    string          `json:"output"`
	Error     string          `json:"error"`
	// Stop event fields
	StopReason string `json:"stop_reason"`
	Answer     string `json:"answer"`
}

// ServeHTTP handles POST /api/hooks/claude-code.
func (h *HooksBridge) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Verify token
	if h.token != "" {
		auth := strings.TrimPrefix(r.Header.Get("Authorization"), "Bearer ")
		if subtle.ConstantTimeCompare([]byte(strings.TrimSpace(auth)), []byte(h.token)) != 1 {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}
	}

	defer r.Body.Close()
	body, err := io.ReadAll(io.LimitReader(r.Body, 64*1024))
	if err != nil {
		http.Error(w, "read body failed", http.StatusBadRequest)
		return
	}

	var payload hookPayload
	if err := json.Unmarshal(body, &payload); err != nil {
		http.Error(w, "invalid json", http.StatusBadRequest)
		return
	}

	chatID := h.resolveChatID(r)
	if chatID == "" {
		http.Error(w, "no target chat_id", http.StatusBadRequest)
		return
	}

	message := h.formatHookEvent(payload)
	if message == "" {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	h.sendToLark(r.Context(), chatID, message)
	w.WriteHeader(http.StatusOK)
}

// resolveChatID determines the target Lark chat for a hook event.
// Priority: query param > notice binding > defaultChatID.
func (h *HooksBridge) resolveChatID(r *http.Request) string {
	// 1. Allow override via query parameter.
	if chatID := strings.TrimSpace(r.URL.Query().Get("chat_id")); chatID != "" {
		return chatID
	}

	// 2. Try notice binding.
	if h.noticeLoader != nil {
		if chatID, ok, err := h.noticeLoader.Load(); err == nil && ok && chatID != "" {
			return chatID
		}
	}

	// 3. Fall back to default.
	return h.defaultChatID
}

// formatHookEvent formats a hook payload into a friendly Chinese Lark message.
func (h *HooksBridge) formatHookEvent(p hookPayload) string {
	switch p.Event {
	case "PostToolUse":
		return formatPostToolUse(p)
	case "Stop":
		return formatStop(p)
	case "PreToolUse":
		return formatPreToolUse(p)
	default:
		return ""
	}
}

// formatPostToolUse creates a friendly message for a completed tool use.
func formatPostToolUse(p hookPayload) string {
	phrase := uxphrases.ToolPhrase(p.ToolName, 0)
	detail := toolDetail(p.ToolName, p.ToolInput)
	if detail != "" {
		return fmt.Sprintf("%s\n%s", phrase, detail)
	}
	return phrase
}

// formatPreToolUse creates a friendly message for a tool about to be used.
func formatPreToolUse(p hookPayload) string {
	phrase := uxphrases.ToolPhrase(p.ToolName, 1)
	detail := toolDetail(p.ToolName, p.ToolInput)
	if detail != "" {
		return fmt.Sprintf("%s\n%s", phrase, detail)
	}
	return phrase
}

// formatStop creates a completion message.
func formatStop(p hookPayload) string {
	var sb strings.Builder
	sb.WriteString("任务完成")
	if p.StopReason != "" {
		sb.WriteString(fmt.Sprintf(" (%s)", p.StopReason))
	}
	if p.Answer != "" {
		sb.WriteString("\n")
		sb.WriteString(truncateHookText(p.Answer, 800))
	}
	if p.Error != "" {
		sb.WriteString("\n出错了: ")
		sb.WriteString(truncateHookText(p.Error, 400))
	}
	return sb.String()
}

// toolDetail extracts a brief context hint from tool input.
func toolDetail(toolName string, input json.RawMessage) string {
	if len(input) == 0 {
		return ""
	}
	var m map[string]interface{}
	if err := json.Unmarshal(input, &m); err != nil {
		return ""
	}

	lower := strings.ToLower(toolName)

	// File operations: show filename.
	if hasFilePrefix(lower) {
		if path := extractString(m, "path", "file_path", "filename"); path != "" {
			return fmt.Sprintf("📄 %s", filepath.Base(path))
		}
	}

	// Shell/exec: show command.
	if hasShellPrefix(lower) {
		if cmd := extractString(m, "command", "cmd"); cmd != "" {
			return fmt.Sprintf("$ %s", truncateHookText(cmd, 120))
		}
	}

	// Search: show query.
	if hasSearchPrefix(lower) {
		if q := extractString(m, "query", "search_query", "pattern"); q != "" {
			return fmt.Sprintf("🔍 %s", truncateHookText(q, 120))
		}
	}

	return ""
}

func hasFilePrefix(name string) bool {
	for _, p := range []string{"read_file", "write_file", "replace_in_file", "create_file",
		"read", "write", "edit", "glob", "grep", "view_file", "patch_file"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func hasShellPrefix(name string) bool {
	for _, p := range []string{"shell_exec", "execute_code", "run_command", "bash", "terminal", "exec"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func hasSearchPrefix(name string) bool {
	for _, p := range []string{"web_search", "web_fetch", "tavily", "search_web", "search_file", "search_code"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func extractString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			if s, ok := v.(string); ok {
				return strings.TrimSpace(s)
			}
		}
	}
	return ""
}

// sendToLark dispatches a text message to the specified Lark chat.
func (h *HooksBridge) sendToLark(ctx context.Context, chatID, message string) {
	if h.gateway == nil {
		h.logger.Warn("Hooks bridge: gateway not set")
		return
	}

	if err := h.gateway.SendNotification(ctx, chatID, message); err != nil {
		h.logger.Warn("Hooks bridge send failed: %v", err)
	}
}

func truncateHookText(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if max <= 0 || len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
