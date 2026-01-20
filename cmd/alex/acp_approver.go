package main

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
)

type acpApprover struct {
	server    *acpServer
	sessionID string
}

func newACPApprover(server *acpServer, sessionID string) *acpApprover {
	return &acpApprover{
		server:    server,
		sessionID: sessionID,
	}
}

func (a *acpApprover) RequestApproval(ctx context.Context, request *ports.ApprovalRequest) (*ports.ApprovalResponse, error) {
	if request == nil {
		return &ports.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}
	if request.AutoApprove {
		return &ports.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}
	if a == nil || a.server == nil || a.server.rpc == nil {
		return &ports.ApprovalResponse{Approved: true, Action: "approve"}, nil
	}

	toolCallID := strings.TrimSpace(request.ToolCallID)
	if toolCallID == "" {
		toolCallID = fmt.Sprintf("tool-%s", randSuffix(8))
	}
	title := strings.TrimSpace(request.Summary)
	if title == "" {
		title = strings.TrimSpace(request.ToolName)
	}
	if title == "" {
		title = "Approval requested"
	}

	toolCall := map[string]any{
		"toolCallId": toolCallID,
		"title":      title,
		"status":     "pending",
	}
	if kind := toolKindForName(request.ToolName); kind != "" {
		toolCall["kind"] = kind
	}
	if len(request.Arguments) > 0 {
		toolCall["rawInput"] = request.Arguments
	}
	if request.FilePath != "" {
		toolCall["locations"] = []map[string]any{{"path": request.FilePath}}
	}

	options := []map[string]any{
		{
			"optionId": "allow_once",
			"name":     "Allow",
			"kind":     "allow_once",
		},
		{
			"optionId": "reject_once",
			"name":     "Reject",
			"kind":     "reject_once",
		},
	}

	resp, err := a.server.rpc.Call(ctx, "session/request_permission", map[string]any{
		"sessionId": a.sessionID,
		"toolCall":  toolCall,
		"options":   options,
	})
	if err != nil {
		if errors.Is(err, context.Canceled) {
			return &ports.ApprovalResponse{Approved: false, Action: "cancel"}, nil
		}
		return nil, err
	}
	if resp == nil {
		return nil, fmt.Errorf("permission response missing")
	}
	if resp.Error != nil {
		return nil, resp.Error
	}

	outcome, optionID := parsePermissionOutcome(resp.Result)
	switch outcome {
	case "cancelled":
		return &ports.ApprovalResponse{Approved: false, Action: "cancel"}, nil
	case "selected":
		if strings.HasPrefix(optionID, "allow") {
			return &ports.ApprovalResponse{Approved: true, Action: "approve"}, nil
		}
		return &ports.ApprovalResponse{Approved: false, Action: "reject"}, nil
	default:
		return &ports.ApprovalResponse{Approved: false, Action: "reject"}, nil
	}
}

func parsePermissionOutcome(result any) (string, string) {
	payload, ok := result.(map[string]any)
	if !ok {
		return "", ""
	}
	outcomeRaw, hasOutcome := payload["outcome"]
	if !hasOutcome {
		return "", ""
	}
	outcomeMap, ok := outcomeRaw.(map[string]any)
	if !ok {
		if val, ok := outcomeRaw.(string); ok {
			return strings.ToLower(val), ""
		}
		return "", ""
	}
	outcome := strings.ToLower(strings.TrimSpace(stringParam(outcomeMap, "outcome")))
	optionID := strings.TrimSpace(stringParam(outcomeMap, "optionId"))
	return outcome, optionID
}
