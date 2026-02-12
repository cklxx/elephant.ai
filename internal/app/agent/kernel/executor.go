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
	id "alex/internal/shared/utils/id"
)

const kernelDefaultSummaryInstruction = `Kernel post-run requirement:
- You MUST complete at least one real tool action (for example: read_file, shell_exec, write_file).
- Do NOT claim completion without tool evidence.
- If blocked, report exact tool error and mark task as blocked.
- In your final answer, include a section titled "## 执行总结".
- Summarize: completed work, concrete evidence/artifacts, remaining risks/next step.
- Keep it concise (3-6 bullets) and factual.`

// ExecutionResult captures the essential completion information from one dispatch.
type ExecutionResult struct {
	TaskID  string
	Summary string
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

	if e.timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, e.timeout)
		defer cancel()
	}

	taskPrompt := appendKernelSummaryInstruction(prompt)
	result, err := e.coordinator.ExecuteTask(execCtx, taskPrompt, sessionID, nil)
	if err != nil {
		return ExecutionResult{}, err
	}
	if !containsSuccessfulRealToolExecution(result) {
		return ExecutionResult{}, errKernelNoRealToolAction
	}
	return ExecutionResult{
		TaskID:  sessionID,
		Summary: extractKernelExecutionSummary(result),
	}, nil
}

func appendKernelSummaryInstruction(prompt string) string {
	trimmed := strings.TrimSpace(prompt)
	if strings.Contains(trimmed, "## 执行总结") {
		return trimmed
	}
	if trimmed == "" {
		return kernelDefaultSummaryInstruction
	}
	return trimmed + "\n\n" + kernelDefaultSummaryInstruction
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

func isOrchestrationTool(name string) bool {
	switch strings.ToLower(strings.TrimSpace(name)) {
	case "plan", "clearify", "clarify", "todo_read", "todo_update", "attention", "context_checkpoint", "request_user":
		return true
	default:
		return false
	}
}
