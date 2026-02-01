package shadow

import (
	"context"
	"testing"

	"alex/internal/coding"
)

type stubGateway struct {
	lastReq coding.TaskRequest
	result  *coding.TaskResult
	err     error
}

func (s *stubGateway) Submit(_ context.Context, req coding.TaskRequest) (*coding.TaskResult, error) {
	s.lastReq = req
	return s.result, s.err
}

func (s *stubGateway) Stream(_ context.Context, req coding.TaskRequest, _ coding.ProgressCallback) (*coding.TaskResult, error) {
	s.lastReq = req
	return s.result, s.err
}

func (s *stubGateway) Cancel(_ context.Context, _ string) error { return coding.ErrNotSupported }

func (s *stubGateway) Status(_ context.Context, _ string) (coding.TaskStatus, error) {
	return coding.TaskStatus{}, coding.ErrNotSupported
}

func TestAgentRunRequiresApproval(t *testing.T) {
	gateway := &stubGateway{result: &coding.TaskResult{TaskID: "t1", Answer: "ok"}}
	agent := NewAgent(Config{}, gateway, nil, nil)
	_, err := agent.Run(context.Background(), Task{ID: "t1", Prompt: "fix"})
	if err == nil {
		t.Fatal("expected approval error")
	}
}

func TestAgentRunDispatchesToGateway(t *testing.T) {
	gateway := &stubGateway{result: &coding.TaskResult{TaskID: "t1", Answer: "ok"}}
	approver := &stubApprover{approved: true}
	agent := NewAgent(Config{DefaultAgentType: "codex"}, gateway, approver, nil)

	result, err := agent.Run(context.Background(), Task{ID: "t1", Prompt: "fix"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Answer != "ok" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if gateway.lastReq.AgentType != "codex" {
		t.Fatalf("expected default agent type, got %q", gateway.lastReq.AgentType)
	}
}
