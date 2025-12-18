package domain

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
)

// SolveTaskPlanned executes a task using a Planner + ReAct loop:
// - planner produces ordered steps (provided by caller)
// - ReAct executes each step sequentially (linear tool execution)
//
// The final summary step is expected to be handled by the application layer so
// streaming workflow.result.final behaviour can remain unchanged.
func (e *ReactEngine) SolveTaskPlanned(
	ctx context.Context,
	task string,
	steps []string,
	state *TaskState,
	services Services,
) (*TaskResult, error) {
	start := e.clock.Now()

	trimmedTask := strings.TrimSpace(task)
	if trimmedTask == "" {
		return e.finalize(state, "error", 0), fmt.Errorf("empty task")
	}

	e.prepareTaskContext(ctx, task, state)

	normalized := normalizePlannerSteps(steps)
	if len(normalized) == 0 {
		normalized = []string{"总结"}
	}
	e.emitEvent(&WorkflowPlanCreatedEvent{
		BaseEvent: e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Steps:     normalized,
	})

	summaryIndex := len(normalized) - 1
	if summaryIndex < 0 {
		summaryIndex = 0
	}

	totalIterationsBudget := e.maxIterations * maxInt(1, summaryIndex)
	globalIteration := 0

	for stepIndex := 0; stepIndex < summaryIndex; stepIndex++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		stepDesc := normalized[stepIndex]
		e.emitEvent(&WorkflowNodeStartedEvent{
			BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			StepIndex:       stepIndex,
			StepDescription: stepDesc,
			Input: map[string]any{
				"step": stepDesc,
			},
		})

		result, err := e.executePlannedStep(ctx, stepIndex, summaryIndex, stepDesc, totalIterationsBudget, &globalIteration, state, services)
		if err != nil {
			e.emitEvent(&WorkflowNodeCompletedEvent{
				BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				StepIndex:       stepIndex,
				StepDescription: stepDesc,
				StepResult:      map[string]any{"error": err.Error()},
				Status:          "failed",
				Iteration:       globalIteration,
			})
			return nil, err
		}

		e.emitEvent(&WorkflowNodeCompletedEvent{
			BaseEvent:       e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			StepIndex:       stepIndex,
			StepDescription: stepDesc,
			StepResult:      strings.TrimSpace(result),
			Status:          "succeeded",
			Iteration:       globalIteration,
		})
	}

	// Ensure we have a non-empty interim answer so downstream summarizers have a
	// stable fallback even if the summary step fails.
	if strings.TrimSpace(state.FinalAnswer) == "" {
		for i := len(state.Messages) - 1; i >= 0; i-- {
			if state.Messages[i].Role == "assistant" && strings.TrimSpace(state.Messages[i].Content) != "" {
				state.FinalAnswer = state.Messages[i].Content
				break
			}
		}
	}

	return e.finalize(state, "planned_steps_complete", e.clock.Now().Sub(start)), nil
}

func (e *ReactEngine) executePlannedStep(
	ctx context.Context,
	stepIndex int,
	totalSteps int,
	stepDescription string,
	totalIters int,
	globalIteration *int,
	state *TaskState,
	services Services,
) (string, error) {
	stepPrompt := fmt.Sprintf("第 %d/%d 步：%s\n只完成这一项；需要工具就调用；完成后用一句话给出结果（不要输出额外计划）。",
		stepIndex+1,
		maxInt(1, totalSteps),
		stepDescription,
	)
	state.Messages = append(state.Messages, Message{
		Role:    "user",
		Content: stepPrompt,
		Source:  ports.MessageSourceUserInput,
	})

	perStepIterations := 0
	for perStepIterations < e.maxIterations {
		if ctx.Err() != nil {
			return "", ctx.Err()
		}

		perStepIterations++
		*globalIteration++
		iteration := *globalIteration
		state.Iterations = iteration

		e.emitEvent(&WorkflowNodeStartedEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:  iteration,
			TotalIters: totalIters,
		})
		e.emitEvent(&WorkflowNodeOutputDeltaEvent{
			BaseEvent:    e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:    iteration,
			MessageCount: len(state.Messages),
		})

		thought, err := e.think(ctx, state, services)
		if err != nil {
			e.emitEvent(&WorkflowNodeFailedEvent{
				BaseEvent:   e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
				Iteration:   iteration,
				Phase:       "think",
				Error:       err,
				Recoverable: false,
			})
			return "", fmt.Errorf("think failed: %w", err)
		}

		recordAssistantThought(state, &thought)
		toolCalls := e.parseToolCalls(thought, services.Parser)

		e.emitEvent(&WorkflowNodeOutputSummaryEvent{
			BaseEvent:     e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:     iteration,
			Content:       thought.Content,
			ToolCallCount: len(thought.ToolCalls),
		})

		validCalls := e.filterValidToolCalls(toolCalls)
		if len(validCalls) == 0 {
			trimmed := strings.TrimSpace(thought.Content)
			if trimmed == "" {
				e.emitIterationCompleted(ctx, state, iteration, 0, services)
				continue
			}
			e.emitIterationCompleted(ctx, state, iteration, 0, services)
			return trimmed, nil
		}

		if thought.Content != "" {
			thought.Content = e.cleanToolCallMarkers(thought.Content)
		}

		e.emitWorkflowToolStartedEvents(ctx, state, iteration, validCalls)
		results := newToolCallBatch(e, ctx, state, iteration, validCalls, services.ToolExecutor, nil).execute()
		e.applyToolResults(state, iteration, results)
		e.emitIterationCompleted(ctx, state, iteration, len(results), services)
	}

	return "", fmt.Errorf("step %d exceeded max iterations (%d)", stepIndex+1, e.maxIterations)
}

func (e *ReactEngine) emitWorkflowToolStartedEvents(ctx context.Context, state *TaskState, iteration int, calls []ToolCall) {
	if len(calls) == 0 {
		return
	}
	for idx := range calls {
		call := calls[idx]
		e.emitEvent(&WorkflowToolStartedEvent{
			BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
			Iteration:  iteration,
			CallID:     call.ID,
			ToolName:   call.Name,
			Arguments:  call.Arguments,
		})
	}
}

func (e *ReactEngine) applyToolResults(state *TaskState, iteration int, results []ToolResult) {
	if state == nil || len(results) == 0 {
		return
	}

	state.ToolResults = append(state.ToolResults, results...)
	e.observeToolResults(state, iteration, results)

	toolMessages := e.buildToolMessages(results)
	state.Messages = append(state.Messages, toolMessages...)

	attachmentsChanged := false
	for _, msg := range toolMessages {
		if registerMessageAttachments(state, msg) {
			attachmentsChanged = true
		}
	}
	if attachmentsChanged {
		e.updateAttachmentCatalogMessage(state)
	}
}

func (e *ReactEngine) emitIterationCompleted(ctx context.Context, state *TaskState, iteration int, toolsRun int, services Services) {
	if state == nil || services.Context == nil {
		return
	}
	state.TokenCount = services.Context.EstimateTokens(state.Messages)
	e.emitEvent(&WorkflowNodeCompletedEvent{
		BaseEvent:  e.newBaseEvent(ctx, state.SessionID, state.TaskID, state.ParentTaskID),
		Iteration:  iteration,
		TokensUsed: state.TokenCount,
		ToolsRun:   toolsRun,
	})
}

func (e *ReactEngine) filterValidToolCalls(toolCalls []ToolCall) []ToolCall {
	var validCalls []ToolCall
	for _, tc := range toolCalls {
		if strings.Contains(tc.Name, "<|") || strings.Contains(tc.Name, "functions.") || strings.Contains(tc.Name, "user<") {
			e.logger.Warn("Filtering out invalid tool call with leaked markers: %s", tc.Name)
			continue
		}
		validCalls = append(validCalls, tc)
	}
	return validCalls
}

func recordAssistantThought(state *TaskState, thought *Message) {
	if state == nil || thought == nil {
		return
	}
	if att := resolveContentAttachments(thought.Content, state); len(att) > 0 {
		if thought.Attachments == nil {
			thought.Attachments = make(map[string]ports.Attachment, len(att))
		}
		for key, attachment := range att {
			thought.Attachments[key] = attachment
		}
	}
	if trimmed := strings.TrimSpace(thought.Content); trimmed != "" || len(thought.ToolCalls) > 0 {
		state.Messages = append(state.Messages, *thought)
	}
	registerMessageAttachments(state, *thought)
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
