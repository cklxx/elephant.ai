package lark

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"alex/internal/shared/logging"

	larkcore "github.com/larksuite/oapi-sdk-go/v3/core"
	larkevent "github.com/larksuite/oapi-sdk-go/v3/event"
	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

func TestNewCardCallbackHandlerEnabledWithoutVerificationToken(t *testing.T) {
	gw := &Gateway{cfg: Config{CardsEnabled: true}}
	handler := NewCardCallbackHandler(gw, logging.OrNop(nil))
	if handler == nil {
		t.Fatal("expected callback handler to be available even without verification token")
	}
}

func TestCardCallbackHandlerPlainURLVerificationWithEncryptKeyConfigured(t *testing.T) {
	gw := &Gateway{
		cfg: Config{
			CardsEnabled:                  true,
			CardCallbackVerificationToken: "verify_token",
			CardCallbackEncryptKey:        "encrypt_key",
		},
	}
	handler := NewCardCallbackHandler(gw, logging.OrNop(nil))
	if handler == nil {
		t.Fatal("expected callback handler")
	}

	req := httptest.NewRequest(http.MethodPost, "/api/lark/card/callback", strings.NewReader(`{
		"type":"url_verification",
		"challenge":"challenge_plain",
		"token":"verify_token"
	}`))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", rec.Code, rec.Body.String())
	}
	var payload struct {
		Challenge string `json:"challenge"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%q", err, rec.Body.String())
	}
	if payload.Challenge != "challenge_plain" {
		t.Fatalf("expected challenge_plain, got %q", payload.Challenge)
	}
}

func TestCardCallbackHandlerEncryptedEventWithoutSignatureHeaders(t *testing.T) {
	gw := &Gateway{
		cfg: Config{
			CardsEnabled:                  true,
			CardCallbackVerificationToken: "verify_token",
			CardCallbackEncryptKey:        "encrypt_key",
		},
	}
	handler := NewCardCallbackHandler(gw, logging.OrNop(nil))
	if handler == nil {
		t.Fatal("expected callback handler")
	}

	plainEvent := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"event_id":    "evt-1",
			"event_type":  "card.action.trigger",
			"app_id":      "app",
			"tenant_key":  "tenant",
			"create_time": "1760000000000",
			"token":       "verify_token",
		},
		"event": map[string]interface{}{
			"action": map[string]interface{}{
				"tag": "confirm_yes",
			},
		},
	}
	encrypt, err := larkcore.EncryptedEventMsg(context.Background(), plainEvent, "encrypt_key")
	if err != nil {
		t.Fatalf("encrypt event: %v", err)
	}
	body, err := json.Marshal(map[string]string{"encrypt": encrypt})
	if err != nil {
		t.Fatalf("marshal encrypted payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/lark/card/callback", strings.NewReader(string(body)))
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", rec.Code, rec.Body.String())
	}
	var payload struct {
		Toast *struct {
			Content string `json:"content"`
		} `json:"toast"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%q", err, rec.Body.String())
	}
	if payload.Toast == nil || payload.Toast.Content != "缺少会话信息" {
		t.Fatalf("expected toast content 缺少会话信息, got %#v", payload.Toast)
	}
}

func TestCardCallbackHandlerEncryptedEventWithInvalidSignatureFallsBackToNoSign(t *testing.T) {
	gw := &Gateway{
		cfg: Config{
			CardsEnabled:                  true,
			CardCallbackVerificationToken: "verify_token",
			CardCallbackEncryptKey:        "encrypt_key",
		},
	}
	handler := NewCardCallbackHandler(gw, logging.OrNop(nil))
	if handler == nil {
		t.Fatal("expected callback handler")
	}

	plainEvent := map[string]interface{}{
		"schema": "2.0",
		"header": map[string]interface{}{
			"event_id":    "evt-2",
			"event_type":  "card.action.trigger",
			"app_id":      "app",
			"tenant_key":  "tenant",
			"create_time": "1760000000001",
			"token":       "verify_token",
		},
		"event": map[string]interface{}{
			"action": map[string]interface{}{
				"tag": "confirm_yes",
			},
		},
	}
	encrypt, err := larkcore.EncryptedEventMsg(context.Background(), plainEvent, "encrypt_key")
	if err != nil {
		t.Fatalf("encrypt event: %v", err)
	}
	body, err := json.Marshal(map[string]string{"encrypt": encrypt})
	if err != nil {
		t.Fatalf("marshal encrypted payload: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/lark/card/callback", strings.NewReader(string(body)))
	req.Header.Set(larkevent.EventSignature, "invalid-signature")
	req.Header.Set(larkevent.EventRequestTimestamp, "1760000001")
	req.Header.Set(larkevent.EventRequestNonce, "nonce-1")

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("expected status 200, got %d with body %q", rec.Code, rec.Body.String())
	}
	var payload struct {
		Toast *struct {
			Content string `json:"content"`
		} `json:"toast"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &payload); err != nil {
		t.Fatalf("unmarshal response: %v, body=%q", err, rec.Body.String())
	}
	if payload.Toast == nil || payload.Toast.Content != "缺少会话信息" {
		t.Fatalf("expected toast content 缺少会话信息, got %#v", payload.Toast)
	}
}

func TestNewCardCallbackHandlerDisabledWhenCardsDisabled(t *testing.T) {
	gw := &Gateway{cfg: Config{CardsEnabled: false, CardCallbackVerificationToken: "token"}}
	handler := NewCardCallbackHandler(gw, logging.OrNop(nil))
	if handler != nil {
		t.Fatal("expected nil callback handler when cards are disabled")
	}
}

func TestCardActionToUserInputApprove(t *testing.T) {
	input := cardActionToUserInput(&callback.CallBackAction{Tag: "plan_review_approve"})
	if input != "OK" {
		t.Fatalf("expected OK, got %q", input)
	}
}

func TestCardActionToUserInputRequestChangesWithFormValue(t *testing.T) {
	action := &callback.CallBackAction{
		Tag: "plan_review_request_changes",
		FormValue: map[string]interface{}{
			"plan_feedback": "请加一步验收",
		},
	}
	input := cardActionToUserInput(action)
	if input != "请加一步验收" {
		t.Fatalf("expected feedback, got %q", input)
	}
}

func TestCardActionToUserInputRequestChangesFallback(t *testing.T) {
	action := &callback.CallBackAction{Tag: "plan_review_request_changes"}
	input := cardActionToUserInput(action)
	if input != "需要修改" {
		t.Fatalf("expected fallback, got %q", input)
	}
}

func TestCardActionToUserInputConfirm(t *testing.T) {
	input := cardActionToUserInput(&callback.CallBackAction{Tag: "confirm_yes"})
	if input != "OK" {
		t.Fatalf("expected OK, got %q", input)
	}
	input = cardActionToUserInput(&callback.CallBackAction{Tag: "confirm_no"})
	if input != "取消" {
		t.Fatalf("expected 取消, got %q", input)
	}
}

func TestCardActionToUserInputFallbackToInputValue(t *testing.T) {
	input := cardActionToUserInput(&callback.CallBackAction{Tag: "unknown", InputValue: "hello"})
	if input != "hello" {
		t.Fatalf("expected input value, got %q", input)
	}
}

func TestCardActionToUserInputFallbackToTextValue(t *testing.T) {
	input := cardActionToUserInput(&callback.CallBackAction{
		Tag: "model_use",
		Value: map[string]interface{}{
			"text": "/model use codex/gpt-5.2-codex",
		},
	})
	if input != "/model use codex/gpt-5.2-codex" {
		t.Fatalf("expected text value, got %q", input)
	}
}

func TestCardActionToUserInputAwaitChoice(t *testing.T) {
	input := cardActionToUserInput(&callback.CallBackAction{
		Tag: "await_choice_select",
		Value: map[string]interface{}{
			"text": "staging",
		},
	})
	if input != "staging" {
		t.Fatalf("expected option text, got %q", input)
	}
}

func TestHandleCardActionAttachmentSendImage(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{messenger: recorder, logger: logging.OrNop(nil)}
	event := &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Action: &callback.CallBackAction{
				Tag: "attachment_send",
				Value: map[string]interface{}{
					"image_key": "img_123",
				},
			},
			Context: &callback.Context{
				OpenChatID:    "oc_chat",
				OpenMessageID: "om_msg",
			},
		},
	}

	resp, err := gw.handleCardAction(context.Background(), event)
	if err != nil {
		t.Fatalf("handleCardAction failed: %v", err)
	}
	if resp == nil || resp.Toast == nil || resp.Toast.Content != "附件已发送" {
		t.Fatalf("expected attachment toast, got %#v", resp)
	}

	calls := waitForMessengerCalls(t, recorder, 1)
	if len(calls) == 0 {
		t.Fatal("expected attachment dispatch call")
	}
	if calls[0].Method != "ReplyMessage" {
		t.Fatalf("expected ReplyMessage, got %s", calls[0].Method)
	}
	if calls[0].MsgType != "image" {
		t.Fatalf("expected image message, got %q", calls[0].MsgType)
	}
}

func TestHandleCardActionAttachmentSendFile(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{messenger: recorder, logger: logging.OrNop(nil)}
	event := &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Action: &callback.CallBackAction{
				Tag: "attachment_send",
				Value: map[string]interface{}{
					"file_key": "file_123",
				},
			},
			Context: &callback.Context{
				OpenChatID:    "oc_chat",
				OpenMessageID: "om_msg",
			},
		},
	}

	resp, err := gw.handleCardAction(context.Background(), event)
	if err != nil {
		t.Fatalf("handleCardAction failed: %v", err)
	}
	if resp == nil || resp.Toast == nil || resp.Toast.Content != "附件已发送" {
		t.Fatalf("expected attachment toast, got %#v", resp)
	}

	calls := waitForMessengerCalls(t, recorder, 1)
	if len(calls) == 0 {
		t.Fatal("expected attachment dispatch call")
	}
	if calls[0].Method != "ReplyMessage" {
		t.Fatalf("expected ReplyMessage, got %s", calls[0].Method)
	}
	if calls[0].MsgType != "file" {
		t.Fatalf("expected file message, got %q", calls[0].MsgType)
	}
}

func TestHandleCardActionAttachmentSendMissingKey(t *testing.T) {
	recorder := NewRecordingMessenger()
	gw := &Gateway{messenger: recorder, logger: logging.OrNop(nil)}
	event := &callback.CardActionTriggerEvent{
		Event: &callback.CardActionTriggerRequest{
			Action: &callback.CallBackAction{Tag: "attachment_send"},
			Context: &callback.Context{
				OpenChatID:    "oc_chat",
				OpenMessageID: "om_msg",
			},
		},
	}

	resp, err := gw.handleCardAction(context.Background(), event)
	if err != nil {
		t.Fatalf("handleCardAction failed: %v", err)
	}
	if resp == nil || resp.Toast == nil || resp.Toast.Content != "附件信息缺失" {
		t.Fatalf("expected missing attachment toast, got %#v", resp)
	}

	calls := waitForMessengerCalls(t, recorder, 1)
	if len(calls) != 0 {
		t.Fatalf("expected no dispatch calls, got %#v", calls)
	}
}

func TestIsEncryptedCallbackPayload(t *testing.T) {
	if !isEncryptedCallbackPayload([]byte(`{"encrypt":"ciphertext"}`)) {
		t.Fatal("expected encrypted payload")
	}
	if isEncryptedCallbackPayload([]byte(`{"encrypt":"   "}`)) {
		t.Fatal("expected blank encrypt to be treated as plaintext payload")
	}
	if isEncryptedCallbackPayload([]byte(`{"token":"x"}`)) {
		t.Fatal("expected plaintext payload")
	}
	if isEncryptedCallbackPayload([]byte(`not-json`)) {
		t.Fatal("expected invalid json to be treated as plaintext payload")
	}
}

func TestHasLarkCallbackSignatureHeaders(t *testing.T) {
	header := http.Header{}
	if hasLarkCallbackSignatureHeaders(header) {
		t.Fatal("expected false without signature headers")
	}

	header.Set("X-Lark-Signature", "sig")
	header.Set("X-Lark-Request-Timestamp", "123")
	if hasLarkCallbackSignatureHeaders(header) {
		t.Fatal("expected false without nonce header")
	}

	header.Set("X-Lark-Request-Nonce", "nonce")
	if !hasLarkCallbackSignatureHeaders(header) {
		t.Fatal("expected true with signature, timestamp and nonce headers")
	}
}

func waitForMessengerCalls(t *testing.T, recorder *RecordingMessenger, min int) []MessengerCall {
	t.Helper()
	deadline := time.Now().Add(200 * time.Millisecond)
	for {
		calls := recorder.Calls()
		if len(calls) >= min {
			return calls
		}
		if time.Now().After(deadline) {
			return calls
		}
		time.Sleep(5 * time.Millisecond)
	}
}
