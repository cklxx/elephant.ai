package http

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	lark "alex/internal/delivery/channels/lark"
)

type stubInjectGateway struct {
	lastReq lark.InjectSyncRequest
	resp    *lark.InjectSyncResponse
}

func (s *stubInjectGateway) InjectMessageSync(_ context.Context, req lark.InjectSyncRequest) *lark.InjectSyncResponse {
	s.lastReq = req
	if s.resp != nil {
		return s.resp
	}
	return &lark.InjectSyncResponse{}
}

func TestLarkInjectHandlerPassesToolMessageRounds(t *testing.T) {
	gw := &stubInjectGateway{
		resp: &lark.InjectSyncResponse{
			Replies: []lark.MessengerCall{{Method: "SendMessage", Content: `{"text":"ok"}`, MsgType: "text"}},
		},
	}
	handler := NewLarkInjectHandler(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/dev/inject", strings.NewReader(`{
		"text":"test",
		"chat_id":"oc_test",
		"tool_message_rounds":5,
		"timeout_seconds":60
	}`))
	w := httptest.NewRecorder()

	handler.Handle(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d body=%s", w.Code, w.Body.String())
	}
	if gw.lastReq.ToolMessageRounds != 5 {
		t.Fatalf("expected tool_message_rounds=5, got %d", gw.lastReq.ToolMessageRounds)
	}
	if gw.lastReq.Timeout != 60*time.Second {
		t.Fatalf("expected timeout 60s, got %v", gw.lastReq.Timeout)
	}
}

func TestLarkInjectHandlerRequiresText(t *testing.T) {
	gw := &stubInjectGateway{}
	handler := NewLarkInjectHandler(gw)

	req := httptest.NewRequest(http.MethodPost, "/api/dev/inject", strings.NewReader(`{"chat_id":"oc_test"}`))
	w := httptest.NewRecorder()

	handler.Handle(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400 for missing text, got %d", w.Code)
	}
}
