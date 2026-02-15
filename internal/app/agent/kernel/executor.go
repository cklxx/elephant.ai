package kernel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	agent "alex/internal/domain/agent/ports/agent"
	toolshared "alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

const kernelFounderDirective = `你是 elephant.ai 的 kernel 自主代理，以创始人心态运作。

核心行为准则：
- 永不询问：遇到不确定性时，自主决策并记录决策理由。不要发起 request_user、clarify 等任何需要人类回应的动作。
- 永不等待：不要阻塞在任何需要外部回应的环节。如果某路径受阻，立即切换到备选方案。
- 只做四件事：思考/规划 → 派发任务（工具调用） → 记录状态 → 做总结。
- 创始人心态：对结果负全责。主动发现问题、主动解决、主动推进。不找借口、不等指令。
- 每个 cycle 必须产出可观测的进展：一个写入的文件、一次搜索结果、一个状态更新。`

const kernelDefaultSummaryInstruction = `Kernel post-run requirement:
- You MUST complete at least one real tool action (for example: read_file, shell_exec, write_file, web_search).
- Do NOT claim completion without tool evidence.
- Do NOT use request_user, clarify, or any tool that requires human response.
- If blocked, pivot to an alternative approach. Record the blocker and your decision in the summary.
- Write files directly to ~/.alex/kernel/default/ (this path is authorized and writable).
- In your final answer, include a section titled "## 执行总结".
- Summarize: completed work, concrete evidence/artifacts, decisions made, remaining risks/next step.
- Keep it concise (3-6 bullets) and factual.`

const kernelRetryInstruction = `Kernel retry requirement:
- Your previous attempt was not autonomously complete.
- Do NOT ask questions, confirmations, or A/B choices.
- Execute at least one concrete real tool action now.
- Write files directly to ~/.alex/kernel/default/ (this path is authorized and writable).
- Return a factual "## 执行总结" with concrete actions and artifact paths.`

// ExecutionResult captures the essential completion information from one dispatch.
type ExecutionResult struct {
	TaskID        string
	Summary       string
	Attempts      int
	RecoveredFrom string
	Autonomy      string
}

// TaskRunner is the minimal task execution contract used by the kernel executor.
type TaskRunner interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Executor dispatches a single agent task and returns structured execution results.
type Executor interface {
	Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error)
}

// CoordinatorExecutor adapts AgentCoordinator.ExecuteTask for kernel dispatch.
type CoordinatorExecutor struct {
	coordinator       TaskRunner
	timeout           time.Duration
	selectionResolver SelectionResolver
}

var errKernelNoRealToolAction = errors.New("kernel dispatch completed without successful real tool action")
var errKernelAwaitingUserConfirmation = errors.New("kernel dispatch completed while still awaiting user confirmation")

const (
	kernelAutonomyActionable  = "actionable"
	kernelAutonomyAwaiting    = "awaiting_input"
	kernelAutonomyNoTool      = "no_real_action"
	kernelAutonomyInvalid     = "invalid_result"
	defaultKernelAttemptCount = 1
)

// NewCoordinatorExecutor creates an executor backed by the given AgentCoordinator.
func NewCoordinatorExecutor(coordinator TaskRunner, timeout time.Duration) *CoordinatorExecutor {
	return &CoordinatorExecutor{
		coordinator: coordinator,
		timeout:     timeout,
	}
}

// SelectionResolver resolves a request-scoped pinned LLM selection for kernel runs.
type SelectionResolver func(ctx context.Context, channel, chatID, userID string) (subscription.ResolvedSelection, bool)

// SetSelectionResolver configures an optional selection resolver used to pin
// provider/model credentials per dispatch context.
func (e *CoordinatorExecutor) SetSelectionResolver(resolver SelectionResolver) {
	if e == nil {
		return
	}
	e.selectionResolver = resolver
}

// Execute runs a task through the AgentCoordinator and returns the session ID as task identifier.
func (e *CoordinatorExecutor) Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error) {
	runID := id.NewRunID()
	sessionID := fmt.Sprintf("kernel-%s-%s", agentID, runID)

	execCtx := id.WithRunID(ctx, runID)
	execCtx = id.WithSessionID(execCtx, sessionID)

	if uid, ok := meta["user_id"]; ok && uid != "" {
		execCtx = id.WithUserID(execCtx, uid)
	}
	if channel, ok := meta["channel"]; ok && channel != "" {
		execCtx = appcontext.WithChannel(execCtx, strings.TrimSpace(channel))
	}
	if chatID, ok := meta["chat_id"]; ok && chatID != "" {
		execCtx = appcontext.WithChatID(execCtx, strings.TrimSpace(chatID))
	}
	if e.selectionResolver != nil {
		channel := strings.TrimSpace(meta["channel"])
		chatID := strings.TrimSpace(meta["chat_id"])
		userID := strings.TrimSpace(meta["user_id"])
		if selection, ok := e.selectionResolver(execCtx, channel, chatID, userID); ok {
			execCtx = appcontext.WithLLMSelection(execCtx, selection)
		}
	}

	// Kernel cycles are unattended — auto-approve all tool executions to
	// prevent deadlocks on approval gates.
	execCtx = toolshared.WithAutoApprove(execCtx, true)

	if e.timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, e.timeout)
		defer cancel()
	}

	taskPrompt := wrapKernelPrompt(prompt)
	result, err := e.coordinator.ExecuteTask(execCtx, taskPrompt, sessionID, nil)
	if err != nil {
		return ExecutionResult{}, err
	}
	attempts := defaultKernelAttemptCount
	recoveredFrom := ""
	if validateErr := validateKernelDispatchResult(result); validateErr != nil {
		recoveredFrom = classifyKernelValidationError(validateErr)
		retryPrompt := appendKernelRetryInstruction(taskPrompt, result)
		retryResult, retryErr := e.coordinator.ExecuteTask(execCtx, retryPrompt, sessionID, nil)
		if retryErr != nil {
			return ExecutionResult{}, retryErr
		}
		attempts++
		if retryValidateErr := validateKernelDispatchResult(retryResult); retryValidateErr != nil {
			return ExecutionResult{}, retryValidateErr
		}
		result = retryResult
	}
	return ExecutionResult{
		TaskID:        sessionID,
		Summary:       extractKernelExecutionSummary(result),
		Attempts:      attempts,
		RecoveredFrom: recoveredFrom,
		Autonomy:      kernelAutonomyActionable,
	}, nil
}

func classifyKernelValidationError(err error) string {
	switch {
	case errors.Is(err, errKernelAwaitingUserConfirmation):
		return kernelAutonomyAwaiting
	case errors.Is(err, errKernelNoRealToolAction):
		return kernelAutonomyNoTool
	default:
		return kernelAutonomyInvalid
	}
}

func wrapKernelPrompt(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		trimmed = kernelDefaultSummaryInstruction
	}
	var b strings.Builder
	b.WriteString(kernelFounderDirective)
	b.WriteString("\n\n")
	b.WriteString(trimmed)
	if !strings.Contains(trimmed, "## 执行总结") {
		b.WriteString("\n\n")
		b.WriteString(kernelDefaultSummaryInstruction)
	}
	return b.String()
}

func appendKernelRetryInstruction(prompt string, result *agent.TaskResult) string {
	var previousSummary string
	if result != nil {
		previousSummary = strings.TrimSpace(extractKernelExecutionSummary(result))
	}
	sections := []string{
		strings.TrimSpace(prompt),
		kernelRetryInstruction,
	}
	if previousSummary != "" {
		sections = append(sections, "Previous attempt summary:\n"+previousSummary)
	}
	return strings.Join(sections, "\n\n")
}

func extractKernelExecutionSummary(result *agent.TaskResult) string {
	if result == nil {
		return ""
	}
	answer := strings.TrimSpace(result.Answer)
	if answer == "" {
		return ""
	}
	if idx := strings.Index(answer, "## 执行总结"); idx >= 0 {
		answer = answer[idx:]
	}
	return compactSummary(answer, 500)
}

func compactSummary(raw string, maxLen int) string {
	if maxLen <= 0 {
		return ""
	}
	fields := strings.Fields(raw)
	if len(fields) == 0 {
		return ""
	}
	normalized := strings.Join(fields, " ")
	runes := []rune(normalized)
	if len(runes) <= maxLen {
		return normalized
	}
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return strings.TrimSpace(string(runes[:maxLen-3])) + "..."
}

func containsSuccessfulRealToolExecution(result *agent.TaskResult) bool {
	if result == nil {
		return false
	}
	callNames := make(map[string]string)
	hasToolResults := false
	sawSuccessfulUnknown := false
	for _, msg := range result.Messages {
		for _, tc := range msg.ToolCalls {
			callNames[tc.ID] = tc.Name
		}
	}
	for _, msg := range result.Messages {
		if len(msg.ToolResults) > 0 {
			hasToolResults = true
		}
		for _, tr := range msg.ToolResults {
			name := callNames[tr.CallID]
			if strings.TrimSpace(name) == "" {
				if tr.Error == nil {
					sawSuccessfulUnknown = true
				}
				continue
			}
			if tr.Error == nil && !isOrchestrationTool(name) {
				return true
			}
		}
	}
	if hasToolResults {
		return sawSuccessfulUnknown
	}

	// Fallback for providers that emit tool calls without structured tool results.
	for _, msg := range result.Messages {
		for _, tc := range msg.ToolCalls {
			name := strings.TrimSpace(tc.Name)
			if name == "" || isOrchestrationTool(name) {
				continue
			}
			return true
		}
	}
	return false
}

func validateKernelDispatchResult(result *agent.TaskResult) error {
	if dispatchStillAwaitsUserConfirmation(result) {
		return errKernelAwaitingUserConfirmation
	}
	if !containsSuccessfulRealToolExecution(result) {
		return errKernelNoRealToolAction
	}
	return nil
}

func dispatchStillAwaitsUserConfirmation(result *agent.TaskResult) bool {
	if result == nil {
		return false
	}
	if strings.EqualFold(strings.TrimSpace(result.StopReason), "await_user_input") {
		return true
	}
	if _, ok := agent.ExtractAwaitUserInputPrompt(result.Messages); ok {
		return true
	}
	return answerContainsUserConfirmationPrompt(result.Answer)
}

func answerContainsUserConfirmationPrompt(answer string) bool {
	trimmed := strings.TrimSpace(answer)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)

	if strings.Contains(lower, "do you want me") ||
		strings.Contains(lower, "my understanding is") && strings.Contains(lower, "?") ||
		strings.Contains(lower, "please confirm") ||
		strings.Contains(lower, "please choose") ||
		strings.Contains(lower, "option a") && strings.Contains(lower, "option b") {
		return true
	}

	if strings.Contains(trimmed, "我的理解是") && (strings.Contains(trimmed, "对吗") || strings.Contains(trimmed, "是否")) {
		return true
	}
	if strings.Contains(trimmed, "你要我") && strings.Contains(trimmed, "吗") {
		return true
	}
	if strings.Contains(trimmed, "请确认") || strings.Contains(trimmed, "请选择") || strings.Contains(trimmed, "请回复") {
		return true
	}
	if strings.Contains(trimmed, "可选") && (strings.Contains(lower, "a)") || strings.Contains(lower, "b)")) {
		return true
	}

	return false
}

func isOrchestrationTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plan", "clearify", "clarify", "todo_read", "todo_update", "attention", "context_checkpoint", "request_user":
		return true
	default:
		return false
	}
}
