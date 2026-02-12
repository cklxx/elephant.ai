package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"testing"
	"time"

	agentports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	id "alex/internal/shared/utils/id"
)

// mockExecutor records calls for testing. Thread-safe via mutex.
type mockExecutor struct {
	mu      sync.Mutex
	calls   []executorCall
	err     error
	taskIDs []string // returned in order
	idx     int
}

type executorCall struct {
	AgentID string
	Prompt  string
	Meta    map[string]string
}

func (m *mockExecutor) Execute(_ context.Context, agentID, prompt string, meta map[string]string) (string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, executorCall{AgentID: agentID, Prompt: prompt, Meta: meta})
	if m.err != nil {
		return "", m.err
	}
	taskID := fmt.Sprintf("task-%d", m.idx)
	if m.idx < len(m.taskIDs) {
		taskID = m.taskIDs[m.idx]
	}
	m.idx++
	return taskID, nil
}

// callCount returns the number of recorded calls (thread-safe).
func (m *mockExecutor) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

// getCalls returns a copy of recorded calls (thread-safe).
func (m *mockExecutor) getCalls() []executorCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	out := make([]executorCall, len(m.calls))
	copy(out, m.calls)
	return out
}

func TestMockExecutor_RecordsCalls(t *testing.T) {
	exec := &mockExecutor{}
	tid, err := exec.Execute(context.Background(), "agent-1", "do stuff", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(tid, "task-") {
		t.Errorf("unexpected task ID: %s", tid)
	}
	calls := exec.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].AgentID != "agent-1" {
		t.Errorf("wrong agent: %s", calls[0].AgentID)
	}
}

type coordinatorCall struct {
	Task        string
	SessionID   string
	HasDeadline bool
	UserID      string
}

type mockCoordinator struct {
	mu     sync.Mutex
	result *agent.TaskResult
	err    error
	calls  []coordinatorCall
}

func (m *mockCoordinator) ExecuteTask(ctx context.Context, task string, sessionID string, _ agent.EventListener) (*agent.TaskResult, error) {
	_, hasDeadline := ctx.Deadline()
	m.mu.Lock()
	m.calls = append(m.calls, coordinatorCall{
		Task:        task,
		SessionID:   sessionID,
		HasDeadline: hasDeadline,
		UserID:      id.UserIDFromContext(ctx),
	})
	result := m.result
	err := m.err
	m.mu.Unlock()
	return result, err
}

func (m *mockCoordinator) lastCall() coordinatorCall {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.calls[len(m.calls)-1]
}

func TestCoordinatorExecutor_RequiresRealToolCall(t *testing.T) {
	coord := &mockCoordinator{
		result: &agent.TaskResult{
			Messages: []agentports.Message{
				{Role: "assistant", Content: "done"},
			},
		},
	}
	exec := NewCoordinatorExecutor(coord, 0)
	_, err := exec.Execute(context.Background(), "agent-a", "do", nil)
	if !errors.Is(err, errKernelNoRealToolAction) {
		t.Fatalf("expected errKernelNoRealToolAction, got %v", err)
	}
}

func TestCoordinatorExecutor_AllowsRealToolCall(t *testing.T) {
	coord := &mockCoordinator{
		result: &agent.TaskResult{
			Messages: []agentports.Message{
				{
					Role: "assistant",
					ToolCalls: []agentports.ToolCall{
						{ID: "call-1", Name: "shell_exec"},
					},
				},
				{
					Role: "tool",
					ToolResults: []agentports.ToolResult{
						{CallID: "call-1", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(coord, 0)
	taskID, err := exec.Execute(context.Background(), "agent-b", "do", nil)
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(taskID, "kernel-agent-b-") {
		t.Fatalf("unexpected task id: %s", taskID)
	}
}

func TestCoordinatorExecutor_RejectsFailedRealToolCall(t *testing.T) {
	coord := &mockCoordinator{
		result: &agent.TaskResult{
			Messages: []agentports.Message{
				{
					Role: "assistant",
					ToolCalls: []agentports.ToolCall{
						{ID: "call-1", Name: "shell_exec"},
					},
				},
				{
					Role: "tool",
					ToolResults: []agentports.ToolResult{
						{CallID: "call-1", Content: "failed", Error: errors.New("boom")},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(coord, 0)
	_, err := exec.Execute(context.Background(), "agent-f", "do", nil)
	if !errors.Is(err, errKernelNoRealToolAction) {
		t.Fatalf("expected errKernelNoRealToolAction, got %v", err)
	}
}

func TestCoordinatorExecutor_AllowsUnknownSuccessfulToolResult(t *testing.T) {
	coord := &mockCoordinator{
		result: &agent.TaskResult{
			Messages: []agentports.Message{
				{
					Role: "tool",
					ToolResults: []agentports.ToolResult{
						{CallID: "unknown-call", Content: "ok"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(coord, 0)
	_, err := exec.Execute(context.Background(), "agent-u", "do", nil)
	if err != nil {
		t.Fatalf("expected success for unknown successful tool result, got %v", err)
	}
}

func TestCoordinatorExecutor_RejectsOrchestrationOnlyCalls(t *testing.T) {
	coord := &mockCoordinator{
		result: &agent.TaskResult{
			Messages: []agentports.Message{
				{
					Role: "assistant",
					ToolCalls: []agentports.ToolCall{
						{ID: "call-1", Name: "plan"},
						{ID: "call-2", Name: "clarify"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(coord, 0)
	_, err := exec.Execute(context.Background(), "agent-c", "do", nil)
	if !errors.Is(err, errKernelNoRealToolAction) {
		t.Fatalf("expected errKernelNoRealToolAction, got %v", err)
	}
}

func TestCoordinatorExecutor_PropagatesContextMetadata(t *testing.T) {
	coord := &mockCoordinator{
		result: &agent.TaskResult{
			Messages: []agentports.Message{
				{
					Role: "assistant",
					ToolCalls: []agentports.ToolCall{
						{ID: "call-1", Name: "shell_exec"},
					},
				},
			},
		},
	}
	exec := NewCoordinatorExecutor(coord, 3*time.Second)
	_, err := exec.Execute(context.Background(), "agent-d", "do", map[string]string{"user_id": "cklxx"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	call := coord.lastCall()
	if call.UserID != "cklxx" {
		t.Fatalf("expected propagated user_id=cklxx, got %q", call.UserID)
	}
	if !call.HasDeadline {
		t.Fatalf("expected timeout deadline to be set on context")
	}
	if !strings.HasPrefix(call.SessionID, "kernel-agent-d-") {
		t.Fatalf("unexpected session id: %s", call.SessionID)
	}
}
