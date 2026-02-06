package lark

import (
	"context"
	"testing"
	"time"

	"alex/internal/shared/logging"

	"github.com/larksuite/oapi-sdk-go/v3/event/dispatcher/callback"
)

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
