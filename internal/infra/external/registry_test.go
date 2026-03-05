package external

import (
	"context"
	"errors"
	"reflect"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
)

type stubExternalExecutor struct {
	supported []string
	run       func(context.Context, agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error)
	calls     []agent.ExternalAgentRequest
}

func (s *stubExternalExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	s.calls = append(s.calls, req)
	if s.run != nil {
		return s.run(ctx, req)
	}
	return &agent.ExternalAgentResult{Answer: "ok"}, nil
}

func (s *stubExternalExecutor) SupportedTypes() []string {
	return append([]string(nil), s.supported...)
}

func TestBuildAgentFallbackChain(t *testing.T) {
	t.Parallel()

	chain := buildAgentFallbackChain("codex", map[string]string{
		"fallback_clis": " codex , claude-code , kimi, claude_code ",
	})
	want := []string{"codex", "claude_code", "kimi"}
	if !reflect.DeepEqual(chain, want) {
		t.Fatalf("fallback chain mismatch: got %v want %v", chain, want)
	}
}

func TestRegistryExecute_UsesFallbackCLIs(t *testing.T) {
	t.Parallel()

	primary := &stubExternalExecutor{
		supported: []string{agent.AgentTypeCodex},
		run: func(context.Context, agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
			return nil, errors.New("primary unavailable")
		},
	}
	fallback := &stubExternalExecutor{
		supported: []string{agent.AgentTypeClaudeCode},
		run: func(context.Context, agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
			return &agent.ExternalAgentResult{Answer: "fallback answer"}, nil
		},
	}

	registry := &Registry{
		executors: map[string]agent.ExternalAgentExecutor{
			agent.AgentTypeCodex:      primary,
			agent.AgentTypeClaudeCode: fallback,
		},
		logger: logging.Nop(),
	}

	result, err := registry.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "team-worker-1",
		AgentType: "codex",
		Config: map[string]string{
			"fallback_clis": "claude_code,kimi",
		},
	})
	if err != nil {
		t.Fatalf("execute with fallback: %v", err)
	}
	if result == nil || result.Answer != "fallback answer" {
		t.Fatalf("unexpected result: %#v", result)
	}
	if len(primary.calls) != 1 {
		t.Fatalf("expected primary called once, got %d", len(primary.calls))
	}
	if len(fallback.calls) != 1 {
		t.Fatalf("expected fallback called once, got %d", len(fallback.calls))
	}
	if result.Metadata["fallback_used"] != true {
		t.Fatalf("expected fallback_used=true metadata, got %#v", result.Metadata)
	}
	if result.Metadata["fallback_from_agent_type"] != agent.AgentTypeCodex {
		t.Fatalf("unexpected fallback_from_agent_type: %#v", result.Metadata)
	}
	if result.Metadata["fallback_to_agent_type"] != agent.AgentTypeClaudeCode {
		t.Fatalf("unexpected fallback_to_agent_type: %#v", result.Metadata)
	}
	if result.Metadata["fallback_attempt"] != 2 {
		t.Fatalf("unexpected fallback_attempt metadata: %#v", result.Metadata)
	}
}

func TestRegistryExecute_ContextCanceledStopsFallback(t *testing.T) {
	t.Parallel()

	primary := &stubExternalExecutor{
		supported: []string{agent.AgentTypeCodex},
		run: func(context.Context, agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
			return nil, context.Canceled
		},
	}
	fallback := &stubExternalExecutor{
		supported: []string{agent.AgentTypeClaudeCode},
	}
	registry := &Registry{
		executors: map[string]agent.ExternalAgentExecutor{
			agent.AgentTypeCodex:      primary,
			agent.AgentTypeClaudeCode: fallback,
		},
		logger: logging.Nop(),
	}

	_, err := registry.Execute(context.Background(), agent.ExternalAgentRequest{
		TaskID:    "team-worker-2",
		AgentType: "codex",
		Config: map[string]string{
			"fallback_clis": "claude_code",
		},
	})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}
	if len(primary.calls) != 1 {
		t.Fatalf("expected primary called once, got %d", len(primary.calls))
	}
	if len(fallback.calls) != 0 {
		t.Fatalf("expected no fallback call when context canceled, got %d", len(fallback.calls))
	}
}
