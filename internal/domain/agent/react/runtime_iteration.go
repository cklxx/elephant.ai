package react

import (
	"fmt"
	"strings"
	"time"

	domain "alex/internal/domain/agent"

	"go.opentelemetry.io/otel/attribute"
)

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

func (r *reactRuntime) newIteration() *reactIteration {
	return &reactIteration{runtime: r, index: r.state.Iterations}
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

	it.runtime.tracker.completePlan(it.index, it.toolCalls, nil)
	if len(it.toolCalls) == 0 {
		return it.handleNoTools()
	}

	it.plan = toolExecutionPlan{
		iteration: it.index,
		nodeID:    iterationToolsNode(it.index),
		calls:     it.toolCalls,
	}

	it.runtime.tracker.startTools(it.plan.iteration, it.plan.nodeID, len(it.toolCalls))

	if it.thought.Content != "" {
		it.thought.Content = it.runtime.engine.cleanToolCallMarkers(it.thought.Content)
	}

	it.runtime.engine.logger.Debug("EXECUTE phase: Running %d tools in parallel", len(it.toolCalls))
	it.runtime.emitWorkflowToolStartedEvents(it.toolCalls)

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

	toolMessages := it.runtime.engine.buildToolMessages(it.plan.calls, it.toolResult)
	toolMessages = appendLongRunningToolReminder(it.plan.calls, it.toolResult, toolMessages)
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
