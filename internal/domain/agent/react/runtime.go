package react

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"

	"go.opentelemetry.io/otel/attribute"
)

// reactRuntime wraps the ReAct loop with explicit lifecycle bookkeeping so the
// engine can focus on business rules while the runtime owns iteration control,
// workflow transitions, and cancellation handling.
type reactRuntime struct {
	engine    *ReactEngine
	ctx       context.Context
	task      string
	state     *TaskState
	services  Services
	tracker   *reactWorkflow
	startTime time.Time
	finalizer sync.Once
	finalize  sync.Once
	prepare   func()

	// UI orchestration state (Plan → Clarify → ReAct → Finalize).
	runID                 string
	planEmitted           bool
	planVersion           int
	planComplexity        string
	planReviewEnabled     bool
	lastPlanReviewVersion int
	currentTaskID         string
	clarifyEmitted        map[string]bool
	pendingTaskID         string
	nextTaskSeq           int
	pauseRequested        bool
	replanRequested       bool
	finalReviewAttempts   int
	finalReviewCallID     string
	finalReviewStartedAt  time.Time
	finalReviewInFlight   bool
	finalReviewAttempt    int

	// Background task manager for async subagent execution.
	bgManager      *BackgroundTaskManager
	bgManagerOwned bool
	// Track emitted completion events to avoid duplicates.
	bgCompletionEmitted map[string]bool
	// External input requests from interactive external agents.
	externalInputCh      <-chan agent.InputRequest
	externalInputEmitted map[string]bool

	// User input channel for live message injection from chat gateways.
	userInputCh <-chan agent.UserInput
}

const (
	planStatusPending    = "pending"
	planStatusInProgress = "in_progress"
	planStatusBlocked    = "blocked"
	planStatusCompleted  = "completed"
)

const replanPrompt = "工具执行失败，请重新规划并在继续前调用 plan() 或 clarify()。"

const finalAnswerReviewPrompt = `<final_answer_review>
Before finalizing, do a quick review pass:
- Confirm the user goal is fully satisfied and the answer is correct.
- If any required information is missing, ask the user via clarify(needs_user_input=true, question_to_user=...) or request_user(...), then stop.
- If additional tool usage would materially improve correctness or completeness, call the relevant tools now.
- If an external CLI tool is required (e.g., ffmpeg):
  1) Use bash to check availability: command -v ffmpeg
  2) If missing and on macOS with Homebrew: brew install ffmpeg
  3) If install fails or brew is unavailable: ask the user with clear manual steps and wait.
- If everything is complete, provide the final answer (no tool calls).
</final_answer_review>`

type reactIteration struct {
	runtime    *reactRuntime
	index      int
	thought    Message
	toolCalls  []ToolCall
	plan       toolExecutionPlan
	toolResult []ToolResult
}

type toolExecutionPlan struct {
	iteration int
	nodeID    string
	calls     []ToolCall
}

func newReactRuntime(engine *ReactEngine, ctx context.Context, task string, state *TaskState, services Services, prepare func()) *reactRuntime {
	runtime := &reactRuntime{
		engine:    engine,
		ctx:       ctx,
		task:      task,
		state:     state,
		services:  services,
		tracker:   newReactWorkflow(engine.workflow),
		startTime: engine.clock.Now(),
		prepare:   prepare,
	}
	if state != nil {
		runtime.runID = strings.TrimSpace(state.RunID)
		runtime.planReviewEnabled = state.PlanReviewEnabled
	}
	runtime.clarifyEmitted = make(map[string]bool)
	runtime.nextTaskSeq = 1
	runtime.userInputCh = agent.UserInputChFromContext(ctx)

	// Initialize background task manager when executor is available.
	if engine.backgroundExecutor != nil || engine.backgroundManager != nil {
		sessionID := ""
		runID := ""
		parentRunID := ""
		if state != nil {
			sessionID = state.SessionID
			runID = state.RunID
			parentRunID = state.ParentRunID
		}
		if engine.backgroundManager != nil {
			runtime.bgManager = engine.backgroundManager
			runtime.bgManagerOwned = false
		} else {
			runtime.bgManager = newBackgroundTaskManagerWithDeps(
				ctx,
				engine.logger,
				engine.clock,
				engine.idGenerator,
				engine.idContextReader,
				engine.goRunner,
				engine.workingDirResolver,
				engine.workspaceMgrFactory,
				engine.backgroundExecutor,
				engine.externalExecutor,
				engine.emitEvent,
				func(ctx context.Context) domain.BaseEvent {
					return engine.newBaseEvent(ctx, sessionID, runID, parentRunID)
				},
				sessionID,
				engine.eventListener,
			)
			runtime.bgManagerOwned = true
		}
		runtime.bgCompletionEmitted = make(map[string]bool)
		if runtime.bgManager != nil {
			runtime.ctx = withBackgroundEventSink(runtime.ctx, backgroundEventSink{
				emitEvent: engine.emitEvent,
				baseEvent: func(ctx context.Context) domain.BaseEvent {
					return engine.newBaseEvent(ctx, sessionID, runID, parentRunID)
				},
				parentListener: engine.eventListener,
			})
			runtime.externalInputCh = runtime.bgManager.InputRequests()
			runtime.externalInputEmitted = make(map[string]bool)
		}
	}

	return runtime
}

func (r *reactRuntime) run() (*TaskResult, error) {
	r.tracker.startContext(r.task)
	resumed, err := r.engine.ResumeFromCheckpoint(r.ctx, r.state.SessionID, r.state, r.services)
	if err != nil {
		r.engine.logger.Warn("Failed to resume from checkpoint: %v", err)
	}
	if resumed {
		r.tracker.completeContext(workflowContextOutput(r.state))
	} else {
		r.prepareContext()
	}

	// Set background dispatcher in context for tools.
	if r.bgManager != nil {
		r.ctx = agent.WithBackgroundDispatcher(r.ctx, newBackgroundDispatcherWithEvents(r, r.bgManager))
	}

	for r.state.Iterations < r.engine.maxIterations {
		// Inject background completion notifications before each iteration.
		r.injectBackgroundNotifications()
		r.injectExternalInputRequests()
		r.injectUserInput()

		if result, stop, err := r.handleCancellation(); stop || err != nil {
			r.cleanupBackgroundTasks()
			return result, err
		}

		r.state.Iterations++
		r.engine.logger.Info("=== Iteration %d/%d ===", r.state.Iterations, r.engine.maxIterations)

		result, done, err := r.runIteration()
		if err != nil {
			r.cleanupBackgroundTasks()
			return nil, err
		}
		if done {
			r.cleanupBackgroundTasks()
			return result, nil
		}
	}

	r.cleanupBackgroundTasks()
	return r.handleMaxIterations()
}

func (r *reactRuntime) prepareContext() {
	if r.prepare != nil {
		r.prepare()
	} else {
		r.engine.prepareUserTaskContext(r.ctx, r.task, r.state)
	}
	r.tracker.completeContext(workflowContextOutput(r.state))
}

func (r *reactRuntime) handleCancellation() (*TaskResult, bool, error) {
	if r.ctx.Err() == nil {
		return nil, false, nil
	}

	r.engine.logger.Info("Context cancelled, stopping execution: %v", r.ctx.Err())
	finalResult := r.finalizeResult("cancelled", nil, true, nil)
	return finalResult, true, r.ctx.Err()
}

func (r *reactRuntime) runIteration() (_ *TaskResult, _ bool, err error) {
	iteration := r.newIteration()
	prevCtx := r.ctx
	spanCtx, span := startReactSpan(
		r.ctx,
		traceSpanReactIteration,
		r.state,
		attribute.Int(traceAttrIteration, iteration.index),
		attribute.Int("alex.message_count", len(r.state.Messages)),
	)
	r.ctx = spanCtx
	defer func() {
		span.SetAttributes(
			attribute.Int("alex.tool_result_count", len(iteration.toolResult)),
			attribute.Int("alex.token_count", r.state.TokenCount),
		)
		markSpanResult(span, err)
		span.End()
		r.ctx = prevCtx
	}()

	r.applyIterationHook(iteration.index)

	if thinkErr := iteration.think(); thinkErr != nil {
		err = thinkErr
		return nil, true, err
	}

	result, done, planErr := iteration.planTools()
	if done || planErr != nil {
		err = planErr
		return result, done, err
	}

	if len(iteration.plan.calls) == 0 && iteration.plan.nodeID == "" && result == nil {
		return nil, false, nil
	}

	iteration.executeTools()
	iteration.observeTools()
	r.engine.saveCheckpoint(r.ctx, r.state, nil)
	iteration.finish()
	if r.pauseRequested {
		finalResult := r.finalizeResult("await_user_input", nil, true, nil)
		return finalResult, true, nil
	}

	return nil, false, nil
}

func (r *reactRuntime) newIteration() *reactIteration {
	return &reactIteration{runtime: r, index: r.state.Iterations}
}

func (r *reactRuntime) emitWorkflowToolStartedEvents(calls []ToolCall) {
	if len(calls) == 0 {
		return
	}

	state := r.state
	for idx := range calls {
		call := calls[idx]
		r.engine.emitEvent(&domain.WorkflowToolStartedEvent{
			BaseEvent: r.engine.newBaseEvent(r.ctx, state.SessionID, state.RunID, state.ParentRunID),
			Iteration: state.Iterations,
			CallID:    call.ID,
			ToolName:  call.Name,
			Arguments: call.Arguments,
		})
	}
}

func (r *reactRuntime) handleMaxIterations() (*TaskResult, error) {
	r.engine.logger.Warn("Max iterations (%d) reached, requesting final answer", r.engine.maxIterations)
	finalResult := r.engine.finalize(r.state, "max_iterations", r.engine.clock.Now().Sub(r.startTime))

	if strings.TrimSpace(finalResult.Answer) == "" {
		r.state.Messages = append(r.state.Messages, Message{
			Role:    "user",
			Content: "Please provide your final answer to the user's question now.",
			Source:  ports.MessageSourceSystemPrompt,
		})

		finalThought, err := r.engine.think(r.ctx, r.state, r.services)
		if err == nil && finalThought.Content != "" {
			if att := resolveContentAttachments(finalThought.Content, r.state); len(att) > 0 {
				finalThought.Attachments = att
			}
			r.state.Messages = append(r.state.Messages, finalThought)
			if registerMessageAttachments(r.ctx, r.state, &r.state.Messages[len(r.state.Messages)-1], r.engine.attachmentPersister) {
				r.engine.updateAttachmentCatalogMessage(r.state)
			}
			finalResult.Answer = finalThought.Content
			r.engine.logger.Info("Got final answer from retry: %d chars", len(finalResult.Answer))
		}
	}

	return r.finalizeResult("max_iterations", finalResult, true, nil), nil
}

// enforceOrchestratorGates prevents plan/clarify/request_user from being
// called in parallel with other tools (they must be sole calls in a batch).
// No longer enforces mandatory plan→clarify ordering.
func (r *reactRuntime) enforceOrchestratorGates(calls []ToolCall) (bool, string) {
	if len(calls) == 0 {
		return false, ""
	}

	hasPlan := false
	hasClarify := false
	hasRequestUser := false
	for _, call := range calls {
		name := strings.ToLower(strings.TrimSpace(call.Name))
		switch name {
		case "plan":
			hasPlan = true
		case "clarify":
			hasClarify = true
		case "request_user":
			hasRequestUser = true
		}
	}

	if hasPlan && len(calls) > 1 {
		return true, "plan() 必须单独调用。请移除同轮其它工具调用并重试。"
	}
	if hasClarify && len(calls) > 1 {
		return true, "clarify() 必须单独调用。请移除同轮其它工具调用并重试。"
	}
	if hasRequestUser && len(calls) > 1 {
		return true, "request_user() 必须单独调用。请移除同轮其它工具调用并重试。"
	}

	return false, ""
}

func (r *reactRuntime) injectOrchestratorCorrection(content string) {
	if r == nil || r.state == nil {
		return
	}
	trimmed := strings.TrimSpace(content)
	if trimmed == "" {
		return
	}
	r.state.Messages = append(r.state.Messages, Message{
		Role:    "system",
		Content: trimmed,
		Source:  ports.MessageSourceSystemPrompt,
	})
}

func (r *reactRuntime) shouldTriggerFinalAnswerReview() bool {
	if r == nil || r.state == nil || r.engine == nil {
		return false
	}
	cfg := r.engine.finalAnswerReview
	if !cfg.Enabled {
		return false
	}
	maxExtra := cfg.MaxExtraIterations
	if maxExtra <= 0 {
		maxExtra = 1
	}
	if r.finalReviewAttempts >= maxExtra {
		return false
	}
	// Only trigger for non-trivial runs (at least one tool executed).
	if len(r.state.ToolResults) == 0 {
		return false
	}
	// Need at least one remaining iteration to perform the review.
	if r.state.Iterations >= r.engine.maxIterations {
		return false
	}
	return true
}

func (r *reactRuntime) startFinalAnswerReviewToolEvent(callID string, attempt int) {
	if r == nil || r.engine == nil || r.state == nil {
		return
	}
	callID = strings.TrimSpace(callID)
	if callID == "" {
		return
	}

	if r.finalReviewInFlight {
		r.completeFinalAnswerReviewToolEvent("superseded by another review attempt")
	}

	r.finalReviewCallID = callID
	r.finalReviewStartedAt = r.engine.clock.Now()
	r.finalReviewInFlight = true
	r.finalReviewAttempt = attempt

	r.engine.emitEvent(&domain.WorkflowToolStartedEvent{
		BaseEvent: r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		Iteration: r.state.Iterations,
		CallID:    callID,
		ToolName:  "final_answer_review",
		Arguments: map[string]any{"attempt": attempt},
	})
}

func (r *reactRuntime) completeFinalAnswerReviewToolEvent(result string) {
	if r == nil || r.engine == nil || r.state == nil {
		return
	}
	if !r.finalReviewInFlight {
		return
	}
	callID := strings.TrimSpace(r.finalReviewCallID)
	if callID == "" {
		return
	}

	duration := r.engine.clock.Now().Sub(r.finalReviewStartedAt)
	r.engine.emitEvent(&domain.WorkflowToolCompletedEvent{
		BaseEvent: r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		CallID:    callID,
		ToolName:  "final_answer_review",
		Result:    strings.TrimSpace(result),
		Duration:  duration,
		Metadata:  map[string]any{"attempt": r.finalReviewAttempt},
	})

	r.finalReviewInFlight = false
	r.finalReviewCallID = ""
	r.finalReviewAttempt = 0
	r.finalReviewStartedAt = time.Time{}
}

func (r *reactRuntime) injectFinalAnswerReviewPrompt() {
	if r == nil || r.state == nil {
		return
	}
	attempt := r.finalReviewAttempts + 1
	r.finalReviewAttempts = attempt
	callID := fmt.Sprintf("final_answer_review:%d", attempt)
	r.startFinalAnswerReviewToolEvent(callID, attempt)
	r.state.Messages = append(r.state.Messages, Message{
		Role:    "system",
		Content: finalAnswerReviewPrompt,
		Source:  ports.MessageSourceSystemPrompt,
	})
}

func (r *reactRuntime) updateOrchestratorState(calls []ToolCall, results []ToolResult) {
	if r == nil || len(calls) == 0 || len(results) == 0 {
		return
	}
	limit := len(calls)
	if len(results) < limit {
		limit = len(results)
	}

	for i := 0; i < limit; i++ {
		call := calls[i]
		result := results[i]
		if result.Error != nil {
			r.handleToolError(call, result)
			continue
		}

		name := strings.ToLower(strings.TrimSpace(call.Name))
		switch name {
		case "plan":
			r.planEmitted = true
			r.planVersion++
			r.replanRequested = false
			if raw, ok := call.Arguments["complexity"].(string); ok {
				complexity := strings.ToLower(strings.TrimSpace(raw))
				if complexity == "simple" || complexity == "complex" {
					r.planComplexity = complexity
				}
			} else if result.Metadata != nil {
				if raw, ok := result.Metadata["complexity"].(string); ok {
					complexity := strings.ToLower(strings.TrimSpace(raw))
					if complexity == "simple" || complexity == "complex" {
						r.planComplexity = complexity
					}
				}
			}
			r.maybeTriggerPlanReview(call, result)
		case "clarify":
			r.handleClarifyResult(result)
			if result.Metadata != nil {
				if needs, ok := result.Metadata["needs_user_input"].(bool); ok && needs {
					r.pauseRequested = true
				}
			}
		case "request_user":
			r.pauseRequested = true
		}
	}
}

func (r *reactRuntime) handleToolError(call ToolCall, result ToolResult) {
	if r == nil || r.state == nil {
		return
	}
	targetID := strings.TrimSpace(r.currentTaskID)
	if targetID == "" && len(r.state.Plans) > 0 {
		targetID = strings.TrimSpace(r.state.Plans[len(r.state.Plans)-1].ID)
	}
	if targetID != "" {
		r.updatePlanStatus(targetID, planStatusBlocked, false)
	}
	if !r.replanRequested {
		r.injectOrchestratorCorrection(replanPrompt)
		reason := "orchestrator tool failure triggered replan injection"
		errMsg := "tool execution failed"
		if result.Error != nil {
			errMsg = result.Error.Error()
		}
		r.engine.emitEvent(&domain.WorkflowReplanRequestedEvent{
			BaseEvent: r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
			CallID:    call.ID,
			ToolName:  call.Name,
			Reason:    reason,
			Error:     errMsg,
		})
		r.replanRequested = true
	}
}

func (r *reactRuntime) handleClarifyResult(result ToolResult) {
	if r == nil || r.state == nil || result.Metadata == nil {
		return
	}
	taskID, _ := result.Metadata["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	taskGoal, _ := result.Metadata["task_goal_ui"].(string)
	taskGoal = strings.TrimSpace(taskGoal)

	if r.currentTaskID != "" && r.currentTaskID != taskID {
		r.updatePlanStatus(r.currentTaskID, planStatusCompleted, true)
	}

	node := agent.PlanNode{
		ID:          taskID,
		Title:       taskGoal,
		Status:      planStatusInProgress,
		Description: strings.Join(extractSuccessCriteria(result.Metadata), "\n"),
	}
	r.upsertPlanNode(node)

	r.currentTaskID = taskID
	r.clarifyEmitted[taskID] = true
	r.pendingTaskID = ""
	r.replanRequested = false
}

func extractSuccessCriteria(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	if raw, ok := metadata["success_criteria"].([]string); ok {
		return append([]string(nil), raw...)
	}
	raw, ok := metadata["success_criteria"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func (r *reactRuntime) upsertPlanNode(node agent.PlanNode) {
	if r == nil || r.state == nil {
		return
	}
	if strings.TrimSpace(node.ID) == "" {
		return
	}
	if updatePlanNode(r.state.Plans, node) {
		return
	}
	r.state.Plans = append(r.state.Plans, node)
}

func updatePlanNode(nodes []agent.PlanNode, node agent.PlanNode) bool {
	for i := range nodes {
		if strings.TrimSpace(nodes[i].ID) == strings.TrimSpace(node.ID) {
			nodes[i].Title = node.Title
			nodes[i].Description = node.Description
			nodes[i].Status = node.Status
			return true
		}
		if updatePlanNode(nodes[i].Children, node) {
			return true
		}
	}
	return false
}

func (r *reactRuntime) updatePlanStatus(id string, status string, skipIfBlocked bool) bool {
	if r == nil || r.state == nil {
		return false
	}
	return updatePlanStatus(r.state.Plans, strings.TrimSpace(id), status, skipIfBlocked)
}

func updatePlanStatus(nodes []agent.PlanNode, id string, status string, skipIfBlocked bool) bool {
	if strings.TrimSpace(id) == "" {
		return false
	}
	for i := range nodes {
		if strings.TrimSpace(nodes[i].ID) == id {
			if skipIfBlocked && nodes[i].Status == planStatusBlocked {
				return true
			}
			nodes[i].Status = status
			return true
		}
		if updatePlanStatus(nodes[i].Children, id, status, skipIfBlocked) {
			return true
		}
	}
	return false
}

func (r *reactRuntime) maybeTriggerPlanReview(call ToolCall, result ToolResult) {
	if r == nil || !r.planReviewEnabled {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.planComplexity), "complex") {
		return
	}
	if r.planVersion <= r.lastPlanReviewVersion {
		return
	}

	goal := ""
	internalPlan := any(nil)
	if result.Metadata != nil {
		if raw, ok := result.Metadata["overall_goal_ui"].(string); ok {
			goal = strings.TrimSpace(raw)
		}
		if raw, ok := result.Metadata["internal_plan"]; ok {
			internalPlan = raw
		}
	}
	if goal == "" {
		if raw, ok := call.Arguments["overall_goal_ui"].(string); ok {
			goal = strings.TrimSpace(raw)
		}
	}
	if internalPlan == nil {
		if raw, ok := call.Arguments["internal_plan"]; ok {
			internalPlan = raw
		}
	}
	if goal == "" {
		return
	}

	r.injectPlanReviewMarker(goal, internalPlan, r.runID)
	r.pauseRequested = true
	r.lastPlanReviewVersion = r.planVersion
}

func (r *reactRuntime) injectPlanReviewMarker(goal string, internalPlan any, runID string) {
	if r == nil || r.state == nil {
		return
	}
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return
	}
	if runID == "" {
		runID = "<run_id>"
	}

	planText := ""
	if internalPlan != nil {
		if data, err := r.engine.jsonCodec.Marshal(internalPlan); err == nil {
			planText = string(data)
		}
	}

	var sb strings.Builder
	sb.WriteString("<plan_review_pending>\n")
	sb.WriteString("run_id: ")
	sb.WriteString(runID)
	sb.WriteString("\n")
	sb.WriteString("overall_goal_ui: ")
	sb.WriteString(goal)
	sb.WriteString("\n")
	if planText != "" {
		sb.WriteString("internal_plan: ")
		sb.WriteString(planText)
		sb.WriteString("\n")
	}
	sb.WriteString("</plan_review_pending>")

	r.state.Messages = append(r.state.Messages, Message{
		Role:    "system",
		Content: sb.String(),
		Source:  ports.MessageSourceSystemPrompt,
	})
}

func (r *reactRuntime) filterValidToolCalls(toolCalls []ToolCall) []ToolCall {
	var validCalls []ToolCall
	for _, tc := range toolCalls {
		if strings.Contains(tc.Name, "<|") || strings.Contains(tc.Name, "functions.") || strings.Contains(tc.Name, "user<") {
			r.engine.logger.Warn("Filtering out invalid tool call with leaked markers: %s", tc.Name)
			continue
		}
		validCalls = append(validCalls, tc)
		r.engine.logger.Debug("Tool call: %s (id=%s)", tc.Name, tc.ID)
	}
	return validCalls
}

func (r *reactRuntime) finishWorkflow(stopReason string, result *TaskResult, err error) {
	r.finalize.Do(func() {
		if r.tracker != nil {
			r.tracker.finalize(stopReason, result, err)
		}
	})
}

func (it *reactIteration) think() error {
	tracker := it.runtime.tracker
	state := it.runtime.state
	services := it.runtime.services

	tracker.startThink(it.index)

	it.runtime.engine.emitEvent(&domain.WorkflowNodeStartedEvent{
		BaseEvent:  it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
		Iteration:  it.index,
		TotalIters: it.runtime.engine.maxIterations,
	})

	it.runtime.engine.logger.Debug("THINK phase: Calling LLM with %d messages", len(state.Messages))
	it.runtime.engine.emitEvent(&domain.WorkflowNodeOutputDeltaEvent{
		BaseEvent:    it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
		Iteration:    it.index,
		MessageCount: len(state.Messages),
	})

	thought, err := it.runtime.engine.think(it.runtime.ctx, state, services)
	if err != nil {
		it.runtime.engine.logger.Error("Think step failed: %v", err)

		it.runtime.engine.emitEvent(&domain.WorkflowNodeFailedEvent{
			BaseEvent:   it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
			Iteration:   it.index,
			Phase:       "think",
			Error:       err,
			Recoverable: false,
		})
		tracker.completeThink(it.index, Message{}, nil, err)
		it.runtime.finishWorkflow("error", nil, err)
		return fmt.Errorf("think step failed: %w", err)
	}

	parsedCalls := it.runtime.engine.parseToolCalls(thought, services.Parser)
	it.runtime.engine.logger.Info("Parsed %d tool calls", len(parsedCalls))

	validCalls := it.runtime.filterValidToolCalls(parsedCalls)
	if retry, prompt := it.runtime.enforceOrchestratorGates(validCalls); retry {
		it.runtime.injectOrchestratorCorrection(prompt)
		tracker.completeThink(it.index, Message{}, nil, nil)
		it.toolCalls = nil
		it.thought = Message{}
		return nil
	}

	it.recordThought(&thought)
	it.toolCalls = validCalls

	tracker.completeThink(it.index, thought, it.toolCalls, nil)

	hasPlanCall := false
	for _, call := range it.toolCalls {
		if call.Name == "plan" {
			hasPlanCall = true
			break
		}
	}

	if len(it.toolCalls) > 0 && !hasPlanCall {
		meta := extractLLMMetadata(thought.Metadata)
		it.runtime.engine.emitEvent(&domain.WorkflowNodeOutputSummaryEvent{
			BaseEvent:     it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
			Iteration:     it.index,
			Content:       thought.Content,
			ToolCallCount: len(it.toolCalls),
			Metadata:      meta,
		})
	}

	it.thought = thought
	return nil
}

func (it *reactIteration) planTools() (*TaskResult, bool, error) {
	it.runtime.tracker.startPlan(it.index, len(it.toolCalls))

	validCalls := it.runtime.filterValidToolCalls(it.toolCalls)
	it.runtime.tracker.completePlan(it.index, validCalls, nil)
	if len(validCalls) == 0 {
		return it.handleNoTools()
	}

	it.plan = toolExecutionPlan{
		iteration: it.index,
		nodeID:    iterationToolsNode(it.index),
		calls:     validCalls,
	}

	it.runtime.tracker.startTools(it.plan.iteration, it.plan.nodeID, len(validCalls))

	if it.thought.Content != "" {
		it.thought.Content = it.runtime.engine.cleanToolCallMarkers(it.thought.Content)
	}

	it.runtime.engine.logger.Debug("EXECUTE phase: Running %d tools in parallel", len(validCalls))
	it.runtime.emitWorkflowToolStartedEvents(validCalls)

	return nil, false, nil
}

func (it *reactIteration) executeTools() {
	if len(it.plan.calls) == 0 {
		return
	}

	it.runtime.engine.saveCheckpoint(it.runtime.ctx, it.runtime.state, pendingToolStates(it.plan.calls))
	it.toolResult = newToolCallBatch(
		it.runtime.engine,
		it.runtime.ctx,
		it.runtime.state,
		it.plan.iteration,
		it.plan.calls,
		it.runtime.services.ToolExecutor,
		it.runtime.services.ToolLimiter,
		it.runtime.tracker,
	).execute()
}

func (it *reactIteration) observeTools() {
	if len(it.plan.calls) == 0 {
		return
	}

	state := it.runtime.state
	state.ToolResults = append(state.ToolResults, it.toolResult...)
	it.runtime.engine.observeToolResults(it.runtime.ctx, state, it.plan.iteration, it.toolResult)
	it.runtime.engine.updateGoalPlanPrompts(state, it.plan.calls, it.toolResult)

	for i, res := range it.toolResult {
		if res.Error != nil {
			it.runtime.engine.logger.Warn("Tool %d failed: %v", i, res.Error)
			continue
		}
		it.runtime.engine.logger.Debug("Tool %d succeeded: result_length=%d", i, len(res.Content))
	}

	toolMessages := it.runtime.engine.buildToolMessages(it.toolResult)
	toolMessages = it.runtime.engine.appendGoalPlanReminder(state, toolMessages)
	startIdx := len(state.Messages)
	state.Messages = append(state.Messages, toolMessages...)
	attachmentsChanged := false
	for i := range toolMessages {
		if registerMessageAttachments(it.runtime.ctx, state, &state.Messages[startIdx+i], it.runtime.engine.attachmentPersister) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		it.runtime.engine.updateAttachmentCatalogMessage(state)
	}
	offloadMessageAttachmentData(state)
	it.runtime.engine.logger.Debug("OBSERVE phase: Added %d tool message(s) to state", len(toolMessages))

	it.runtime.updateOrchestratorState(it.plan.calls, it.toolResult)
	it.runtime.tracker.completeTools(it.plan.iteration, it.plan.nodeID, it.toolResult, nil)
}

func (it *reactIteration) finish() {
	state := it.runtime.state
	services := it.runtime.services

	tokenCount := services.Context.EstimateTokens(state.Messages)
	state.TokenCount = tokenCount
	it.runtime.engine.logger.Debug("Current token count: %d", tokenCount)

	it.runtime.engine.emitEvent(&domain.WorkflowNodeCompletedEvent{
		BaseEvent:  it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
		Iteration:  it.index,
		TokensUsed: state.TokenCount,
		ToolsRun:   len(it.toolResult),
	})

	it.runtime.engine.logger.Debug("Iteration %d complete, continuing to next iteration", it.index)
}

func (it *reactIteration) handleNoTools() (*TaskResult, bool, error) {
	trimmed := strings.TrimSpace(it.thought.Content)
	if trimmed == "" {
		it.runtime.engine.logger.Warn("No tool calls and empty content - continuing loop")
		return nil, false, nil
	}

	if it.runtime.shouldTriggerFinalAnswerReview() {
		it.runtime.engine.logger.Info("Triggering final answer review iteration (attempt=%d)", it.runtime.finalReviewAttempts+1)
		it.runtime.injectFinalAnswerReviewPrompt()
		return nil, false, nil
	}

	it.runtime.engine.logger.Info("No tool calls with content - treating response as final answer")
	finalResult := it.runtime.finalizeResult("final_answer", nil, true, nil)
	return finalResult, true, nil
}

func (it *reactIteration) recordThought(thought *Message) {
	if thought == nil {
		return
	}

	state := it.runtime.state

	if att := resolveContentAttachments(thought.Content, state); len(att) > 0 {
		thought.Attachments = att
	}
	trimmed := strings.TrimSpace(thought.Content)
	if trimmed != "" || len(thought.ToolCalls) > 0 || len(thought.Thinking.Parts) > 0 {
		state.Messages = append(state.Messages, *thought)
	}
	it.runtime.engine.logger.Debug("LLM response: content_length=%d, tool_calls=%d", len(thought.Content), len(thought.ToolCalls))
}

func (r *reactRuntime) finalizeResult(stopReason string, result *TaskResult, emitCompletionEvent bool, workflowErr error) *TaskResult {
	r.finalizer.Do(func() {
		r.completeFinalAnswerReviewToolEvent("review complete")

		if result == nil {
			result = r.engine.finalize(r.state, stopReason, r.engine.clock.Now().Sub(r.startTime))
		} else {
			result.StopReason = stopReason
			if result.Duration == 0 {
				result.Duration = r.engine.clock.Now().Sub(r.startTime)
			}
		}

		attachments := r.engine.decorateFinalResult(r.state, result)
		if emitCompletionEvent {
			r.emitFinalAnswerStream(stopReason, result)
			r.engine.emitEvent(&domain.WorkflowResultFinalEvent{
				BaseEvent:       r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
				FinalAnswer:     result.Answer,
				TotalIterations: result.Iterations,
				TotalTokens:     result.TokensUsed,
				StopReason:      stopReason,
				Duration:        result.Duration,
				StreamFinished:  true,
				Attachments:     attachments,
			})
		}

		r.engine.clearCheckpoint(r.ctx, r.state.SessionID)
		r.finishWorkflow(stopReason, result, workflowErr)
	})

	return result
}

func (r *reactRuntime) emitFinalAnswerStream(stopReason string, result *TaskResult) {
	if result == nil {
		return
	}
	answer := result.Answer
	if strings.TrimSpace(answer) == "" {
		return
	}

	const chunkSize = 800
	runes := []rune(answer)
	if len(runes) == 0 {
		return
	}

	var builder strings.Builder
	for i := 0; i < len(runes); i += chunkSize {
		end := i + chunkSize
		if end > len(runes) {
			end = len(runes)
		}
		builder.WriteString(string(runes[i:end]))
		r.engine.emitEvent(&domain.WorkflowResultFinalEvent{
			BaseEvent:       r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
			FinalAnswer:     builder.String(),
			TotalIterations: result.Iterations,
			TotalTokens:     result.TokensUsed,
			StopReason:      stopReason,
			Duration:        result.Duration,
			IsStreaming:     true,
			StreamFinished:  false,
		})
	}
}

func extractLLMMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	out := make(map[string]any)
	for _, key := range []string{"llm_duration_ms", "llm_request_id", "llm_model"} {
		if val, ok := metadata[key]; ok {
			out[key] = val
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func (r *reactRuntime) applyIterationHook(iteration int) {
	if r.engine.iterationHook == nil || iteration == 0 {
		return
	}
	result := r.engine.iterationHook.OnIteration(r.ctx, r.state, iteration)
	if result.MemoriesInjected <= 0 {
		return
	}
	r.engine.emitEvent(&domain.ProactiveContextRefreshEvent{
		BaseEvent:        r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		Iteration:        iteration,
		MemoriesInjected: result.MemoriesInjected,
	})
}
