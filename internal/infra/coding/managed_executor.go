package coding

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

const (
	configTaskKind        = "task_kind"
	configCodingProfile   = "coding_profile"
	configRetryMaxAttempt = "retry_max_attempts"
	configMergeOnSuccess  = "merge_on_success"
	defaultRetryAttempts  = 3
	maxRetryAttempts      = 10
)

// ManagedExternalExecutor wraps an external executor with coding-task defaults:
// full-access agent config, verification, and bounded retry loops.
type ManagedExternalExecutor struct {
	base   agent.ExternalAgentExecutor
	inter  agent.InteractiveExternalExecutor
	logger logging.Logger
	runner CommandRunner
}

// NewManagedExternalExecutor wraps an external executor with coding-task policy.
func NewManagedExternalExecutor(base agent.ExternalAgentExecutor, logger logging.Logger) agent.ExternalAgentExecutor {
	if base == nil {
		return nil
	}
	wrapped := &ManagedExternalExecutor{
		base:   base,
		logger: logging.OrNop(logger),
	}
	if inter, ok := base.(agent.InteractiveExternalExecutor); ok {
		wrapped.inter = inter
	}
	return wrapped
}

// SupportedTypes returns the base executor supported types.
func (m *ManagedExternalExecutor) SupportedTypes() []string {
	if m == nil || m.base == nil {
		return nil
	}
	return m.base.SupportedTypes()
}

// InputRequests delegates interactive requests when supported.
func (m *ManagedExternalExecutor) InputRequests() <-chan agent.InputRequest {
	if m == nil || m.inter == nil {
		return nil
	}
	return m.inter.InputRequests()
}

// Reply delegates interactive replies when supported.
func (m *ManagedExternalExecutor) Reply(ctx context.Context, resp agent.InputResponse) error {
	if m == nil || m.inter == nil {
		return ErrNotSupported
	}
	return m.inter.Reply(ctx, resp)
}

// Execute runs external execution with coding policy when task_kind=coding.
func (m *ManagedExternalExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if m == nil || m.base == nil {
		return nil, fmt.Errorf("external executor is not configured")
	}

	cfg := applyExecutionControls(req.AgentType, req.ExecutionMode, req.AutonomyLevel, req.Config)
	effectiveMode := normalizeExecutionMode(req.ExecutionMode)
	effectiveAutonomy := normalizeAutonomyLevel(req.AutonomyLevel)
	if effectiveMode == executionModePlan {
		req.Prompt = buildPlanOnlyPrompt(req.Prompt)
	}

	if !isCodingTask(cfg) {
		req.Config = cfg
		res, err := m.base.Execute(ctx, req)
		if res != nil {
			if res.Metadata == nil {
				res.Metadata = make(map[string]any)
			}
			res.Metadata["execution_mode"] = effectiveMode
			res.Metadata["autonomy_level"] = effectiveAutonomy
		}
		return res, err
	}
	cfg = applyCodingDefaults(req.AgentType, cfg)

	retries := boundedRetryAttempts(cfg[configRetryMaxAttempt], defaultRetryAttempts)
	verifyPlan := ResolveVerificationPlan(cfg)

	basePrompt := strings.TrimSpace(req.Prompt)
	prompt := basePrompt

	var (
		lastErr    error
		lastResult *agent.ExternalAgentResult
		lastVerify *VerifyResult
	)
	for attempt := 1; attempt <= retries; attempt++ {
		runReq := req
		runReq.Prompt = prompt
		runReq.Config = core.CloneStringMap(cfg)

		result, execErr := m.base.Execute(ctx, runReq)
		lastResult = result
		if execErr != nil {
			lastErr = fmt.Errorf("attempt %d/%d execution failed: %w", attempt, retries, execErr)
		} else if result != nil && utils.HasContent(result.Error) {
			lastErr = fmt.Errorf("attempt %d/%d execution failed: %s", attempt, retries, strings.TrimSpace(result.Error))
		} else {
			workingDir := strings.TrimSpace(req.WorkingDir)
			if workingDir == "" {
				workingDir = "."
			}
			lastVerify = VerifyAll(ctx, workingDir, m.runner, verifyPlan)
			if verifyErr := VerifyError(lastVerify); verifyErr == nil {
				if result == nil {
					result = &agent.ExternalAgentResult{}
				}
				if result.Metadata == nil {
					result.Metadata = make(map[string]any)
				}
				result.Metadata["verify"] = lastVerify
				result.Metadata["retry_attempt"] = attempt
				result.Metadata["coding_profile"] = cfg[configCodingProfile]
				result.Metadata["execution_mode"] = effectiveMode
				result.Metadata["autonomy_level"] = effectiveAutonomy
				return result, nil
			} else {
				lastErr = fmt.Errorf("attempt %d/%d verification failed: %w", attempt, retries, verifyErr)
			}
		}

		if attempt < retries {
			prompt = buildRetryPrompt(basePrompt, lastErr, attempt)
			m.logger.Warn("coding task retrying attempt %d/%d for agent=%s task=%s: %v", attempt+1, retries, req.AgentType, req.TaskID, lastErr)
		}
	}

	if lastResult == nil {
		lastResult = &agent.ExternalAgentResult{}
	}
	lastResult.Error = strings.TrimSpace(errorString(lastErr))
	if lastResult.Metadata == nil {
		lastResult.Metadata = make(map[string]any)
	}
	lastResult.Metadata["retry_attempts"] = retries
	lastResult.Metadata["coding_profile"] = cfg[configCodingProfile]
	lastResult.Metadata["execution_mode"] = effectiveMode
	lastResult.Metadata["autonomy_level"] = effectiveAutonomy
	if lastVerify != nil {
		lastResult.Metadata["verify"] = lastVerify
	}

	if lastErr == nil {
		lastErr = fmt.Errorf("coding task failed after %d attempts", retries)
	}
	return lastResult, lastErr
}

func isCodingTask(config map[string]string) bool {
	if len(config) == 0 {
		return false
	}
	return strings.EqualFold(strings.TrimSpace(config[configTaskKind]), "coding")
}

func applyCodingDefaults(agentType string, config map[string]string) map[string]string {
	cfg := core.CloneStringMap(config)
	if cfg == nil {
		cfg = make(map[string]string)
	}
	if utils.IsBlank(cfg[configCodingProfile]) {
		cfg[configCodingProfile] = "full_access"
	}
	if utils.IsBlank(cfg[verifyKeyEnabled]) {
		cfg[verifyKeyEnabled] = "true"
	}
	if utils.IsBlank(cfg[configRetryMaxAttempt]) {
		cfg[configRetryMaxAttempt] = strconv.Itoa(defaultRetryAttempts)
	}
	if utils.IsBlank(cfg[configMergeOnSuccess]) {
		cfg[configMergeOnSuccess] = "true"
	}

	switch strings.ToLower(strings.TrimSpace(agentType)) {
	case "codex", "kimi":
		if utils.IsBlank(cfg["approval_policy"]) {
			cfg["approval_policy"] = "never"
		}
		if utils.IsBlank(cfg["sandbox"]) {
			cfg["sandbox"] = "danger-full-access"
		}
	case "claude_code":
		if utils.IsBlank(cfg["mode"]) {
			cfg["mode"] = "autonomous"
		}
		if utils.IsBlank(cfg["allowed_tools"]) {
			cfg["allowed_tools"] = "*"
		}
	}
	return cfg
}

func boundedRetryAttempts(raw string, fallback int) int {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return fallback
	}
	value, err := strconv.Atoi(trimmed)
	if err != nil {
		return fallback
	}
	if value < 1 {
		return fallback
	}
	if value > maxRetryAttempts {
		return maxRetryAttempts
	}
	return value
}

func buildRetryPrompt(base string, lastErr error, attempt int) string {
	if lastErr == nil {
		return base
	}
	return fmt.Sprintf(
		"%s\n\n[Retry Context]\nPrevious attempt #%d failed with:\n%s\nPlease fix the issue and complete the task end-to-end.",
		base,
		attempt,
		strings.TrimSpace(lastErr.Error()),
	)
}

func errorString(err error) string {
	if err == nil {
		return ""
	}
	return err.Error()
}
