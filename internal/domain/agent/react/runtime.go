package react

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/utils"

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

	// Background task manager for async agent execution.
	bgManager      *BackgroundTaskManager
	bgManagerOwned bool
	// Track emitted completion events to avoid duplicates.
	bgCompletionEmitted map[string]bool
	// External input requests from interactive external agents.
	externalInputCh      <-chan agent.InputRequest
	externalInputEmitted map[string]bool

	// User input channel for live message injection from chat gateways.
	userInputCh <-chan agent.UserInput
	// Consecutive non-recoverable tool failures used to prevent retry loops.
	lastNonRetryableToolFailure  string
	consecutiveNonRetryableFails int
}

const (
	planStatusPending    = "pending"
	planStatusInProgress = "in_progress"
	planStatusBlocked    = "blocked"
	planStatusCompleted  = "completed"
)

const repeatedNonRetryableToolFailureThreshold = 3

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
				0,
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

	// Inject team definitions for run_tasks tool.
	if len(r.engine.teamDefinitions) > 0 {
		r.ctx = agent.WithTeamDefinitions(r.ctx, r.engine.teamDefinitions)
	}
	if r.engine.teamRunRecorder != nil {
		r.ctx = agent.WithTeamRunRecorder(r.ctx, r.engine.teamRunRecorder)
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
		r.engine.logger.Debug("Iteration %d/%d", r.state.Iterations, r.engine.maxIterations)

		result, done, err := r.runIteration()

		// Async persist session state after successful iteration (best-effort).
		r.persistSessionAfterIteration()

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

	r.engine.logger.Debug("Context cancelled, stopping execution: %v", r.ctx.Err())
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
	if stopResult, stop := r.maybeStopAfterRepeatedToolFailures(iteration.toolResult); stop {
		return stopResult, true, nil
	}
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
		r.engine.emitEvent(domain.NewToolStartedEvent(
			r.engine.newBaseEvent(r.ctx, state.SessionID, state.RunID, state.ParentRunID),
			state.Iterations, call.ID, call.Name, call.Arguments,
		))
	}
}

func (r *reactRuntime) handleMaxIterations() (*TaskResult, error) {
	r.engine.logger.Warn("Max iterations (%d) reached, requesting final answer", r.engine.maxIterations)
	finalResult := r.engine.finalize(r.state, "max_iterations", r.engine.clock.Now().Sub(r.startTime))

	if utils.IsBlank(finalResult.Answer) {
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
			r.engine.logger.Debug("Final answer retry produced %d chars", len(finalResult.Answer))
		}
	}

	return r.finalizeResult("max_iterations", finalResult, true, nil), nil
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

		name := utils.TrimLower(call.Name)
		switch name {
		case "plan":
			r.planEmitted = true
			r.planVersion++
			if raw, ok := call.Arguments["complexity"].(string); ok {
				complexity := utils.TrimLower(raw)
				if complexity == "simple" || complexity == "complex" {
					r.planComplexity = complexity
				}
			} else if result.Metadata != nil {
				if raw, ok := result.Metadata["complexity"].(string); ok {
					complexity := utils.TrimLower(raw)
					if complexity == "simple" || complexity == "complex" {
						r.planComplexity = complexity
					}
				}
			}
			r.maybeTriggerPlanReview(call, result)
		case "ask_user":
			r.handleClarifyResult(result)
			if result.Metadata != nil {
				if needs, ok := result.Metadata["needs_user_input"].(bool); ok && needs {
					r.pauseRequested = true
				}
			}
		}
	}
}

func (r *reactRuntime) handleToolError(_ ToolCall, _ ToolResult) {
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
	if utils.IsBlank(node.ID) {
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
	if utils.IsBlank(id) {
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
		if data, err := r.engine.jsonCodec(internalPlan); err == nil {
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

	it.runtime.engine.emitEvent(domain.NewNodeStartedEvent(
		it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
		it.index, it.runtime.engine.maxIterations, 0, "", nil, nil,
	))

	it.runtime.engine.logger.Debug("THINK phase: Calling LLM with %d messages", len(state.Messages))
	it.runtime.engine.emitEvent(domain.NewNodeOutputDeltaEvent(
		it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
		it.index, len(state.Messages), "", false, time.Time{}, "",
	))

	thought, err := it.runtime.engine.think(it.runtime.ctx, state, services)
	if err != nil {
		classification := classifyContextOverflow(err)
		if classification.Matched {
			it.runtime.engine.logger.Warn(
				"Context overflow detected (rule=%s), applying compaction plan and retrying think step",
				classification.Rule,
			)
			if compacted, ok := it.runtime.engine.tryArtifactCompaction(
				it.runtime.ctx,
				state,
				services,
				state.Messages,
				compactionReasonOverflow,
				true,
			); ok {
				state.Messages = compacted
			} else {
				// Fallback when artifact compaction cannot run (e.g. missing state/session).
				emergencyTrimState(state, services)
			}
			thought, err = it.runtime.engine.think(it.runtime.ctx, state, services)
		}
	}
	if err != nil {
		it.runtime.engine.logger.Error("Think step failed: %v", err)

		it.runtime.engine.emitEvent(domain.NewNodeFailedEvent(
			it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
			it.index, "think", err, false,
		))
		tracker.completeThink(it.index, Message{}, nil, err)
		it.runtime.finishWorkflow("error", nil, err)
		return fmt.Errorf("think step failed: %w", err)
	}

	parsedCalls := it.runtime.engine.parseToolCalls(thought, services.Parser)
	it.runtime.engine.logger.Debug("Parsed %d tool calls", len(parsedCalls))

	validCalls := it.runtime.filterValidToolCalls(parsedCalls)
	if len(validCalls) > 0 {
		thought.ToolCalls = append([]ToolCall(nil), validCalls...)
	} else {
		thought.ToolCalls = nil
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
		it.runtime.engine.emitEvent(domain.NewNodeOutputSummaryEvent(
			it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
			it.index, thought.Content, len(it.toolCalls), meta,
		))
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

	// Apply context checkpoint if requested in this iteration.
	if it.runtime.engine.applyContextCheckpoint(
		it.runtime.ctx, state, it.runtime.services,
		it.toolResult, it.plan.calls,
	) {
		it.runtime.engine.logger.Debug("Context checkpoint applied — pruned intermediate messages")
	}

	it.runtime.updateOrchestratorState(it.plan.calls, it.toolResult)
	it.runtime.tracker.completeTools(it.plan.iteration, it.plan.nodeID, it.toolResult, nil)
}

func (r *reactRuntime) maybeStopAfterRepeatedToolFailures(results []ToolResult) (*TaskResult, bool) {
	if r == nil || len(results) == 0 {
		return nil, false
	}

	hasSuccess := false
	matchedNonRetryable := false

	for i := range results {
		res := results[i]
		if res.Error == nil {
			hasSuccess = true
			continue
		}

		failure, ok := classifyNonRetryableToolFailure(res.Error)
		if !ok {
			continue
		}
		matchedNonRetryable = true

		if failure.signature == r.lastNonRetryableToolFailure {
			r.consecutiveNonRetryableFails++
		} else {
			r.lastNonRetryableToolFailure = failure.signature
			r.consecutiveNonRetryableFails = 1
		}

		if r.consecutiveNonRetryableFails >= repeatedNonRetryableToolFailureThreshold {
			r.engine.logger.Warn(
				"Stopping after repeated non-recoverable tool failure: signature=%s count=%d",
				failure.signature,
				r.consecutiveNonRetryableFails,
			)
			return r.finalizeRepeatedToolFailure(failure.hint, res.Error), true
		}
	}

	if hasSuccess || !matchedNonRetryable {
		r.resetNonRetryableToolFailures()
	}

	return nil, false
}

func (r *reactRuntime) finalizeRepeatedToolFailure(hint string, lastErr error) *TaskResult {
	result := r.engine.finalize(r.state, "repeated_tool_failure", r.engine.clock.Now().Sub(r.startTime))

	var summary strings.Builder
	summary.WriteString("Stopped after repeated non-recoverable tool errors to avoid retry loops.")
	if trimmed := strings.TrimSpace(hint); trimmed != "" {
		summary.WriteString("\n")
		summary.WriteString(trimmed)
	}
	if lastErr != nil {
		summary.WriteString("\nLast error: ")
		summary.WriteString(strings.TrimSpace(lastErr.Error()))
	}
	result.Answer = strings.TrimSpace(summary.String())

	r.resetNonRetryableToolFailures()
	return r.finalizeResult("repeated_tool_failure", result, true, nil)
}

func (r *reactRuntime) resetNonRetryableToolFailures() {
	if r == nil {
		return
	}
	r.lastNonRetryableToolFailure = ""
	r.consecutiveNonRetryableFails = 0
}

type nonRetryableToolFailure struct {
	signature string
	hint      string
}

func classifyNonRetryableToolFailure(err error) (nonRetryableToolFailure, bool) {
	if err == nil {
		return nonRetryableToolFailure{}, false
	}

	text := utils.TrimLower(err.Error())
	if text == "" {
		return nonRetryableToolFailure{}, false
	}

	switch {
	case strings.Contains(text, "path must stay within the working directory"):
		return nonRetryableToolFailure{
			signature: "path_guard",
			hint:      "Use relative paths or set exec_dir under the current working directory.",
		}, true
	case strings.Contains(text, "template \"") && strings.Contains(text, "not found"):
		return nonRetryableToolFailure{
			signature: "template_not_found",
			hint:      "Call run_tasks(template=\"list\") first, then choose one of the listed templates.",
		}, true
	default:
		return nonRetryableToolFailure{}, false
	}
}

func (it *reactIteration) finish() {
	state := it.runtime.state
	services := it.runtime.services

	tokenCount := services.Context.EstimateTokens(state.Messages)
	state.TokenCount = tokenCount
	it.runtime.engine.logger.Debug("Current token count: %d", tokenCount)

	it.runtime.engine.emitEvent(domain.NewNodeCompletedEvent(
		it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.RunID, state.ParentRunID),
		0, "", nil, "", it.index, state.TokenCount, len(it.toolResult), 0, nil,
	))

	it.runtime.engine.logger.Debug("Iteration %d complete", it.index)
}

func (it *reactIteration) handleNoTools() (*TaskResult, bool, error) {
	trimmed := strings.TrimSpace(it.thought.Content)
	if trimmed == "" {
		it.runtime.engine.logger.Debug("No tool calls and empty content, continuing loop")
		return nil, false, nil
	}

	it.runtime.engine.logger.Debug("No tool calls with content, treating response as final answer")
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
	if trimmed != "" || len(thought.ToolCalls) > 0 {
		state.Messages = append(state.Messages, *thought)
	}
	it.runtime.engine.logger.Debug("LLM response: content_length=%d, tool_calls=%d", len(thought.Content), len(thought.ToolCalls))
}

func (r *reactRuntime) finalizeResult(stopReason string, result *TaskResult, emitCompletionEvent bool, workflowErr error) *TaskResult {
	r.finalizer.Do(func() {
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
			r.engine.emitEvent(domain.NewResultFinalEvent(
				r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
				result.Answer, result.Iterations, result.TokensUsed, stopReason,
				result.Duration, false, true, attachments,
			))
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
	if utils.IsBlank(answer) {
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
		r.engine.emitEvent(domain.NewResultFinalEvent(
			r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
			builder.String(), result.Iterations, result.TokensUsed, stopReason,
			result.Duration, true, false, nil,
		))
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
	r.engine.emitEvent(domain.NewProactiveContextRefreshEvent(
		r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		iteration, result.MemoriesInjected,
	))
}

// persistSessionAfterIteration calls the optional sessionPersister callback
// to asynchronously persist session state after each successful iteration.
// The callback is responsible for async behavior and error handling.
func (r *reactRuntime) persistSessionAfterIteration() {
	if r.engine.sessionPersister != nil {
		r.engine.sessionPersister(r.ctx, nil, r.state)
	}
}

// isContextLengthExceeded checks whether the error indicates the LLM provider
// rejected the request because the input exceeded the model's context window.
// Maintained as a compatibility wrapper for existing tests.
func isContextLengthExceeded(err error) bool {
	return classifyContextOverflow(err).Matched
}

// emergencyTrimState applies aggressive trimming to state.Messages when the
// LLM rejects the request due to context length. This is the last-resort
// safety net after pre-flight enforcement has already been attempted.
func emergencyTrimState(state *TaskState, services Services) {
	if state == nil {
		return
	}
	trimmed := aggressiveTrimMessages(state.Messages, 2)
	state.Messages = trimmed
}
