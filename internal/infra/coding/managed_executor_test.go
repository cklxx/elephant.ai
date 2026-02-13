package coding

import (
	"context"
	"errors"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
)

type stubExternalExecutor struct {
	results []*agent.ExternalAgentResult
	errs    []error
	reqs    []agent.ExternalAgentRequest
}

func (s *stubExternalExecutor) Execute(_ context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	s.reqs = append(s.reqs, req)
	idx := len(s.reqs) - 1
	var result *agent.ExternalAgentResult
	if idx < len(s.results) {
		result = s.results[idx]
	}
	var err error
	if idx < len(s.errs) {
		err = s.errs[idx]
	}
	return result, err
}

func (s *stubExternalExecutor) SupportedTypes() []string {
	return []string{"codex", "claude_code"}
}

type stubRunner struct {
	errByCommand map[string]error
	calls        []string
}

func (s *stubRunner) Run(_ context.Context, _ string, command string) (string, error) {
	s.calls = append(s.calls, command)
	if s.errByCommand != nil {
		if err, ok := s.errByCommand[command]; ok {
			return "", err
		}
	}
	return "ok", nil
}

func TestManagedExternalExecutor_PassThroughForNonCoding(t *testing.T) {
	base := &stubExternalExecutor{
		results: []*agent.ExternalAgentResult{{Answer: "done"}},
	}
	wrapped := NewManagedExternalExecutor(base, nil)

	result, err := wrapped.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "task-1",
		AgentType: "codex",
		Prompt:    "hello",
		Config: map[string]string{
			"task_kind": "general",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Answer != "done" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(base.reqs) != 1 {
		t.Fatalf("expected 1 execute call, got %d", len(base.reqs))
	}
	if base.reqs[0].Config["approval_policy"] != "" {
		t.Fatalf("did not expect coding defaults for non-coding task, got %+v", base.reqs[0].Config)
	}
}

func TestManagedExternalExecutor_CodingDefaultsAndRetry(t *testing.T) {
	base := &stubExternalExecutor{
		results: []*agent.ExternalAgentResult{
			nil,
			{Answer: "fixed"},
		},
		errs: []error{
			errors.New("first execution failed"),
			nil,
		},
	}
	wrapped := NewManagedExternalExecutor(base, nil)
	managed, ok := wrapped.(*ManagedExternalExecutor)
	if !ok {
		t.Fatalf("expected managed executor type")
	}
	managed.runner = &stubRunner{}

	result, err := wrapped.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "task-2",
		AgentType: "codex",
		Prompt:    "implement feature",
		Config: map[string]string{
			"task_kind": "coding",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Answer != "fixed" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(base.reqs) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(base.reqs))
	}
	firstCfg := base.reqs[0].Config
	if firstCfg["approval_policy"] != "never" || firstCfg["sandbox"] != "danger-full-access" {
		t.Fatalf("expected full-access codex defaults, got %+v", firstCfg)
	}
	if firstCfg["verify"] != "true" {
		t.Fatalf("expected verify=true default, got %+v", firstCfg)
	}
	if firstCfg["retry_max_attempts"] != "3" {
		t.Fatalf("expected retry_max_attempts=3, got %+v", firstCfg)
	}
	if !strings.Contains(base.reqs[1].Prompt, "[Retry Context]") {
		t.Fatalf("expected retry prompt enrichment, got %q", base.reqs[1].Prompt)
	}
}

func TestManagedExternalExecutor_VerifyFailureUsesRetryLimit(t *testing.T) {
	base := &stubExternalExecutor{
		results: []*agent.ExternalAgentResult{
			{Answer: "attempt-1"},
			{Answer: "attempt-2"},
		},
	}
	wrapped := NewManagedExternalExecutor(base, nil)
	managed := wrapped.(*ManagedExternalExecutor)
	managed.runner = &stubRunner{
		errByCommand: map[string]error{
			defaultVerifyLint: errors.New("lint failed"),
		},
	}

	_, err := wrapped.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "task-3",
		AgentType: "claude_code",
		Prompt:    "update code",
		Config: map[string]string{
			"task_kind":          "coding",
			"retry_max_attempts": "2",
		},
	})
	if err == nil {
		t.Fatal("expected error after verify retries exhausted")
	}
	if len(base.reqs) != 2 {
		t.Fatalf("expected 2 attempts, got %d", len(base.reqs))
	}
	if !strings.Contains(err.Error(), "verification failed") {
		t.Fatalf("expected verification failure error, got %v", err)
	}
}

func TestManagedExternalExecutor_CodingVerifyDisabledSkipsVerification(t *testing.T) {
	base := &stubExternalExecutor{
		results: []*agent.ExternalAgentResult{
			{Answer: "done"},
		},
	}
	wrapped := NewManagedExternalExecutor(base, nil)
	managed := wrapped.(*ManagedExternalExecutor)
	runner := &stubRunner{}
	managed.runner = runner

	result, err := wrapped.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "task-4",
		AgentType: "codex",
		Prompt:    "update code",
		Config: map[string]string{
			"task_kind": "coding",
			"verify":    "false",
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil || result.Answer != "done" {
		t.Fatalf("unexpected result: %+v", result)
	}
	if len(base.reqs) != 1 {
		t.Fatalf("expected one execution attempt, got %d", len(base.reqs))
	}
	if len(runner.calls) != 0 {
		t.Fatalf("expected no verification commands when verify=false, got %v", runner.calls)
	}
}

func TestManagedExternalExecutor_PlanModeForCodexUsesReadOnlyDefaults(t *testing.T) {
	base := &stubExternalExecutor{
		results: []*agent.ExternalAgentResult{
			{Answer: "plan output"},
		},
	}
	wrapped := NewManagedExternalExecutor(base, nil)

	result, err := wrapped.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:        "task-plan-1",
		AgentType:     "codex",
		Prompt:        "design rollout",
		ExecutionMode: "plan",
		AutonomyLevel: "full",
		Config:        map[string]string{"task_kind": "general"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("expected result")
	}
	if len(base.reqs) != 1 {
		t.Fatalf("expected 1 execution call, got %d", len(base.reqs))
	}
	cfg := base.reqs[0].Config
	if cfg["sandbox"] != "read-only" {
		t.Fatalf("expected sandbox=read-only, got %+v", cfg)
	}
	if cfg["approval_policy"] != "never" {
		t.Fatalf("expected approval_policy=never, got %+v", cfg)
	}
	if !strings.Contains(base.reqs[0].Prompt, "[Plan Mode]") {
		t.Fatalf("expected plan prompt enrichment, got %q", base.reqs[0].Prompt)
	}
	if result.Metadata["execution_mode"] != "plan" {
		t.Fatalf("expected metadata execution_mode=plan, got %+v", result.Metadata)
	}
	if result.Metadata["autonomy_level"] != "full" {
		t.Fatalf("expected metadata autonomy_level=full, got %+v", result.Metadata)
	}
}
