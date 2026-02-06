package main

import (
	"context"
	"io"
	"strings"
	"testing"

	tools "alex/internal/domain/agent/ports/tools"
)

func TestCLIApproverApproveAll(t *testing.T) {
	approver := newCLIApproverWithIO("session-1", strings.NewReader("a\n"), io.Discard, true)
	resp, err := approver.RequestApproval(context.Background(), &tools.ApprovalRequest{ToolName: "bash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || !resp.Approved || resp.Action != "approve_all" {
		t.Fatalf("expected approve_all, got %+v", resp)
	}

	resp, err = approver.RequestApproval(context.Background(), &tools.ApprovalRequest{ToolName: "bash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || !resp.Approved || resp.Action != "approve_all" {
		t.Fatalf("expected approve_all after toggle, got %+v", resp)
	}
}

func TestCLIApproverNonInteractiveDefaultsAllow(t *testing.T) {
	approver := newCLIApproverWithIO("session-2", strings.NewReader(""), io.Discard, false)
	resp, err := approver.RequestApproval(context.Background(), &tools.ApprovalRequest{ToolName: "bash"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if resp == nil || !resp.Approved {
		t.Fatalf("expected approval, got %+v", resp)
	}
}

func TestCLIApproverStoreReusesSession(t *testing.T) {
	store := newCLIApproverStore()
	first := store.forSession("session-1")
	second := store.forSession("session-1")
	if first != second {
		t.Fatalf("expected same approver for session")
	}
}
