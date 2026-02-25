package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils/id"
)

// mockDispatcher implements agent.BackgroundTaskDispatcher for tool tests.
type mockDispatcher struct {
	dispatched  []dispatchCall
	summaries   []agent.BackgroundTaskSummary
	results     []agent.BackgroundTaskResult
	dispatchErr error
	replyCalls  []agent.InputResponse
	replyErr    error
	mergeResult *agent.MergeResult
	mergeErr    error
}

type dispatchCall struct {
	Req agent.BackgroundDispatchRequest
}

func (m *mockDispatcher) Dispatch(_ context.Context, req agent.BackgroundDispatchRequest) error {
	if m.dispatchErr != nil {
		return m.dispatchErr
	}
	m.dispatched = append(m.dispatched, dispatchCall{Req: req})
	return nil
}

func (m *mockDispatcher) Status(_ []string) []agent.BackgroundTaskSummary {
	return m.summaries
}

func (m *mockDispatcher) Collect(_ []string, _ bool, _ time.Duration) []agent.BackgroundTaskResult {
	return m.results
}

func (m *mockDispatcher) ReplyExternalInput(_ context.Context, resp agent.InputResponse) error {
	if m.replyErr != nil {
		return m.replyErr
	}
	m.replyCalls = append(m.replyCalls, resp)
	return nil
}

func (m *mockDispatcher) MergeExternalWorkspace(_ context.Context, _ string, _ agent.MergeStrategy) (*agent.MergeResult, error) {
	if m.mergeErr != nil {
		return nil, m.mergeErr
	}
	return m.mergeResult, nil
}

func ctxWithDispatcher(d agent.BackgroundTaskDispatcher) context.Context {
	return agent.WithBackgroundDispatcher(context.Background(), d)
}

// --- bg_dispatch tests ---

func TestBGDispatch_Success(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"description": "analyze logs",
			"prompt":      "Please analyze the logs",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 1 {
		t.Fatalf("expected 1 dispatch call, got %d", len(d.dispatched))
	}
	taskID := d.dispatched[0].Req.TaskID
	if taskID == "" {
		t.Fatal("expected generated task_id")
	}
	if !strings.HasPrefix(taskID, "bg-") {
		t.Errorf("expected task_id to start with bg-, got %s", taskID)
	}
	if d.dispatched[0].Req.AgentType != "internal" {
		t.Errorf("expected default agent_type=internal, got %s", d.dispatched[0].Req.AgentType)
	}
	if d.dispatched[0].Req.CausationID != "call-1" {
		t.Errorf("expected causation_id=call-1, got %s", d.dispatched[0].Req.CausationID)
	}
}

func TestBGDispatch_CustomAgentType(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"description": "external task",
			"prompt":      "do work",
			"agent_type":  "claude_code",
		},
	})

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if d.dispatched[0].Req.AgentType != "claude_code" {
		t.Errorf("expected agent_type=claude_code, got %s", d.dispatched[0].Req.AgentType)
	}
}

func TestBGDispatch_CodingDefaults(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-coding",
		Arguments: map[string]any{
			"description": "implement feature",
			"prompt":      "implement it",
			"task_kind":   "coding",
			"agent_type":  "codex",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 1 {
		t.Fatalf("expected dispatch call, got %d", len(d.dispatched))
	}
	req := d.dispatched[0].Req
	if req.WorkspaceMode != agent.WorkspaceModeWorktree {
		t.Fatalf("expected workspace_mode=worktree, got %s", req.WorkspaceMode)
	}
	if req.Config["task_kind"] != "coding" {
		t.Fatalf("expected task_kind=coding, got %q", req.Config["task_kind"])
	}
	if req.Config["coding_profile"] != "full_access" {
		t.Fatalf("expected coding_profile=full_access, got %q", req.Config["coding_profile"])
	}
	if req.Config["verify"] != "true" {
		t.Fatalf("expected verify=true, got %q", req.Config["verify"])
	}
	if req.Config["retry_max_attempts"] != "3" {
		t.Fatalf("expected retry_max_attempts=3, got %q", req.Config["retry_max_attempts"])
	}
	if req.Config["merge_on_success"] != "true" {
		t.Fatalf("expected merge_on_success=true, got %q", req.Config["merge_on_success"])
	}
}

func TestBGDispatch_CodingPlanDefaults(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-coding-plan",
		Arguments: map[string]any{
			"description":    "plan feature",
			"prompt":         "plan it",
			"task_kind":      "coding",
			"agent_type":     "codex",
			"execution_mode": "plan",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 1 {
		t.Fatalf("expected dispatch call, got %d", len(d.dispatched))
	}
	req := d.dispatched[0].Req
	if req.ExecutionMode != "plan" {
		t.Fatalf("expected execution_mode=plan, got %q", req.ExecutionMode)
	}
	if req.AutonomyLevel != "full" {
		t.Fatalf("expected autonomy_level=full, got %q", req.AutonomyLevel)
	}
	if req.WorkspaceMode != agent.WorkspaceModeShared {
		t.Fatalf("expected workspace_mode=shared in plan mode, got %s", req.WorkspaceMode)
	}
	if req.Config["verify"] != "false" {
		t.Fatalf("expected verify=false, got %q", req.Config["verify"])
	}
	if req.Config["merge_on_success"] != "false" {
		t.Fatalf("expected merge_on_success=false, got %q", req.Config["merge_on_success"])
	}
}

func TestBGDispatch_CodingNormalizesAgentType(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-coding-normalize",
		Arguments: map[string]any{
			"description": "implement feature",
			"prompt":      "implement it",
			"task_kind":   "coding",
			"agent_type":  "CoDeX",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 1 {
		t.Fatalf("expected dispatch call, got %d", len(d.dispatched))
	}
	if d.dispatched[0].Req.AgentType != "codex" {
		t.Fatalf("expected normalized agent_type=codex, got %q", d.dispatched[0].Req.AgentType)
	}
}

func TestBGDispatch_CodingRequiresExternalAgent(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-coding-internal",
		Arguments: map[string]any{
			"description": "implement feature",
			"prompt":      "implement it",
			"task_kind":   "coding",
			"agent_type":  "internal",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for coding task with internal agent")
	}
	if len(d.dispatched) != 0 {
		t.Fatalf("expected no dispatch on validation failure, got %d", len(d.dispatched))
	}
}

func TestBGPlan_DispatchesTopologicalOrder(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGPlan()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-plan-1",
		Arguments: map[string]any{
			"dispatch": true,
			"defaults": map[string]any{
				"agent_type":     "codex",
				"execution_mode": "execute",
				"autonomy_level": "full",
			},
			"tasks": []any{
				map[string]any{
					"task_id":     "A",
					"description": "root",
					"prompt":      "do A",
				},
				map[string]any{
					"task_id":     "B",
					"description": "child",
					"prompt":      "do B",
					"depends_on":  []any{"A"},
				},
				map[string]any{
					"task_id":     "C",
					"description": "leaf",
					"prompt":      "do C",
					"depends_on":  []any{"B"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 3 {
		t.Fatalf("expected 3 dispatched tasks, got %d", len(d.dispatched))
	}
	gotOrder := []string{
		d.dispatched[0].Req.TaskID,
		d.dispatched[1].Req.TaskID,
		d.dispatched[2].Req.TaskID,
	}
	wantOrder := []string{"A", "B", "C"}
	for i := range wantOrder {
		if gotOrder[i] != wantOrder[i] {
			t.Fatalf("unexpected dispatch order: got=%v want=%v", gotOrder, wantOrder)
		}
	}
	if d.dispatched[0].Req.Config["task_kind"] != "coding" {
		t.Fatalf("expected coding task defaults for external agents")
	}
}

func TestBGPlan_NormalizesAgentTypeAndAppliesCodingDefaults(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGPlan()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-plan-normalize",
		Arguments: map[string]any{
			"dispatch": true,
			"defaults": map[string]any{
				"agent_type": "CoDeX",
			},
			"tasks": []any{
				map[string]any{
					"task_id":     "A",
					"description": "root",
					"prompt":      "do A",
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) != 1 {
		t.Fatalf("expected 1 dispatched task, got %d", len(d.dispatched))
	}
	req := d.dispatched[0].Req
	if req.AgentType != "codex" {
		t.Fatalf("expected normalized agent_type=codex, got %q", req.AgentType)
	}
	if req.Config["task_kind"] != "coding" {
		t.Fatalf("expected coding defaults task_kind=coding, got %q", req.Config["task_kind"])
	}
}

func TestBGGraph_RendersDependencyGraph(t *testing.T) {
	d := &mockDispatcher{
		summaries: []agent.BackgroundTaskSummary{
			{
				ID:            "A",
				AgentType:     "codex",
				Status:        agent.BackgroundTaskStatusCompleted,
				ExecutionMode: "execute",
				AutonomyLevel: "full",
				Description:   "root task",
			},
			{
				ID:            "B",
				AgentType:     "claude_code",
				Status:        agent.BackgroundTaskStatusRunning,
				ExecutionMode: "plan",
				AutonomyLevel: "full",
				DependsOn:     []string{"A"},
				Description:   "child task",
			},
		},
	}
	ctx := ctxWithDispatcher(d)
	tool := NewBGGraph()

	result, err := tool.Execute(ctx, ports.ToolCall{ID: "call-graph-1"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected tool error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Background Dependency Graph") {
		t.Fatalf("unexpected content: %s", result.Content)
	}
	if !strings.Contains(result.Content, "depends_on: A") {
		t.Fatalf("expected dependency edge in content: %s", result.Content)
	}
	if !strings.Contains(result.Content, "mode=plan autonomy=full") {
		t.Fatalf("expected execution controls in content: %s", result.Content)
	}
}

func TestBGDispatch_CodingMergeRequiresVerify(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-coding-merge-verify",
		Arguments: map[string]any{
			"description":      "implement feature",
			"prompt":           "implement it",
			"task_kind":        "coding",
			"agent_type":       "codex",
			"merge_on_success": true,
			"verify":           false,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected validation error when merge_on_success=true and verify=false")
	}
	if len(d.dispatched) != 0 {
		t.Fatalf("expected no dispatch on validation failure, got %d", len(d.dispatched))
	}
}

func TestBGDispatch_MetadataIncludesRunIDs(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	ctx = id.WithSessionID(ctx, "session-1")
	ctx = id.WithRunID(ctx, "run-main")
	ctx = id.WithParentRunID(ctx, "run-parent")
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-meta",
		Arguments: map[string]any{
			"description": "metadata test",
			"prompt":      "do work",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if result.Metadata == nil {
		t.Fatal("expected metadata to be present")
	}
	if result.Metadata["run_id"] != "run-main" {
		t.Fatalf("expected run_id=run-main, got %v", result.Metadata["run_id"])
	}
	if result.Metadata["parent_run_id"] != "run-parent" {
		t.Fatalf("expected parent_run_id=run-parent, got %v", result.Metadata["parent_run_id"])
	}
	if result.Metadata["session_id"] != "session-1" {
		t.Fatalf("expected session_id=session-1, got %v", result.Metadata["session_id"])
	}
}

func TestBGDispatch_MissingRequired(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing description", map[string]any{"prompt": "p"}},
		{"missing prompt", map[string]any{"description": "d"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			result, err := tool.Execute(ctx, ports.ToolCall{ID: "c", Arguments: tc.args})
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if result.Error == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}

func TestBGDispatch_AutoTaskID(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "call-auto",
		Arguments: map[string]any{
			"description": "Analyze Server Logs",
			"prompt":      "Please analyze the server logs",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.dispatched) == 0 {
		t.Fatal("expected dispatch call")
	}
	taskID := d.dispatched[len(d.dispatched)-1].Req.TaskID
	if taskID == "" {
		t.Fatal("expected generated task_id")
	}
	if !strings.HasPrefix(taskID, "bg-") {
		t.Errorf("expected task_id to start with bg-, got %s", taskID)
	}
}

func TestBGDispatch_NoDispatcher(t *testing.T) {
	tool := NewBGDispatch()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "c",
		Arguments: map[string]any{
			"description": "d",
			"prompt":      "p",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when dispatcher not available")
	}
}

func TestBGDispatch_DispatchError(t *testing.T) {
	d := &mockDispatcher{dispatchErr: fmt.Errorf("duplicate ID")}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "c",
		Arguments: map[string]any{"description": "d", "prompt": "p"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected dispatch error in result")
	}
}

func TestBGDispatch_UnsupportedParam(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "c",
		Arguments: map[string]any{"description": "d", "prompt": "p", "unknown": "x"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported parameter")
	}
}

func TestBGDispatch_TaskIDRejected(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "c",
		Arguments: map[string]any{
			"task_id":     "t1",
			"description": "d",
			"prompt":      "p",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when task_id is provided")
	}
	if !strings.Contains(result.Content, "task_id") {
		t.Fatalf("expected task_id error, got: %s", result.Content)
	}
}

// --- bg_status tests ---

func TestBGStatus_Success(t *testing.T) {
	d := &mockDispatcher{
		summaries: []agent.BackgroundTaskSummary{
			{ID: "t1", Description: "task one", Status: agent.BackgroundTaskStatusRunning, AgentType: "internal"},
			{ID: "t2", Description: "task two", Status: agent.BackgroundTaskStatusCompleted, AgentType: "internal"},
		},
	}
	ctx := ctxWithDispatcher(d)
	tool := NewBGStatus()

	result, err := tool.Execute(ctx, ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "t1") || !strings.Contains(result.Content, "t2") {
		t.Errorf("expected both task IDs in output: %s", result.Content)
	}
}

func TestBGStatus_NoTasks(t *testing.T) {
	d := &mockDispatcher{summaries: nil}
	ctx := ctxWithDispatcher(d)
	tool := NewBGStatus()

	result, err := tool.Execute(ctx, ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No background tasks") {
		t.Errorf("expected 'No background tasks' message, got: %s", result.Content)
	}
}

func TestBGStatus_NoDispatcher(t *testing.T) {
	tool := NewBGStatus()
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when dispatcher not available")
	}
}

// --- bg_collect tests ---

func TestBGCollect_Success(t *testing.T) {
	d := &mockDispatcher{
		results: []agent.BackgroundTaskResult{
			{ID: "t1", Description: "done", Status: agent.BackgroundTaskStatusCompleted, Answer: "the answer"},
		},
	}
	ctx := ctxWithDispatcher(d)
	tool := NewBGCollect()

	result, err := tool.Execute(ctx, ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "the answer") {
		t.Errorf("expected 'the answer' in output: %s", result.Content)
	}
}

func TestBGCollect_WithWait(t *testing.T) {
	d := &mockDispatcher{
		results: []agent.BackgroundTaskResult{
			{ID: "t1", Description: "done", Status: agent.BackgroundTaskStatusCompleted},
		},
	}
	ctx := ctxWithDispatcher(d)
	tool := NewBGCollect()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "c",
		Arguments: map[string]any{
			"task_ids":        []any{"t1"},
			"wait":            true,
			"timeout_seconds": float64(5),
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
}

func TestBGCollect_NoTasks(t *testing.T) {
	d := &mockDispatcher{results: nil}
	ctx := ctxWithDispatcher(d)
	tool := NewBGCollect()

	result, err := tool.Execute(ctx, ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "No background tasks") {
		t.Errorf("expected 'No background tasks' message, got: %s", result.Content)
	}
}

func TestBGCollect_NoDispatcher(t *testing.T) {
	tool := NewBGCollect()
	result, err := tool.Execute(context.Background(), ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error when dispatcher not available")
	}
}

func TestBGCollect_FailedResult(t *testing.T) {
	d := &mockDispatcher{
		results: []agent.BackgroundTaskResult{
			{ID: "f1", Description: "failed task", Status: agent.BackgroundTaskStatusFailed, Error: "timeout"},
		},
	}
	ctx := ctxWithDispatcher(d)
	tool := NewBGCollect()

	result, err := tool.Execute(ctx, ports.ToolCall{ID: "c", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(result.Content, "timeout") {
		t.Errorf("expected error message in output: %s", result.Content)
	}
	if !strings.Contains(result.Content, "1 failed/cancelled") {
		t.Errorf("expected failure count in output: %s", result.Content)
	}
}

// --- ext_reply tests ---

func TestExtReply_Success(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewExtReply()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "c",
		Arguments: map[string]any{
			"task_id":    "t1",
			"request_id": "r1",
			"approved":   true,
			"message":    "ok",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if len(d.replyCalls) != 1 {
		t.Fatalf("expected 1 reply call, got %d", len(d.replyCalls))
	}
	if d.replyCalls[0].TaskID != "t1" || d.replyCalls[0].RequestID != "r1" || !d.replyCalls[0].Approved {
		t.Fatalf("unexpected reply payload: %+v", d.replyCalls[0])
	}
}

// --- ext_merge tests ---

func TestExtMerge_Success(t *testing.T) {
	d := &mockDispatcher{
		mergeResult: &agent.MergeResult{
			TaskID:       "t1",
			Strategy:     agent.MergeStrategyAuto,
			Success:      true,
			FilesChanged: []string{"file1.go"},
		},
	}
	ctx := ctxWithDispatcher(d)
	tool := NewExtMerge()

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID: "c",
		Arguments: map[string]any{
			"task_id": "t1",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("unexpected result error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "Merge successful") {
		t.Fatalf("expected merge success output, got: %s", result.Content)
	}
}
