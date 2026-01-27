package main

import (
	"context"
	"sync"

	tools "alex/internal/agent/ports/tools"
)

type tuiApprovalPrompt struct {
	request  *tools.ApprovalRequest
	response chan approvalDecision
}

type gocuiApprover struct {
	ui        *gocuiChatUI
	sessionID string

	mu             sync.Mutex
	autoApproveAll bool
}

func newGocuiApprover(ui *gocuiChatUI, sessionID string) *gocuiApprover {
	return &gocuiApprover{
		ui:        ui,
		sessionID: sessionID,
	}
}

func (a *gocuiApprover) RequestApproval(ctx context.Context, request *tools.ApprovalRequest) (*tools.ApprovalResponse, error) {
	if request == nil {
		return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}
	if request.AutoApprove {
		return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if a.autoApproveAll {
		return &tools.ApprovalResponse{Approved: true, Action: "approve_all"}, nil
	}
	if a.ui == nil {
		return &tools.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}

	decision, err := a.ui.requestApproval(ctx, request)
	if err != nil {
		return &tools.ApprovalResponse{Approved: false, Action: "cancel"}, nil
	}

	if decision.AutoApproveAll {
		a.autoApproveAll = true
	}

	return &tools.ApprovalResponse{Approved: decision.Approved, Action: decision.Action}, nil
}

type gocuiApproverStore struct {
	mu        sync.Mutex
	approvers map[string]*gocuiApprover
}

var defaultGocuiApproverStore = newGocuiApproverStore()

func newGocuiApproverStore() *gocuiApproverStore {
	return &gocuiApproverStore{approvers: make(map[string]*gocuiApprover)}
}

func (s *gocuiApproverStore) forSession(ui *gocuiChatUI, sessionID string) *gocuiApprover {
	if sessionID == "" {
		return newGocuiApprover(ui, "")
	}
	s.mu.Lock()
	defer s.mu.Unlock()

	if approver, ok := s.approvers[sessionID]; ok {
		if approver.ui != ui {
			approver.ui = ui
		}
		return approver
	}
	approver := newGocuiApprover(ui, sessionID)
	s.approvers[sessionID] = approver
	return approver
}

func gocuiApproverForSession(ui *gocuiChatUI, sessionID string) *gocuiApprover {
	return defaultGocuiApproverStore.forSession(ui, sessionID)
}
