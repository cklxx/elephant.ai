package http

import (
	"context"
	"net/http"
	"time"

	lark "alex/internal/delivery/channels/lark"
)

// LarkInjectGateway is the minimal interface for message injection.
type LarkInjectGateway interface {
	InjectMessageSync(ctx context.Context, req lark.InjectSyncRequest) *lark.InjectSyncResponse
}

// LarkInjectHandler handles POST /api/dev/inject for local end-to-end testing.
type LarkInjectHandler struct {
	gateway LarkInjectGateway
}

// NewLarkInjectHandler creates a new inject handler.
func NewLarkInjectHandler(gateway LarkInjectGateway) *LarkInjectHandler {
	return &LarkInjectHandler{gateway: gateway}
}

type injectRequest struct {
	Text               string `json:"text"`
	ChatID             string `json:"chat_id"`
	ChatType           string `json:"chat_type"`
	SenderID           string `json:"sender_id"`
	ToolMessageRounds  int    `json:"tool_message_rounds"`
	TimeoutSeconds     int    `json:"timeout_seconds"`
	AutoReply          bool   `json:"auto_reply"`
	MaxAutoReplyRounds int    `json:"max_auto_reply_rounds"`
}

type injectReplyItem struct {
	Method  string `json:"method"`
	Content string `json:"content"`
	MsgType string `json:"msg_type,omitempty"`
	Emoji   string `json:"emoji,omitempty"`
}

type injectResponseBody struct {
	Replies     []injectReplyItem `json:"replies"`
	DurationMs  int64             `json:"duration_ms"`
	Error       string            `json:"error,omitempty"`
	AutoReplies int               `json:"auto_replies,omitempty"`
}

// Handle processes a POST /api/dev/inject request.
func (h *LarkInjectHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req injectRequest
	if !decodeJSONRequest(w, r, &req, "invalid request body") {
		return
	}
	if req.Text == "" {
		http.Error(w, `"text" field is required`, http.StatusBadRequest)
		return
	}

	timeout := 5 * time.Minute
	if req.TimeoutSeconds > 0 {
		timeout = time.Duration(req.TimeoutSeconds) * time.Second
	}

	syncReq := lark.InjectSyncRequest{
		ChatID:             req.ChatID,
		ChatType:           req.ChatType,
		SenderID:           req.SenderID,
		Text:               req.Text,
		ToolMessageRounds:  req.ToolMessageRounds,
		Timeout:            timeout,
		AutoReply:          req.AutoReply,
		MaxAutoReplyRounds: req.MaxAutoReplyRounds,
	}

	resp := h.gateway.InjectMessageSync(r.Context(), syncReq)

	// Convert to wire format.
	var replies []injectReplyItem
	for _, call := range resp.Replies {
		replies = append(replies, injectReplyItem{
			Method:  call.Method,
			Content: call.Content,
			MsgType: call.MsgType,
			Emoji:   call.Emoji,
		})
	}

	body := injectResponseBody{
		Replies:     replies,
		DurationMs:  resp.Duration.Milliseconds(),
		Error:       resp.Error,
		AutoReplies: resp.AutoReplies,
	}
	status := http.StatusOK
	if resp.Error != "" {
		status = http.StatusInternalServerError
	}
	writeJSON(w, status, body)
}
