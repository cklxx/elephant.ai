package orchestration

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
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
			"task_id":     "t1",
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
	if d.dispatched[0].Req.TaskID != "t1" {
		t.Errorf("expected task_id=t1, got %s", d.dispatched[0].Req.TaskID)
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
			"task_id":     "ext-1",
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

func TestBGDispatch_MissingRequired(t *testing.T) {
	d := &mockDispatcher{}
	ctx := ctxWithDispatcher(d)
	tool := NewBGDispatch()

	tests := []struct {
		name string
		args map[string]any
	}{
		{"missing task_id", map[string]any{"description": "d", "prompt": "p"}},
		{"missing description", map[string]any{"task_id": "t", "prompt": "p"}},
		{"missing prompt", map[string]any{"task_id": "t", "description": "d"}},
		{"empty task_id", map[string]any{"task_id": "", "description": "d", "prompt": "p"}},
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

func TestBGDispatch_NoDispatcher(t *testing.T) {
	tool := NewBGDispatch()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "c",
		Arguments: map[string]any{
			"task_id":     "t",
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
		Arguments: map[string]any{"task_id": "t", "description": "d", "prompt": "p"},
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
		Arguments: map[string]any{"task_id": "t", "description": "d", "prompt": "p", "unknown": "x"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatal("expected error for unsupported parameter")
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
