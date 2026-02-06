package shadow

import (
	"context"
	"testing"

	tools "alex/internal/agent/ports/tools"
)

type stubApprover struct {
	approved bool
	requests []*tools.ApprovalRequest
}

func (s *stubApprover) RequestApproval(_ context.Context, req *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
	s.requests = append(s.requests, req)
	return &tools.ApprovalResponse{Approved: s.approved, Action: "approve"}, nil
}

func TestRequireApprovalRejectsWhenMissingApprover(t *testing.T) {
	err := RequireApproval(context.Background(), nil, Task{ID: "t1"})
	if err == nil {
		t.Fatal("expected error when approver missing")
	}
}

func TestRequireApprovalHonorsRejection(t *testing.T) {
	approver := &stubApprover{approved: false}
	err := RequireApproval(context.Background(), approver, Task{ID: "t1", Summary: "test"})
	if err == nil {
		t.Fatal("expected rejection error")
	}
	if len(approver.requests) != 1 {
		t.Fatalf("expected 1 approval request, got %d", len(approver.requests))
	}
}

func TestRequireApprovalAccepts(t *testing.T) {
	approver := &stubApprover{approved: true}
	err := RequireApproval(context.Background(), approver, Task{ID: "t1", Summary: "test"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(approver.requests) != 1 {
		t.Fatalf("expected 1 approval request, got %d", len(approver.requests))
	}
}
