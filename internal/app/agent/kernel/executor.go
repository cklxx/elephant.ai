package kernel

import (
	"alex/internal/shared/utils"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
	kerneldomain "alex/internal/domain/kernel"
	toolshared "alex/internal/infra/tools/builtin/shared"
	id "alex/internal/shared/utils/id"
)

const kernelFounderDirective = `You are the elephant.ai kernel autonomous agent, operating with a founder mindset.

Core behavioral rules:
- Never ask: When facing uncertainty, make autonomous decisions and record rationale. Never invoke ask_user or any action requiring human response.
- Never wait: Never block on anything that requires external response. If a path is blocked, immediately switch to an alternative.
- Only four actions: Think/plan → dispatch tasks (tool calls) → record state → summarize.
- Founder mindset: Take full ownership of outcomes. Proactively identify problems, solve them, and drive progress. No excuses, no waiting for instructions.
- Every cycle must produce observable progress: a written file, a search result, or a state update.`

const kernelDefaultSummaryInstructionTmpl = `Kernel post-run requirement:
- You MUST complete at least one real tool action (for example: read_file, shell_exec, write_file, web_search).
- Do NOT claim completion without tool evidence.
- Do NOT use ask_user or any tool that requires human response.
- If blocked, pivot to an alternative approach. Record the blocker and your decision in the summary.
- Write output files to the kernel artifacts directory: %s
- In your final answer, include a section titled "## Execution Summary".
- Summarize: completed work, concrete evidence/artifacts, decisions made, remaining risks/next step.
- Keep it concise (3-6 bullets) and factual.`

const kernelRetryInstructionTmpl = `Kernel retry requirement:
- Your previous attempt was not autonomously complete.
- Do NOT ask questions, confirmations, or A/B choices.
- Execute at least one concrete real tool action now.
- Write output files to the kernel artifacts directory: %s
- Return a factual "## Execution Summary" with concrete actions and artifact paths.`

// ExecutionResult captures the essential completion information from one dispatch.
type ExecutionResult struct {
	TaskID        string
	Summary       string
	Attempts      int
	RecoveredFrom string
	Autonomy      string
	TeamRoles     []TeamRoleResult
}

// TeamRoleResult captures one role's outcome from a team dispatch.
type TeamRoleResult struct {
	RoleID  string
	Status  string
	Error   string
	Elapsed string
}

// TaskRunner is the minimal task execution contract used by the kernel executor.
type TaskRunner interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// Executor dispatches a single agent task and returns structured execution results.
type Executor interface {
	Execute(ctx context.Context, agentID, prompt string, meta map[string]string) (ExecutionResult, error)
}

// TeamExecutor dispatches a structured team run through the kernel executor.
type TeamExecutor interface {
	ExecuteTeam(ctx context.Context, spec kerneldomain.TeamDispatchSpec, meta map[string]string) (ExecutionResult, error)
}

// CoordinatorExecutor adapts AgentCoordinator.ExecuteTask for kernel dispatch.
type CoordinatorExecutor struct {
	coordinator       TaskRunner
	timeout           time.Duration
	stateDir          string // e.g. ~/.alex/kernel/{kernel_id}
	selectionResolver SelectionResolver
}

var errKernelNoRealToolAction = errors.New("kernel dispatch completed without successful real tool action")
var errKernelAwaitingUserConfirmation = errors.New("kernel dispatch completed while still awaiting user confirmation")
var errKernelInvalidExecutionSummary = errors.New("kernel dispatch completed with invalid execution summary")

const (
	kernelAutonomyActionable  = "actionable"
	kernelAutonomyAwaiting    = "awaiting_input"
	kernelAutonomyNoTool      = "no_real_action"
	kernelAutonomyInvalid     = "invalid_result"
	defaultKernelAttemptCount = 1
)

// NewCoordinatorExecutor creates an executor backed by the given AgentCoordinator.
// stateDir is the kernel-specific state directory (e.g. ~/.alex/kernel/{kernel_id}).
// Artifacts produced by dispatched agents are directed to stateDir/artifacts/.
func NewCoordinatorExecutor(coordinator TaskRunner, timeout time.Duration, stateDir string) *CoordinatorExecutor {
	return &CoordinatorExecutor{
		coordinator: coordinator,
		timeout:     timeout,
		stateDir:    stateDir,
	}
}

// artifactsDir returns the absolute path for kernel agent output artifacts.
func (e *CoordinatorExecutor) artifactsDir() string {
	if e.stateDir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return filepath.Join(".", "artifacts")
		}
		return filepath.Join(home, ".alex", "kernel", "artifacts")
	}
	return filepath.Join(e.stateDir, "artifacts")
}

// tasksDir returns the absolute path for kernel task status sidecars.
func (e *CoordinatorExecutor) tasksDir() string {
	if e.stateDir == "" {
		return filepath.Join(".elephant", "tasks")
	}
	return filepath.Join(e.stateDir, "tasks")
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
	execCtx = appcontext.MarkUnattendedContext(execCtx)
	// Route run_tasks status sidecars to the kernel-specific tasks dir.
	execCtx = toolshared.WithKernelTasksDir(execCtx, e.tasksDir())

	if e.timeout > 0 {
		var cancel context.CancelFunc
		execCtx, cancel = context.WithTimeout(execCtx, e.timeout)
		defer cancel()
	}

	taskPrompt := e.wrapKernelPrompt(prompt)
	result, err := e.coordinator.ExecuteTask(execCtx, taskPrompt, sessionID, nil)
	if err != nil {
		return ExecutionResult{}, err
	}
	attempts := defaultKernelAttemptCount
	recoveredFrom := ""
	if validateErr := validateKernelDispatchResult(result); validateErr != nil {
		recoveredFrom = classifyKernelValidationError(validateErr)
		retryPrompt := e.appendKernelRetryInstruction(taskPrompt, result)
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

// ExecuteTeam executes an explicit team template run through run_tasks.
func (e *CoordinatorExecutor) ExecuteTeam(ctx context.Context, spec kerneldomain.TeamDispatchSpec, meta map[string]string) (ExecutionResult, error) {
	template := strings.TrimSpace(spec.Template)
	if template == "" {
		return ExecutionResult{}, fmt.Errorf("team dispatch template is required")
	}
	goal := strings.TrimSpace(spec.Goal)
	if goal == "" {
		return ExecutionResult{}, fmt.Errorf("team dispatch goal is required")
	}
	prompt := buildKernelTeamDispatchPrompt(spec)
	result, err := e.Execute(ctx, "team:"+template, prompt, meta)
	if err != nil {
		return result, err
	}
	// Best-effort: read status sidecar for role-level results.
	statusPath := filepath.Join(e.tasksDir(), "team-"+template+".status.yaml")
	result.TeamRoles = readTeamRoleResults(statusPath)
	return result, nil
}

// readTeamRoleResults reads a team status sidecar and converts task statuses to role results.
func readTeamRoleResults(statusPath string) []TeamRoleResult {
	sf, err := taskfile.ReadStatusFile(statusPath)
	if err != nil {
		return nil
	}
	roles := make([]TeamRoleResult, 0, len(sf.Tasks))
	for _, ts := range sf.Tasks {
		roles = append(roles, TeamRoleResult{
			RoleID:  ts.ID,
			Status:  ts.Status,
			Error:   ts.Error,
			Elapsed: ts.Elapsed,
		})
	}
	return roles
}

func classifyKernelValidationError(err error) string {
	switch {
	case errors.Is(err, errKernelAwaitingUserConfirmation):
		return kernelAutonomyAwaiting
	case errors.Is(err, errKernelNoRealToolAction):
		return kernelAutonomyNoTool
	case errors.Is(err, errKernelInvalidExecutionSummary):
		return kernelAutonomyInvalid
	default:
		return ""
	}
}

func buildKernelTeamDispatchPrompt(spec kerneldomain.TeamDispatchSpec) string {
	timeoutSeconds := spec.TimeoutSeconds
	if timeoutSeconds <= 0 {
		timeoutSeconds = DefaultKernelTeamTimeoutSeconds
	}
	args := map[string]any{
		"template":        strings.TrimSpace(spec.Template),
		"goal":            strings.TrimSpace(spec.Goal),
		"wait":            true,
		"timeout_seconds": timeoutSeconds,
	}
	if len(spec.Prompts) > 0 {
		args["prompts"] = spec.Prompts
	}
	payload, err := json.Marshal(args)
	if err != nil {
		return fmt.Sprintf(
			"Run the team template %q with goal %q via run_tasks. Wait for completion and include the status file path in Execution Summary.",
			spec.Template,
			spec.Goal,
		)
	}
	return fmt.Sprintf(
		"CRITICAL: Execute autonomously. Do NOT ask for confirmation or clarification. Make all decisions independently.\n\nExecute a structured team run now. Call run_tasks exactly once with arguments: %s\nThen read the generated .status sidecar and summarize completed roles, failures (if any), and artifact paths.",
		string(payload),
	)
}

func (e *CoordinatorExecutor) wrapKernelPrompt(prompt string) string {
	summaryInstruction := fmt.Sprintf(kernelDefaultSummaryInstructionTmpl, e.artifactsDir())
	trimmed := strings.TrimSpace(prompt)
	if trimmed == "" {
		trimmed = summaryInstruction
	}
	var b strings.Builder
	b.WriteString(kernelFounderDirective)
	b.WriteString("\n\n")
	b.WriteString(trimmed)
	if !strings.Contains(trimmed, "## Execution Summary") {
		b.WriteString("\n\n")
		b.WriteString(summaryInstruction)
	}
	return b.String()
}

func (e *CoordinatorExecutor) appendKernelRetryInstruction(prompt string, result *agent.TaskResult) string {
	var previousSummary string
	if result != nil {
		previousSummary = strings.TrimSpace(extractKernelExecutionSummary(result))
	}
	retryInstruction := fmt.Sprintf(kernelRetryInstructionTmpl, e.artifactsDir())
	sections := []string{
		strings.TrimSpace(prompt),
		retryInstruction,
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
	if idx := strings.Index(answer, "## Execution Summary"); idx >= 0 {
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
			if utils.IsBlank(name) {
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
	if !isKernelExecutionSummaryValid(extractKernelExecutionSummary(result)) {
		return errKernelInvalidExecutionSummary
	}
	return nil
}

func isKernelExecutionSummaryValid(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "empty response:") || strings.HasPrefix(lower, "empty completion:") {
		return false
	}
	if strings.HasPrefix(lower, "{") &&
		strings.Contains(lower, "stop_reason") &&
		strings.Contains(lower, "content") &&
		strings.Contains(lower, "input_tokens") &&
		strings.Contains(lower, "output_tokens") {
		return false
	}
	return true
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
	case "plan", "ask_user", "todo_read", "todo_update", "attention", "context_checkpoint":
		return true
	default:
		return false
	}
}
