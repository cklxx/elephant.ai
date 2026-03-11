package tools

import (
	"context"
	"errors"
	"strings"
	"testing"

	ports "alex/internal/domain/agent/ports"
	toolports "alex/internal/domain/agent/ports/tools"
	"alex/internal/infra/tools/builtin/shared"
)

type approvalTestExecutor struct {
	metadata   ports.ToolMetadata
	definition ports.ToolDefinition
	result     *ports.ToolResult
	err        error
	calls      int
}

func (e *approvalTestExecutor) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	e.calls++
	if e.result != nil {
		return e.result, e.err
	}
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, e.err
}

func (e *approvalTestExecutor) Definition() ports.ToolDefinition {
	if e.definition.Name != "" {
		return e.definition
	}
	return ports.ToolDefinition{Name: e.metadata.Name}
}

func (e *approvalTestExecutor) Metadata() ports.ToolMetadata {
	return e.metadata
}

type approvalStub struct {
	response *toolports.ApprovalResponse
	err      error
	calls    int
	lastReq  *toolports.ApprovalRequest
}

func (s *approvalStub) RequestApproval(_ context.Context, req *toolports.ApprovalRequest) (*toolports.ApprovalResponse, error) {
	s.calls++
	s.lastReq = req
	return s.response, s.err
}

func TestApprovalExecutor_NilDelegateReturnsToolResultError(t *testing.T) {
	exec := NewApprovalExecutor(nil)

	result, err := exec.Execute(context.Background(), ports.ToolCall{ID: "call-1"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatal("expected tool result error for missing delegate")
	}
	if result.CallID != "call-1" {
		t.Fatalf("CallID = %q, want call-1", result.CallID)
	}
}

func TestApprovalExecutor_ReadOnlyBypassesApprover(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata: ports.ToolMetadata{Name: "read_file", SafetyLevel: ports.SafetyLevelReadOnly},
	}
	approver := &approvalStub{response: &toolports.ApprovalResponse{Approved: true}}
	ctx := shared.WithApprover(context.Background(), approver)

	exec := NewApprovalExecutor(delegate)
	result, err := exec.Execute(ctx, ports.ToolCall{ID: "call-ro", Name: "read_file"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Content != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if delegate.calls != 1 {
		t.Fatalf("delegate calls = %d, want 1", delegate.calls)
	}
	if approver.calls != 0 {
		t.Fatalf("approver calls = %d, want 0", approver.calls)
	}
}

func TestApprovalExecutor_AutoApproveBypassesApprover(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata: ports.ToolMetadata{Name: "file_write", SafetyLevel: ports.SafetyLevelHighImpact},
	}
	approver := &approvalStub{response: &toolports.ApprovalResponse{Approved: false}}
	ctx := shared.WithAutoApprove(shared.WithApprover(context.Background(), approver), true)

	exec := NewApprovalExecutor(delegate)
	result, err := exec.Execute(ctx, ports.ToolCall{ID: "call-aa", Name: "file_write"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Content != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if delegate.calls != 1 {
		t.Fatalf("delegate calls = %d, want 1", delegate.calls)
	}
	if approver.calls != 0 {
		t.Fatalf("approver calls = %d, want 0", approver.calls)
	}
}

func TestApprovalExecutor_ApprovedRequestIncludesHighImpactFields(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata: ports.ToolMetadata{
			Name:        "file_write",
			SafetyLevel: ports.SafetyLevelHighImpact,
		},
		definition: ports.ToolDefinition{Name: "file_write"},
	}
	approver := &approvalStub{response: &toolports.ApprovalResponse{Approved: true}}
	ctx := shared.WithApprover(context.Background(), approver)
	call := ports.ToolCall{
		ID:   "call-hi",
		Name: "write_alias",
		Arguments: map[string]any{
			" arg9 ": "ignored in summary tail",
			"arg2":   2,
			"arg1":   1,
			"arg0":   0,
			"arg4":   4,
			"arg3":   3,
			"arg5":   5,
			"arg6":   6,
			"arg7":   7,
			"arg8":   8,
			"path":   " /tmp/data.txt ",
		},
	}

	exec := NewApprovalExecutor(delegate)
	result, err := exec.Execute(ctx, call)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Content != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if approver.calls != 1 {
		t.Fatalf("approver calls = %d, want 1", approver.calls)
	}
	if delegate.calls != 1 {
		t.Fatalf("delegate calls = %d, want 1", delegate.calls)
	}
	req := approver.lastReq
	if req == nil {
		t.Fatal("expected approval request")
	}
	if req.Operation != "file_write" {
		t.Fatalf("Operation = %q, want file_write", req.Operation)
	}
	if req.ToolCallID != "call-hi" || req.ToolName != "write_alias" {
		t.Fatalf("unexpected tool identity in request: %+v", req)
	}
	if req.FilePath != "/tmp/data.txt" {
		t.Fatalf("FilePath = %q, want /tmp/data.txt", req.FilePath)
	}
	if req.SafetyLevel != ports.SafetyLevelHighImpact {
		t.Fatalf("SafetyLevel = %d, want %d", req.SafetyLevel, ports.SafetyLevelHighImpact)
	}
	if req.RollbackSteps == "" || !strings.Contains(req.RollbackSteps, "/tmp/data.txt") {
		t.Fatalf("RollbackSteps = %q, want file-specific rollback", req.RollbackSteps)
	}
	if req.AlternativePlan != "" {
		t.Fatalf("AlternativePlan = %q, want empty for L3", req.AlternativePlan)
	}
	if !strings.Contains(req.Summary, "Approval required for file_write (L3)") {
		t.Fatalf("Summary missing safety prefix: %q", req.Summary)
	}
	if !strings.Contains(req.Summary, "path=/tmp/data.txt") {
		t.Fatalf("Summary missing path: %q", req.Summary)
	}
	if !strings.Contains(req.Summary, "args=arg0, arg1, arg2, arg3, arg4, arg5, arg6, arg7, ...") {
		t.Fatalf("Summary missing truncated sorted args: %q", req.Summary)
	}
}

func TestApprovalExecutor_IrreversibleDeleteIncludesAlternativePlan(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata: ports.ToolMetadata{
			Name:        "file_delete",
			SafetyLevel: ports.SafetyLevelIrreversible,
		},
	}
	approver := &approvalStub{response: &toolports.ApprovalResponse{Approved: true}}
	ctx := shared.WithApprover(context.Background(), approver)

	exec := NewApprovalExecutor(delegate)
	_, err := exec.Execute(ctx, ports.ToolCall{
		ID:        "call-del",
		Name:      "file_delete",
		Arguments: map[string]any{"resolved_path": "/tmp/old.txt"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if approver.lastReq == nil {
		t.Fatal("expected approval request")
	}
	if approver.lastReq.AlternativePlan == "" || !strings.Contains(approver.lastReq.AlternativePlan, "archive/disable") {
		t.Fatalf("AlternativePlan = %q, want delete-specific safer alternative", approver.lastReq.AlternativePlan)
	}
	if approver.lastReq.RollbackSteps == "" || !strings.Contains(approver.lastReq.RollbackSteps, "/tmp/old.txt") {
		t.Fatalf("RollbackSteps = %q, want file-specific rollback", approver.lastReq.RollbackSteps)
	}
}

func TestApprovalExecutor_ApproverErrorReturnsToolResultError(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata: ports.ToolMetadata{Name: "file_write", SafetyLevel: ports.SafetyLevelHighImpact},
	}
	approver := &approvalStub{err: errors.New("approval backend unavailable")}
	ctx := shared.WithApprover(context.Background(), approver)

	exec := NewApprovalExecutor(delegate)
	result, err := exec.Execute(ctx, ports.ToolCall{ID: "call-err", Name: "file_write"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || result.Error == nil || result.Error.Error() != "approval backend unavailable" {
		t.Fatalf("unexpected result error: %+v", result)
	}
	if delegate.calls != 0 {
		t.Fatalf("delegate calls = %d, want 0 after approver error", delegate.calls)
	}
}

func TestApprovalExecutor_RejectedApprovalReturnsToolResultError(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata: ports.ToolMetadata{Name: "file_write", SafetyLevel: ports.SafetyLevelHighImpact},
	}
	approver := &approvalStub{response: &toolports.ApprovalResponse{Approved: false}}
	ctx := shared.WithApprover(context.Background(), approver)

	exec := NewApprovalExecutor(delegate)
	result, err := exec.Execute(ctx, ports.ToolCall{ID: "call-reject", Name: "file_write"})
	if err != nil {
		t.Fatalf("unexpected Go error: %v", err)
	}
	if result == nil || result.Error == nil || result.Error.Error() != "operation rejected" {
		t.Fatalf("unexpected result error: %+v", result)
	}
	if delegate.calls != 0 {
		t.Fatalf("delegate calls = %d, want 0 on rejection", delegate.calls)
	}
}

func TestApprovalExecutor_DefinitionMetadataAndUnwrapDelegate(t *testing.T) {
	delegate := &approvalTestExecutor{
		metadata:   ports.ToolMetadata{Name: "file_write", Category: "files"},
		definition: ports.ToolDefinition{Name: "file_write", Description: "write file"},
	}
	exec := NewApprovalExecutor(delegate)
	unwrapped, ok := exec.(*ApprovalExecutor)
	if !ok {
		t.Fatalf("unexpected executor type %T", exec)
	}
	if unwrapped.Unwrap() != delegate {
		t.Fatal("Unwrap() should expose the wrapped executor")
	}
	if got := unwrapped.Definition(); got.Name != "file_write" || got.Description != "write file" {
		t.Fatalf("unexpected definition: %+v", got)
	}
	if got := unwrapped.Metadata(); got.Name != "file_write" || got.Category != "files" {
		t.Fatalf("unexpected metadata: %+v", got)
	}
}

func TestApprovalHelpers(t *testing.T) {
	meta := ports.ToolMetadata{Name: "replace_file", SafetyLevel: ports.SafetyLevelHighImpact}

	if got := extractFilePath(nil); got != "" {
		t.Fatalf("extractFilePath(nil) = %q, want empty", got)
	}
	if got := extractFilePath(map[string]any{"file_path": " /tmp/a.txt ", "path": "/tmp/b.txt"}); got != "/tmp/a.txt" {
		t.Fatalf("extractFilePath(file_path) = %q, want /tmp/a.txt", got)
	}
	if got := extractFilePath(map[string]any{"path": " /tmp/b.txt "}); got != "/tmp/b.txt" {
		t.Fatalf("extractFilePath(path) = %q, want /tmp/b.txt", got)
	}
	if got := extractFilePath(map[string]any{"resolved_path": " /tmp/c.txt "}); got != "/tmp/c.txt" {
		t.Fatalf("extractFilePath(resolved_path) = %q, want /tmp/c.txt", got)
	}
	if got := extractArgumentKeys(map[string]any{" b ": 1, "": 2, "a": 3}); len(got) != 2 || got[0] != "a" || got[1] != "b" {
		t.Fatalf("extractArgumentKeys() = %v, want [a b]", got)
	}
	summary := buildApprovalSummary(meta, ports.ToolCall{Arguments: map[string]any{"file_path": "/tmp/a.txt", "mode": "replace"}})
	if !strings.Contains(summary, "Approval required for replace_file (L3)") || !strings.Contains(summary, "path=/tmp/a.txt") {
		t.Fatalf("unexpected summary: %q", summary)
	}
	if got := buildRollbackSteps(ports.ToolMetadata{Name: "replace_file"}, ""); !strings.Contains(got, "replace_file operation") {
		t.Fatalf("unexpected rollback steps without path: %q", got)
	}
	if got := buildAlternativePlan(ports.ToolMetadata{Name: "replace_file"}); !strings.Contains(got, "dry-run") {
		t.Fatalf("unexpected non-delete alternative plan: %q", got)
	}
}
