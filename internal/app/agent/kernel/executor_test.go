package kernel

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// mockExecutor records calls for testing.
type mockExecutor struct {
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

func TestMockExecutor_RecordsCalls(t *testing.T) {
	exec := &mockExecutor{}
	tid, err := exec.Execute(context.Background(), "agent-1", "do stuff", map[string]string{"k": "v"})
	if err != nil {
		t.Fatalf("Execute: %v", err)
	}
	if !strings.HasPrefix(tid, "task-") {
		t.Errorf("unexpected task ID: %s", tid)
	}
	if len(exec.calls) != 1 {
		t.Fatalf("expected 1 call, got %d", len(exec.calls))
	}
	if exec.calls[0].AgentID != "agent-1" {
		t.Errorf("wrong agent: %s", exec.calls[0].AgentID)
	}
}
