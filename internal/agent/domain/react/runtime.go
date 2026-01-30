package react

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/textutil"
	"alex/internal/jsonx"
	"alex/internal/memory"
	id "alex/internal/utils/id"
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

	// Background task manager for async subagent execution.
	bgManager *BackgroundTaskManager
	// Track emitted completion events to avoid duplicates.
	bgCompletionEmitted map[string]bool
	// External input requests from interactive external agents.
	externalInputCh      <-chan agent.InputRequest
	externalInputEmitted map[string]bool

	memoryRefresh MemoryRefreshConfig
}

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
		engine:        engine,
		ctx:           ctx,
		task:          task,
		state:         state,
		services:      services,
		tracker:       newReactWorkflow(engine.workflow),
		startTime:     engine.clock.Now(),
		prepare:       prepare,
		memoryRefresh: engine.memoryRefresh,
	}
	if state != nil {
		runtime.runID = strings.TrimSpace(state.RunID)
		runtime.planReviewEnabled = state.PlanReviewEnabled
	}
	runtime.clarifyEmitted = make(map[string]bool)
	runtime.nextTaskSeq = 1

	// Initialize background task manager when executor is available.
	if engine.backgroundExecutor != nil {
		sessionID := ""
		runID := ""
		parentRunID := ""
		if state != nil {
			sessionID = state.SessionID
			runID = state.RunID
			parentRunID = state.ParentRunID
		}
		runtime.bgManager = newBackgroundTaskManager(
			ctx,
			engine.logger,
			engine.clock,
			engine.backgroundExecutor,
			engine.externalExecutor,
			engine.emitEvent,
			func(ctx context.Context) domain.BaseEvent {
				return engine.newBaseEvent(ctx, sessionID, runID, parentRunID)
			},
			sessionID,
			engine.eventListener,
		)
		runtime.bgCompletionEmitted = make(map[string]bool)
		if runtime.bgManager != nil {
			runtime.externalInputCh = runtime.bgManager.InputRequests()
			runtime.externalInputEmitted = make(map[string]bool)
		}
	}

	return runtime
}

func (r *reactRuntime) run() (*TaskResult, error) {
	r.tracker.startContext(r.task)
	r.prepareContext()

	// Set background dispatcher in context for tools.
	if r.bgManager != nil {
		r.ctx = agent.WithBackgroundDispatcher(r.ctx, newBackgroundDispatcherWithEvents(r, r.bgManager))
	}

	for r.state.Iterations < r.engine.maxIterations {
		// Inject background completion notifications before each iteration.
		r.injectBackgroundNotifications()
		r.injectExternalInputRequests()

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

func (r *reactRuntime) runIteration() (*TaskResult, bool, error) {
	iteration := r.newIteration()
	r.refreshContext(iteration.index)

	if err := iteration.think(); err != nil {
		return nil, true, err
	}

	result, done, err := iteration.planTools()
	if done || err != nil {
		return result, done, err
	}

	if len(iteration.plan.calls) == 0 && iteration.plan.nodeID == "" && result == nil {
		return nil, false, nil
	}

	iteration.executeTools()
	iteration.observeTools()
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
			if registerMessageAttachments(r.state, finalThought) {
				r.engine.updateAttachmentCatalogMessage(r.state)
			}
			finalResult.Answer = finalThought.Content
			r.engine.logger.Info("Got final answer from retry: %d chars", len(finalResult.Answer))
		}
	}

	return r.finalizeResult("max_iterations", finalResult, true, nil), nil
}

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

	if hasPlan {
		if len(calls) > 1 {
			return true, "plan() 必须单独调用。请移除同轮其它工具调用并重试。"
		}
		return false, ""
	}

	if hasClarify {
		if len(calls) > 1 {
			return true, "clarify() 必须单独调用。请移除同轮其它工具调用并重试。"
		}
		if !r.planEmitted {
			return true, r.planGatePrompt()
		}
		return false, ""
	}

	if hasRequestUser {
		if len(calls) > 1 {
			return true, "request_user() 必须单独调用。请移除同轮其它工具调用并重试。"
		}
		if !r.planEmitted {
			return true, r.planGatePrompt()
		}
		return false, ""
	}

	if !r.planEmitted {
		return true, r.planGatePrompt()
	}

	if strings.EqualFold(strings.TrimSpace(r.planComplexity), "simple") {
		return false, ""
	}
	if r.currentTaskID == "" || !r.clarifyEmitted[r.currentTaskID] {
		return true, r.clarifyGatePrompt()
	}
	return false, ""
}

func (r *reactRuntime) planGatePrompt() string {
	runID := strings.TrimSpace(r.runID)
	if runID == "" {
		runID = "<run_id>"
	}
	return strings.TrimSpace(fmt.Sprintf(`你在调用动作工具前必须先调用 plan()。
请先调用 plan()（仅此一个工具调用），并满足：
- run_id: %q
- complexity: "simple" 或 "complex"
- session_title: (可选) 会话短标题（单行，≤32字）；默认由小模型预分析生成，通常留空
- overall_goal_ui: 目标/范围描述，写清交付状态和可量化验收信号（complex 可多行；simple 必须单行）
- internal_plan: (可选) 仅放结构化计划，不要在 overall_goal_ui 列任务清单；如有验证路径/证据可在此补充
- complexity="simple" 时：plan() 后可直接调用动作工具；无需 clarify()（除非需要用户补充信息并暂停）。
- complexity="complex" 时：在每个任务的首个动作工具调用前必须 clarify()。
plan() 成功后再继续。`, runID))
}

func (r *reactRuntime) clarifyGatePrompt() string {
	runID := strings.TrimSpace(r.runID)
	if runID == "" {
		runID = "<run_id>"
	}

	taskID := strings.TrimSpace(r.pendingTaskID)
	if taskID == "" {
		taskID = fmt.Sprintf("task-%d", r.nextTaskSeq)
		r.pendingTaskID = taskID
		r.nextTaskSeq++
	}

	return strings.TrimSpace(fmt.Sprintf(`在调用动作工具前必须先调用 clarify() 声明当前任务。
请先调用 clarify()（仅此一个工具调用），并满足：
- run_id: %q
- task_id: %q
- task_goal_ui: 描述你接下来要做的具体任务
- success_criteria: (可选) 字符串数组
如需用户补充信息：needs_user_input=true 并提供 question_to_user。
clarify() 成功后再继续。`, runID, taskID))
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
			continue
		}

		name := strings.ToLower(strings.TrimSpace(call.Name))
		switch name {
		case "plan":
			r.planEmitted = true
			r.planVersion++
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
			if result.Metadata == nil {
				continue
			}
			taskID, _ := result.Metadata["task_id"].(string)
			taskID = strings.TrimSpace(taskID)
			if taskID == "" {
				continue
			}
			r.currentTaskID = taskID
			r.clarifyEmitted[taskID] = true
			r.pendingTaskID = ""

			if needs, ok := result.Metadata["needs_user_input"].(bool); ok && needs {
				r.pauseRequested = true
			}
		case "request_user":
			r.pauseRequested = true
		}
	}
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
		if data, err := jsonx.Marshal(internalPlan); err == nil {
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
	it.runtime.engine.observeToolResults(state, it.plan.iteration, it.toolResult)
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
	state.Messages = append(state.Messages, toolMessages...)
	attachmentsChanged := false
	for _, msg := range toolMessages {
		if registerMessageAttachments(state, msg) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		it.runtime.engine.updateAttachmentCatalogMessage(state)
	}
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

func (r *reactRuntime) refreshContext(iteration int) {
	cfg := r.memoryRefresh
	if !cfg.Enabled || cfg.Interval <= 0 || iteration == 0 || iteration%cfg.Interval != 0 {
		return
	}
	if r.engine.memoryService == nil {
		return
	}
	userID := strings.TrimSpace(id.UserIDFromContext(r.ctx))
	if userID == "" {
		return
	}

	keywords := extractRecentKeywords(r.state.ToolResults, 5)
	if len(keywords) == 0 {
		return
	}

	memories, err := r.engine.memoryService.Recall(r.ctx, memory.Query{
		UserID:   userID,
		Text:     strings.Join(keywords, " "),
		Keywords: keywords,
		Limit:    3,
	})
	if err != nil || len(memories) == 0 {
		return
	}

	content := formatRefreshMemories(memories, cfg.MaxTokens)
	if strings.TrimSpace(content) == "" {
		return
	}

	r.state.Messages = append(r.state.Messages, Message{
		Role:    "system",
		Content: content,
		Source:  ports.MessageSourceProactive,
	})

	r.engine.emitEvent(&domain.ProactiveContextRefreshEvent{
		BaseEvent:        r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
		Iteration:        iteration,
		MemoriesInjected: len(memories),
	})
}

func extractRecentKeywords(results []ToolResult, limit int) []string {
	if limit <= 0 || len(results) == 0 {
		return nil
	}
	start := len(results) - limit
	if start < 0 {
		start = 0
	}
	var tokens []string
	for i := start; i < len(results); i++ {
		res := results[i]
		if res.Content != "" {
			tokens = append(tokens, res.Content)
		}
	}
	return textutil.ExtractKeywords(strings.Join(tokens, " "), textutil.KeywordOptions{})
}

func formatRefreshMemories(entries []memory.Entry, maxTokens int) string {
	if len(entries) == 0 {
		return ""
	}
	var sb strings.Builder
	sb.WriteString("## Proactive Memory Refresh\n\n")
	sb.WriteString("Additional context recalled from prior work:\n\n")
	for i, entry := range entries {
		sb.WriteString(fmt.Sprintf("%d. %s\n", i+1, strings.TrimSpace(entry.Content)))
	}
	text := sb.String()
	if maxTokens <= 0 {
		return text
	}
	if estimateTokenCount(text) <= maxTokens {
		return text
	}
	return truncateToTokens(text, maxTokens)
}

func estimateTokenCount(text string) int {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return 0
	}
	return len([]rune(trimmed)) / 4
}

func truncateToTokens(text string, maxTokens int) string {
	if maxTokens <= 0 {
		return text
	}
	runes := []rune(text)
	limit := maxTokens * 4
	if limit <= 0 || limit >= len(runes) {
		return text
	}
	return string(runes[:limit]) + "..."
}
