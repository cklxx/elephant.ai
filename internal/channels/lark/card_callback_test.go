package lark

import (
	"testing"

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
