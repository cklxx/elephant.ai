package server

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"alex/internal/shared/logging"
)

// HooksBridgeConfig configures the Claude Code hooks bridge endpoint.
type HooksBridgeConfig struct {
	Enabled bool   `yaml:"enabled"`
	Token   string `yaml:"token"`
}

// HooksBridge receives Claude Code hook events and forwards them to Lark.
type HooksBridge struct {
	gateway LarkNotifier
	token   string
	logger  logging.Logger
	// chatID is the default chat to send hook events to.
	// In production this would come from a mapping; for now use a config value.
	defaultChatID string
}

// NewHooksBridge constructs a hooks bridge.
// LarkNotifier is the subset of lark.Gateway used by the hooks bridge.
type LarkNotifier interface {
	SendNotification(ctx context.Context, chatID, text string) error
}

func NewHooksBridge(gateway LarkNotifier, token, defaultChatID string, logger logging.Logger) *HooksBridge {
	return &HooksBridge{
		gateway:       gateway,
		token:         token,
		logger:        logging.OrNop(logger),
		defaultChatID: defaultChatID,
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
func (h *HooksBridge) resolveChatID(r *http.Request) string {
	// Allow override via query parameter
	if chatID := strings.TrimSpace(r.URL.Query().Get("chat_id")); chatID != "" {
		return chatID
	}
	return h.defaultChatID
}

// formatHookEvent formats a hook payload into a Lark message.
func (h *HooksBridge) formatHookEvent(p hookPayload) string {
	switch p.Event {
	case "PostToolUse":
		argsPreview := truncateHookText(string(p.ToolInput), 200)
		return fmt.Sprintf("[CC] 执行了: %s(%s)", p.ToolName, argsPreview)
	case "Stop":
		var sb strings.Builder
		sb.WriteString("[CC] 任务完成")
		if p.StopReason != "" {
			sb.WriteString(fmt.Sprintf(" (%s)", p.StopReason))
		}
		if p.Answer != "" {
			sb.WriteString("\n")
			sb.WriteString(truncateHookText(p.Answer, 800))
		}
		if p.Error != "" {
			sb.WriteString("\n错误: ")
			sb.WriteString(truncateHookText(p.Error, 400))
		}
		return sb.String()
	case "PreToolUse":
		// Could be formatted as a permission request, but for passive mode
		// just log the tool about to be used
		argsPreview := truncateHookText(string(p.ToolInput), 200)
		return fmt.Sprintf("[CC] 准备执行: %s(%s)", p.ToolName, argsPreview)
	default:
		return ""
	}
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
