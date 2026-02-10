package toolregistry

import (
	"context"
	"fmt"
	"math"
	"math/rand"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	ports "alex/internal/domain/agent/ports"
	tools "alex/internal/domain/agent/ports/tools"
	toolspolicy "alex/internal/infra/tools"
	alexerrors "alex/internal/shared/errors"
)

const retryJitterFactor = 0.25

// CircuitBreakerConfig configures circuit breaker behavior for tool execution.
type CircuitBreakerConfig = alexerrors.CircuitBreakerConfig

func normalizeCircuitBreakerConfig(cfg CircuitBreakerConfig) CircuitBreakerConfig {
	defaults := alexerrors.DefaultCircuitBreakerConfig()
	if cfg.FailureThreshold <= 0 {
		cfg.FailureThreshold = defaults.FailureThreshold
	}
	if cfg.SuccessThreshold <= 0 {
		cfg.SuccessThreshold = defaults.SuccessThreshold
	}
	if cfg.Timeout <= 0 {
		cfg.Timeout = defaults.Timeout
	}
	return cfg
}

type retryExecutor struct {
	delegate tools.ToolExecutor
	policy   toolspolicy.ToolPolicy
	breaker  *alexerrors.CircuitBreaker
}

type circuitBreakerStore struct {
	manager *alexerrors.CircuitBreakerManager
	config  CircuitBreakerConfig
}

func newCircuitBreakerStore(cfg CircuitBreakerConfig) *circuitBreakerStore {
	normalized := normalizeCircuitBreakerConfig(cfg)
	return &circuitBreakerStore{
		manager: alexerrors.NewCircuitBreakerManager(normalized),
		config:  normalized,
	}
}

func (s *circuitBreakerStore) Get(name string) *alexerrors.CircuitBreaker {
	if s == nil || strings.TrimSpace(name) == "" {
		return nil
	}
	if s.manager == nil {
		s.manager = alexerrors.NewCircuitBreakerManager(normalizeCircuitBreakerConfig(s.config))
	}
	return s.manager.Get(name)
}

func newRetryExecutor(delegate tools.ToolExecutor, policy toolspolicy.ToolPolicy, breakers *circuitBreakerStore) tools.ToolExecutor {
	if delegate == nil {
		return delegate
	}
	name := strings.TrimSpace(delegate.Metadata().Name)
	if name == "" {
		name = strings.TrimSpace(delegate.Definition().Name)
	}
	if name == "" {
		name = "tool"
	}
	var breaker *alexerrors.CircuitBreaker
	if breakers != nil {
		breaker = breakers.Get("tool-" + name)
	}
	return &retryExecutor{
		delegate: delegate,
		policy:   policy,
		breaker:  breaker,
	}
}

func (r *retryExecutor) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if r == nil || r.delegate == nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("tool executor missing")}, nil
	}

	resolved := r.resolvePolicy(ctx, call)
	policyWarnAllow := false
	if !resolved.Enabled {
		if strings.EqualFold(resolved.EnforcementMode, "warn_allow") {
			policyWarnAllow = true
		} else {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("tool denied by policy: %s", call.Name)}, nil
		}
	}

	retryCfg := normalizeRetryConfig(resolved.Retry)
	var lastErr error
	var lastResult *ports.ToolResult

	for attempt := 0; attempt <= retryCfg.MaxRetries; attempt++ {
		if ctx.Err() != nil {
			lastErr = ctx.Err()
			break
		}

		result, err := r.executeOnce(ctx, call, resolved.Timeout)
		if err == nil {
			if policyWarnAllow {
				result = annotatePolicyWarnAllow(result, call)
			}
			return result, nil
		}
		lastResult = result
		lastErr = err

		if !alexerrors.IsTransient(err) {
			break
		}
		if attempt >= retryCfg.MaxRetries {
			break
		}

		delay := calculateRetryBackoff(attempt, retryCfg)
		if delay <= 0 {
			continue
		}
		timer := time.NewTimer(delay)
		select {
		case <-timer.C:
		case <-ctx.Done():
			timer.Stop()
			lastErr = ctx.Err()
			attempt = retryCfg.MaxRetries
		}
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("tool execution failed")
	}
	if lastResult == nil {
		result := &ports.ToolResult{CallID: call.ID, Error: lastErr}
		if policyWarnAllow {
			result = annotatePolicyWarnAllow(result, call)
		}
		return result, nil
	}
	if lastResult.Error == nil {
		lastResult.Error = lastErr
	}
	if policyWarnAllow {
		lastResult = annotatePolicyWarnAllow(lastResult, call)
	}
	return lastResult, nil
}

func (r *retryExecutor) executeOnce(ctx context.Context, call ports.ToolCall, timeout time.Duration) (*ports.ToolResult, error) {
	execCtx := ctx
	var cancel context.CancelFunc
	if timeout > 0 {
		execCtx, cancel = context.WithTimeout(ctx, timeout)
	}
	if cancel != nil {
		defer cancel()
	}

	var result *ports.ToolResult

	// Only infrastructure errors (Go error return from delegate.Execute)
	// should affect the circuit breaker. Application-level errors stored in
	// ToolResult.Error (e.g. "exit status 1" from shell_exec) are normal
	// tool output that the LLM should see and adapt to — they must NOT
	// count toward the breaker failure threshold.
	exec := func(inner context.Context) error {
		res, err := r.delegate.Execute(inner, call)
		result = res
		return err // only infra errors reach the breaker
	}

	var execErr error
	if r.breaker != nil {
		execErr = r.breaker.Execute(execCtx, exec)
	} else {
		execErr = exec(execCtx)
	}
	if execErr != nil {
		return result, execErr
	}

	// Promote ToolResult.Error to Go error for the retry layer (the
	// circuit breaker has already recorded this call as a success).
	if result == nil {
		return nil, fmt.Errorf("tool %s returned no result", call.Name)
	}
	if result.Error != nil {
		return result, result.Error
	}
	return result, nil
}

func (r *retryExecutor) resolvePolicy(ctx context.Context, call ports.ToolCall) toolspolicy.ResolvedPolicy {
	if r.policy == nil {
		return toolspolicy.ResolvedPolicy{Enabled: true}
	}
	meta := r.delegate.Metadata()
	name := strings.TrimSpace(meta.Name)
	if name == "" {
		name = strings.TrimSpace(call.Name)
	}
	if name == "" {
		name = strings.TrimSpace(r.delegate.Definition().Name)
	}
	channel := strings.TrimSpace(appcontext.ChannelFromContext(ctx))
	return r.policy.Resolve(toolspolicy.ToolCallContext{
		ToolName:    name,
		Category:    meta.Category,
		Tags:        meta.Tags,
		Dangerous:   meta.Dangerous,
		Channel:     channel,
		SafetyLevel: meta.EffectiveSafetyLevel(),
	})
}

func (r *retryExecutor) Definition() ports.ToolDefinition {
	return r.delegate.Definition()
}

func (r *retryExecutor) Metadata() ports.ToolMetadata {
	return r.delegate.Metadata()
}

func normalizeRetryConfig(cfg toolspolicy.ToolRetryConfig) toolspolicy.ToolRetryConfig {
	if cfg.MaxRetries < 0 {
		cfg.MaxRetries = 0
	}
	if cfg.InitialBackoff <= 0 {
		cfg.InitialBackoff = time.Second
	}
	if cfg.MaxBackoff <= 0 {
		cfg.MaxBackoff = cfg.InitialBackoff
	}
	if cfg.BackoffFactor <= 0 {
		cfg.BackoffFactor = 2
	}
	return cfg
}

func calculateRetryBackoff(attempt int, cfg toolspolicy.ToolRetryConfig) time.Duration {
	if attempt < 0 {
		attempt = 0
	}
	delay := float64(cfg.InitialBackoff) * math.Pow(cfg.BackoffFactor, float64(attempt))
	if max := float64(cfg.MaxBackoff); max > 0 && delay > max {
		delay = max
	}
	if cfg.InitialBackoff <= 0 {
		return 0
	}
	if retryJitterFactor > 0 {
		jitter := delay * retryJitterFactor
		delay += (rand.Float64()*2 - 1) * jitter
		if delay < 0 {
			delay = 0
		}
	}
	return time.Duration(delay)
}

func annotatePolicyWarnAllow(result *ports.ToolResult, call ports.ToolCall) *ports.ToolResult {
	if result == nil {
		result = &ports.ToolResult{}
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	result.Metadata["policy_enforcement"] = "warn_allow"
	if _, exists := result.Metadata["policy_warning"]; !exists {
		result.Metadata["policy_warning"] = fmt.Sprintf("tool policy denied %s but mode=warn_allow permitted execution", strings.TrimSpace(call.Name))
	}
	return result
}

var _ tools.ToolExecutor = (*retryExecutor)(nil)
