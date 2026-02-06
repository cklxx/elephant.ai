package toolregistry

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
	alexerrors "alex/internal/shared/errors"
)

type retryStubTool struct {
	attempts  int
	failUntil int
}

func (t *retryStubTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.attempts++
	if t.attempts <= t.failUntil {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  &alexerrors.TransientError{Err: errors.New("transient")},
		}, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}

func (t *retryStubTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "retry_tool"}
}

func (t *retryStubTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "retry_tool"}
}

type timeoutProbeTool struct {
	sawDeadline bool
}

func (t *timeoutProbeTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	_, ok := ctx.Deadline()
	t.sawDeadline = ok
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}

func (t *timeoutProbeTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "timeout_tool"}
}

func (t *timeoutProbeTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "timeout_tool"}
}

func TestRetryExecutorRetriesTransientErrors(t *testing.T) {
	tool := &retryStubTool{failUntil: 2}
	policyCfg := toolspolicy.ToolPolicyConfig{
		Timeout: toolspolicy.ToolTimeoutConfig{Default: 50 * time.Millisecond},
		Retry: toolspolicy.ToolRetryConfig{
			MaxRetries:     2,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
			BackoffFactor:  1,
		},
	}
	breakers := newCircuitBreakerStore(CircuitBreakerConfig{
		FailureThreshold: 100,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	executor := newRetryExecutor(tool, toolspolicy.NewToolPolicy(policyCfg), breakers)

	result, err := executor.Execute(context.Background(), ports.ToolCall{ID: "call-1", Name: "retry_tool"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success result, got %+v", result)
	}
	if tool.attempts != 3 {
		t.Fatalf("expected 3 attempts, got %d", tool.attempts)
	}
}

func TestRetryExecutorCircuitBreakerStopsAfterOpen(t *testing.T) {
	tool := &retryStubTool{failUntil: 5}
	policyCfg := toolspolicy.ToolPolicyConfig{
		Retry: toolspolicy.ToolRetryConfig{
			MaxRetries:     1,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
			BackoffFactor:  1,
		},
	}
	breakers := newCircuitBreakerStore(CircuitBreakerConfig{
		FailureThreshold: 1,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	executor := newRetryExecutor(tool, toolspolicy.NewToolPolicy(policyCfg), breakers)

	result, err := executor.Execute(context.Background(), ports.ToolCall{ID: "call-2", Name: "retry_tool"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if tool.attempts != 1 {
		t.Fatalf("expected 1 attempt before circuit opened, got %d", tool.attempts)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected error result, got %+v", result)
	}
	if !strings.Contains(result.Error.Error(), "temporarily unavailable") {
		t.Fatalf("expected circuit breaker error, got %v", result.Error)
	}
}

func TestRetryExecutorAppliesTimeout(t *testing.T) {
	tool := &timeoutProbeTool{}
	policyCfg := toolspolicy.ToolPolicyConfig{
		Timeout: toolspolicy.ToolTimeoutConfig{Default: 25 * time.Millisecond},
		Retry: toolspolicy.ToolRetryConfig{
			MaxRetries:     0,
			InitialBackoff: time.Millisecond,
			MaxBackoff:     time.Millisecond,
			BackoffFactor:  1,
		},
	}
	breakers := newCircuitBreakerStore(CircuitBreakerConfig{
		FailureThreshold: 10,
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	executor := newRetryExecutor(tool, toolspolicy.NewToolPolicy(policyCfg), breakers)

	result, err := executor.Execute(context.Background(), ports.ToolCall{ID: "call-3", Name: "timeout_tool"})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected success result, got %+v", result)
	}
	if !tool.sawDeadline {
		t.Fatal("expected context deadline to be set for tool execution")
	}
}

var _ tools.ToolExecutor = (*retryStubTool)(nil)
