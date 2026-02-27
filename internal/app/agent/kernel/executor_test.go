package kernel

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"testing"

	kerneldomain "alex/internal/domain/kernel"
)

// mockExecutor records calls for testing. Thread-safe via mutex.
type mockExecutor struct {
	mu        sync.Mutex
	calls     []executorCall
	teamCalls []kerneldomain.TeamDispatchSpec
	err       error
	taskIDs   []string // returned in order
	summaries []string // returned in order
	idx       int
}

type executorCall struct {
	AgentID string
	Prompt  string
	Meta    map[string]string
}

func (m *mockExecutor) Execute(_ context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.calls = append(m.calls, executorCall{AgentID: agentID, Prompt: prompt, Meta: meta})
	if m.err != nil {
		return ExecutionResult{}, m.err
	}
	taskID := fmt.Sprintf("task-%d", m.idx)
	if m.idx < len(m.taskIDs) {
		taskID = m.taskIDs[m.idx]
	}
	summary := ""
	if m.idx < len(m.summaries) {
		summary = m.summaries[m.idx]
	}
	m.idx++
	return ExecutionResult{TaskID: taskID, Summary: summary}, nil
}

func (m *mockExecutor) ExecuteTeam(_ context.Context, spec kerneldomain.TeamDispatchSpec, _ map[string]string) (ExecutionResult, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.teamCalls = append(m.teamCalls, spec)
	if m.err != nil {
		return ExecutionResult{}, m.err
	}
	taskID := fmt.Sprintf("team-task-%d", m.idx)
	if m.idx < len(m.taskIDs) {
		taskID = m.taskIDs[m.idx]
	}
	summary := ""
	if m.idx < len(m.summaries) {
		summary = m.summaries[m.idx]
	}
	m.idx++
	return ExecutionResult{TaskID: taskID, Summary: summary}, nil
}

// callCount returns the number of recorded calls (thread-safe).
func (m *mockExecutor) callCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.calls)
}

func (m *mockExecutor) teamCallCount() int {
	m.mu.Lock()
	defer m.mu.Unlock()
	return len(m.teamCalls)
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
	if !strings.HasPrefix(tid.TaskID, "task-") {
		t.Errorf("unexpected task ID: %s", tid.TaskID)
	}
	calls := exec.getCalls()
	if len(calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(calls))
	}
	if calls[0].AgentID != "agent-1" {
		t.Errorf("wrong agent: %s", calls[0].AgentID)
	}
}

func TestBuildKernelTeamDispatchPrompt_ContainsRunTasksArguments(t *testing.T) {
	prompt := buildKernelTeamDispatchPrompt(kerneldomain.TeamDispatchSpec{
		Template:       "kimi_research",
		Goal:           "compare cache strategies",
		TimeoutSeconds: 222,
		Wait:           true,
		Prompts: map[string]string{
			"researcher": "focus on Redis",
		},
	})
	if !strings.Contains(prompt, "\"template\":\"kimi_research\"") {
		t.Fatalf("expected template in prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "\"goal\":\"compare cache strategies\"") {
		t.Fatalf("expected goal in prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "\"timeout_seconds\":222") {
		t.Fatalf("expected timeout in prompt: %q", prompt)
	}
	if !strings.Contains(prompt, "\"wait\":true") {
		t.Fatalf("expected wait in prompt: %q", prompt)
	}
}
