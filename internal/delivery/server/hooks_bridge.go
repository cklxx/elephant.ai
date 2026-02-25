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
	"sync"
	"time"

	"alex/internal/shared/logging"
	"alex/internal/shared/uxphrases"
	"alex/internal/shared/utils"
)

const defaultHooksAggregateWindow = 30 * time.Second

// NoticeLoader loads the notice binding to determine which chat receives
// hook notifications. It mirrors the read-only subset of noticeStateStore.
type NoticeLoader interface {
	Load() (chatID string, ok bool, err error)
}

// NoticeLoaderFunc adapts a plain function to the NoticeLoader interface.
type NoticeLoaderFunc func() (string, bool, error)

func (f NoticeLoaderFunc) Load() (string, bool, error) { return f() }

// HooksBridge receives Claude Code hook events and forwards them to Lark.
// PostToolUse events are aggregated into periodic summaries; Stop events
// are forwarded immediately.
type HooksBridge struct {
	gateway       LarkNotifier
	token         string
	logger        logging.Logger
	defaultChatID string
	noticeLoader  NoticeLoader

	// Aggregation state for PostToolUse events.
	mu          sync.Mutex
	toolBuffer  []toolEvent
	flushTimer  *time.Timer
	aggWindow   time.Duration
	now         func() time.Time // injectable clock for testing
}

// NewHooksBridge constructs a hooks bridge.
// LarkNotifier is the subset of lark.Gateway used by the hooks bridge.
type LarkNotifier interface {
	SendNotification(ctx context.Context, chatID, text string) error
}

// toolEvent captures one PostToolUse for aggregation.
type toolEvent struct {
	chatID  string
	message string
}

func NewHooksBridge(gateway LarkNotifier, noticeLoader NoticeLoader, token, defaultChatID string, logger logging.Logger) *HooksBridge {
	return &HooksBridge{
		gateway:       gateway,
		token:         token,
		logger:        logging.OrNop(logger),
		defaultChatID: defaultChatID,
		noticeLoader:  noticeLoader,
		aggWindow:     defaultHooksAggregateWindow,
		now:           time.Now,
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

// decodeHookPayload parses hook payloads leniently so we can accept
// null/variant field types from different hook emitters.
func decodeHookPayload(body []byte) (hookPayload, error) {
	var raw map[string]interface{}
	if err := json.Unmarshal(body, &raw); err != nil {
		return hookPayload{}, err
	}

	payload := hookPayload{
		Event:      normalizeHookEvent(firstString(raw, "event", "hook_event_name", "event_name")),
		SessionID:  firstString(raw, "session_id", "session", "sessionId"),
		ToolName:   firstString(raw, "tool_name", "tool", "name"),
		Output:     firstString(raw, "output", "tool_response", "result"),
		Error:      firstString(raw, "error", "err"),
		StopReason: firstString(raw, "stop_reason", "reason", "stop"),
		Answer:     firstString(raw, "answer", "final_answer", "finalAnswer", "response"),
	}
	if payload.Answer == "" {
		// Some emitters put terminal text in `output`.
		payload.Answer = payload.Output
	}
	if toolInput, ok := firstValue(raw, "tool_input", "tool_args", "input", "arguments", "args"); ok {
		if data, err := json.Marshal(toolInput); err == nil && string(data) != "null" {
			payload.ToolInput = json.RawMessage(data)
		}
	}
	return payload, nil
}

func normalizeHookEvent(raw string) string {
	s := strings.ToLower(strings.TrimSpace(raw))
	s = strings.ReplaceAll(s, "-", "")
	s = strings.ReplaceAll(s, "_", "")

	switch s {
	case "posttooluse", "tooluse", "tool":
		return "PostToolUse"
	case "pretooluse", "pretool":
		return "PreToolUse"
	case "stop", "complete", "completed", "taskcomplete", "taskcompleted":
		return "Stop"
	default:
		return strings.TrimSpace(raw)
	}
}

func firstValue(m map[string]interface{}, keys ...string) (interface{}, bool) {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			return v, true
		}
	}
	return nil, false
}

func firstString(m map[string]interface{}, keys ...string) string {
	for _, key := range keys {
		if v, ok := m[key]; ok {
			if s := coerceString(v); s != "" {
				return s
			}
		}
	}
	return ""
}

func coerceString(v interface{}) string {
	switch value := v.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(value)
	case json.Number:
		return value.String()
	case float64, float32, int, int32, int64, uint, uint32, uint64, bool:
		return strings.TrimSpace(fmt.Sprint(value))
	case map[string]interface{}:
		// Support nested event objects like {"event":{"name":"Stop"}}.
		if nested := firstString(value, "name", "type", "event", "value"); nested != "" {
			return nested
		}
		data, err := json.Marshal(value)
		if err != nil {
			return ""
		}
		return strings.TrimSpace(string(data))
	default:
		data, err := json.Marshal(value)
		if err == nil && string(data) != "null" {
			return strings.TrimSpace(string(data))
		}
		return strings.TrimSpace(fmt.Sprint(value))
	}
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

	payload, err := decodeHookPayload(body)
	if err != nil {
		h.logger.Warn("Hooks bridge: invalid json payload (%v): %s", err, truncateHookText(string(body), 180))
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

	switch payload.Event {
	case "PostToolUse", "PreToolUse":
		h.bufferToolEvent(r.Context(), chatID, message)
	case "Stop":
		// Flush any buffered tool events before the stop message.
		h.flushToolBuffer(r.Context())
		h.sendToLark(r.Context(), chatID, message)
	default:
		h.sendToLark(r.Context(), chatID, message)
	}
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
	answer := p.Answer
	if utils.IsBlank(answer) {
		// Some hook emitters place the final text in `output` for Stop events.
		answer = p.Output
	}
	if utils.HasContent(answer) {
		sb.WriteString("\n")
		sb.WriteString(truncateHookText(answer, 800))
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
	// Exact matches for single-word tools, prefix matches for compound names.
	switch name {
	case "read", "write", "edit", "glob", "grep":
		return true
	}
	for _, p := range []string{"read_", "write_", "edit_", "replace_in_file", "create_file",
		"view_file", "patch_file", "list_dir", "list_files"} {
		if strings.HasPrefix(name, p) {
			return true
		}
	}
	return false
}

func hasShellPrefix(name string) bool {
	switch name {
	case "bash":
		return true
	}
	for _, p := range []string{"shell_exec", "execute_code", "run_command", "terminal", "exec_"} {
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

// bufferToolEvent adds a tool event to the buffer and schedules a flush.
func (h *HooksBridge) bufferToolEvent(ctx context.Context, chatID, message string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.toolBuffer = append(h.toolBuffer, toolEvent{chatID: chatID, message: message})

	// Start flush timer on first buffered event.
	if h.flushTimer == nil {
		h.flushTimer = time.AfterFunc(h.aggWindow, func() {
			h.flushToolBuffer(ctx)
		})
	}
}

// flushToolBuffer sends all buffered tool events as a single aggregated message.
func (h *HooksBridge) flushToolBuffer(ctx context.Context) {
	h.mu.Lock()
	if len(h.toolBuffer) == 0 {
		h.mu.Unlock()
		return
	}
	events := h.toolBuffer
	h.toolBuffer = nil
	if h.flushTimer != nil {
		h.flushTimer.Stop()
		h.flushTimer = nil
	}
	h.mu.Unlock()

	// Group by chatID (normally all same, but be safe).
	grouped := make(map[string][]string)
	for _, e := range events {
		grouped[e.chatID] = append(grouped[e.chatID], e.message)
	}

	for chatID, messages := range grouped {
		text := formatToolSummary(messages)
		h.sendToLark(ctx, chatID, text)
	}
}

// Close flushes remaining buffered events. Call on shutdown.
func (h *HooksBridge) Close(ctx context.Context) {
	h.flushToolBuffer(ctx)
}

// formatToolSummary compresses multiple tool messages into a single notification.
func formatToolSummary(messages []string) string {
	if len(messages) == 1 {
		return messages[0]
	}
	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("🔧 %d tool calls:\n", len(messages)))
	for _, m := range messages {
		// Take first line of each message as a compact summary.
		line := m
		if idx := strings.IndexByte(m, '\n'); idx > 0 {
			line = m[:idx]
		}
		sb.WriteString("  • ")
		sb.WriteString(truncateHookText(line, 80))
		sb.WriteByte('\n')
	}
	return strings.TrimRight(sb.String(), "\n")
}

func truncateHookText(s string, max int) string {
	s = strings.TrimSpace(s)
	runes := []rune(s)
	if max <= 0 || len(runes) <= max {
		return s
	}
	return string(runes[:max]) + "..."
}
