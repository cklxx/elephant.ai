package toolregistry

import (
	"context"
	"errors"
	"fmt"
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

// infraFailTool returns Go-level errors (infrastructure failures) that
// should trip the circuit breaker.
type infraFailTool struct {
	attempts  int
	failUntil int
}

func (t *infraFailTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.attempts++
	if t.attempts <= t.failUntil {
		return nil, &alexerrors.TransientError{Err: errors.New("connection refused")}
	}
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}

func (t *infraFailTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "infra_fail_tool"}
}

func (t *infraFailTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "infra_fail_tool"}
}

// appFailTool returns ToolResult.Error (application-level failures like
// "exit status 1") that should NOT trip the circuit breaker.
type appFailTool struct {
	attempts int
	failAll  bool
}

func (t *appFailTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	t.attempts++
	if t.failAll {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  errors.New("exit status 1"),
		}, nil
	}
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}

func (t *appFailTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: "app_fail_tool"}
}

func (t *appFailTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{Name: "app_fail_tool"}
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
	// Infrastructure errors (Go-level) should trip the circuit breaker.
	tool := &infraFailTool{failUntil: 5}
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

	result, err := executor.Execute(context.Background(), ports.ToolCall{ID: "call-2", Name: "infra_fail_tool"})
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

func TestRetryExecutorToolResultErrorDoesNotTripBreaker(t *testing.T) {
	// Application-level errors (ToolResult.Error like "exit status 1")
	// should NOT trip the circuit breaker. The LLM should see the error
	// and adapt; the breaker should remain closed.
	tool := &appFailTool{failAll: true}
	policyCfg := toolspolicy.ToolPolicyConfig{
		Retry: toolspolicy.ToolRetryConfig{
			MaxRetries: 0, // no retries — one call per Execute
		},
	}
	breakers := newCircuitBreakerStore(CircuitBreakerConfig{
		FailureThreshold: 2, // would trip after 2 infra failures
		SuccessThreshold: 1,
		Timeout:          time.Minute,
	})
	executor := newRetryExecutor(tool, toolspolicy.NewToolPolicy(policyCfg), breakers)

	// Call 10 times — all return ToolResult.Error but no Go error.
	for i := 0; i < 10; i++ {
		result, _ := executor.Execute(context.Background(), ports.ToolCall{
			ID: fmt.Sprintf("call-%d", i), Name: "app_fail_tool",
		})
		if result == nil || result.Error == nil {
			t.Fatalf("call %d: expected ToolResult.Error, got %+v", i, result)
		}
		// Crucially: the error should be the original "exit status 1",
		// NOT a circuit breaker "temporarily unavailable" error.
		if strings.Contains(result.Error.Error(), "temporarily unavailable") {
			t.Fatalf("call %d: circuit breaker tripped on application-level error", i)
		}
	}
	if tool.attempts != 10 {
		t.Fatalf("expected 10 attempts (breaker should stay closed), got %d", tool.attempts)
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

// safetyLevelTool is a stub with explicit SafetyLevel for policy tests.
type safetyLevelTool struct {
	meta ports.ToolMetadata
}

func (t *safetyLevelTool) Execute(_ context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	return &ports.ToolResult{CallID: call.ID, Content: "ok"}, nil
}
func (t *safetyLevelTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{Name: t.meta.Name}
}
func (t *safetyLevelTool) Metadata() ports.ToolMetadata { return t.meta }

func TestRetryExecutorPropagatesSafetyLevelToPolicy(t *testing.T) {
	// Create an L4 (irreversible) tool and use default policy rules which set
	// MaxRetries=0 for L4 tools. Before the fix, SafetyLevel was never
	// populated, so the l4-irreversible rule would never match.
	tool := &safetyLevelTool{meta: ports.ToolMetadata{
		Name:        "dangerous_delete",
		SafetyLevel: ports.SafetyLevelIrreversible,
	}}
	policyCfg := toolspolicy.DefaultToolPolicyConfigWithRules()
	// Override global retry to verify the rule overrides it
	policyCfg.Retry.MaxRetries = 5

	executor := newRetryExecutor(tool, toolspolicy.NewToolPolicy(policyCfg), nil)
	re := executor.(*retryExecutor)
	resolved := re.resolvePolicy(context.Background(), ports.ToolCall{ID: "c1", Name: "dangerous_delete"})

	if resolved.Retry.MaxRetries != 0 {
		t.Fatalf("expected L4 tool to get MaxRetries=0 from l4-irreversible rule, got %d", resolved.Retry.MaxRetries)
	}
	if resolved.SafetyLevel != ports.SafetyLevelIrreversible {
		t.Fatalf("expected SafetyLevel=%d, got %d", ports.SafetyLevelIrreversible, resolved.SafetyLevel)
	}
}

func TestRetryExecutorSafetyLevelFallsBackFromDangerous(t *testing.T) {
	// When SafetyLevel is unset but Dangerous=true, EffectiveSafetyLevel
	// should return L3, and the l3-high-impact rule should match.
	tool := &safetyLevelTool{meta: ports.ToolMetadata{
		Name:      "risky_write",
		Dangerous: true,
	}}
	policyCfg := toolspolicy.DefaultToolPolicyConfigWithRules()
	policyCfg.Retry.MaxRetries = 5

	executor := newRetryExecutor(tool, toolspolicy.NewToolPolicy(policyCfg), nil)
	re := executor.(*retryExecutor)
	resolved := re.resolvePolicy(context.Background(), ports.ToolCall{ID: "c2", Name: "risky_write"})

	if resolved.Retry.MaxRetries != 0 {
		t.Fatalf("expected L3 tool to get MaxRetries=0 from l3-high-impact rule, got %d", resolved.Retry.MaxRetries)
	}
	if resolved.SafetyLevel != ports.SafetyLevelHighImpact {
		t.Fatalf("expected SafetyLevel=%d, got %d", ports.SafetyLevelHighImpact, resolved.SafetyLevel)
	}
}

var _ tools.ToolExecutor = (*retryStubTool)(nil)
var _ tools.ToolExecutor = (*infraFailTool)(nil)
var _ tools.ToolExecutor = (*appFailTool)(nil)
var _ tools.ToolExecutor = (*safetyLevelTool)(nil)
