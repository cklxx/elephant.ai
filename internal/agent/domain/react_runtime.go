package domain

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
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

	// UI orchestration state (Plan → Clearify → ReAct → Finalize).
	runID           string
	planEmitted     bool
	planVersion     int
	currentTaskID   string
	clearifyEmitted map[string]bool
	pendingTaskID   string
	nextTaskSeq     int
	pauseRequested  bool
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
		runtime.runID = strings.TrimSpace(state.TaskID)
	}
	runtime.clearifyEmitted = make(map[string]bool)
	runtime.nextTaskSeq = 1
	return runtime
}

func (r *reactRuntime) run() (*TaskResult, error) {
	r.tracker.startContext(r.task)
	r.prepareContext()

	for r.state.Iterations < r.engine.maxIterations {
		if result, stop, err := r.handleCancellation(); stop || err != nil {
			return result, err
		}

		r.state.Iterations++
		r.engine.logger.Info("=== Iteration %d/%d ===", r.state.Iterations, r.engine.maxIterations)

		result, done, err := r.runIteration()
		if err != nil {
			return nil, err
		}
		if done {
			return result, nil
		}
	}

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
		r.engine.emitEvent(&WorkflowToolStartedEvent{
			BaseEvent: r.engine.newBaseEvent(r.ctx, state.SessionID, state.TaskID, state.ParentTaskID),
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
	if len(calls) > 1 {
		return true, "每轮仅允许 1 次工具调用。请只保留一个最关键的工具调用并重试。"
	}

	name := strings.ToLower(strings.TrimSpace(calls[0].Name))
	switch name {
	case "plan":
		return false, ""
	case "clearify":
		if !r.planEmitted {
			return true, r.planGatePrompt()
		}
		return false, ""
	default:
		if !r.planEmitted {
			return true, r.planGatePrompt()
		}
		if r.currentTaskID == "" || !r.clearifyEmitted[r.currentTaskID] {
			return true, r.clearifyGatePrompt()
		}
		return false, ""
	}
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
- overall_goal_ui: 目标/范围描述（complex 可多行；simple 必须单行）
- internal_plan: (可选) 仅放结构化计划，不要在 overall_goal_ui 列任务清单
plan() 成功后再继续。`, runID))
}

func (r *reactRuntime) clearifyGatePrompt() string {
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

	return strings.TrimSpace(fmt.Sprintf(`在调用动作工具前必须先调用 clearify() 声明当前任务。
请先调用 clearify()（仅此一个工具调用），并满足：
- run_id: %q
- task_id: %q
- task_goal_ui: 描述你接下来要做的具体任务
- success_criteria: (可选) 字符串数组
如需用户补充信息：needs_user_input=true 并提供 question_to_user。
clearify() 成功后再继续。`, runID, taskID))
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
		case "clearify":
			if result.Metadata == nil {
				continue
			}
			taskID, _ := result.Metadata["task_id"].(string)
			taskID = strings.TrimSpace(taskID)
			if taskID == "" {
				continue
			}
			r.currentTaskID = taskID
			r.clearifyEmitted[taskID] = true
			r.pendingTaskID = ""

			if needs, ok := result.Metadata["needs_user_input"].(bool); ok && needs {
				r.pauseRequested = true
			}
		}
	}
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

	it.runtime.engine.emitEvent(&WorkflowNodeStartedEvent{
		BaseEvent:  it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:  it.index,
		TotalIters: it.runtime.engine.maxIterations,
	})

	it.runtime.engine.logger.Debug("THINK phase: Calling LLM with %d messages", len(state.Messages))
	it.runtime.engine.emitEvent(&WorkflowNodeOutputDeltaEvent{
		BaseEvent:    it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:    it.index,
		MessageCount: len(state.Messages),
	})

	thought, err := it.runtime.engine.think(it.runtime.ctx, state, services)
	if err != nil {
		it.runtime.engine.logger.Error("Think step failed: %v", err)

		it.runtime.engine.emitEvent(&WorkflowNodeFailedEvent{
			BaseEvent:   it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.TaskID, state.ParentTaskID),
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

	it.runtime.engine.emitEvent(&WorkflowNodeOutputSummaryEvent{
		BaseEvent:     it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:     it.index,
		Content:       thought.Content,
		ToolCallCount: len(it.toolCalls),
	})

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

	for i, res := range it.toolResult {
		if res.Error != nil {
			it.runtime.engine.logger.Warn("Tool %d failed: %v", i, res.Error)
			continue
		}
		it.runtime.engine.logger.Debug("Tool %d succeeded: result_length=%d", i, len(res.Content))
	}

	toolMessages := it.runtime.engine.buildToolMessages(it.toolResult)
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

	it.runtime.engine.emitEvent(&WorkflowNodeCompletedEvent{
		BaseEvent:  it.runtime.engine.newBaseEvent(it.runtime.ctx, state.SessionID, state.TaskID, state.ParentTaskID),
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
	if trimmed := strings.TrimSpace(thought.Content); trimmed != "" || len(thought.ToolCalls) > 0 {
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
			r.engine.emitEvent(&WorkflowResultFinalEvent{
				BaseEvent:       r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.TaskID, r.state.ParentTaskID),
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
