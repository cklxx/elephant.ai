package domain

import (
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/domain/workflow"
)

// NewInputReceivedEvent constructs a user task event.
func NewInputReceivedEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	task string,
	attachments map[string]ports.Attachment,
	ts time.Time,
) *Event {
	var cloned map[string]ports.Attachment
	if len(attachments) > 0 {
		cloned = ports.CloneAttachmentMap(attachments)
	}
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventInputReceived,
		Data: EventData{
			Task:        task,
			Attachments: cloned,
		},
	}
}

// NewNodeStartedEvent constructs a node started event.
func NewNodeStartedEvent(base BaseEvent, iteration, totalIters, stepIndex int, stepDescription string, input any, wf *workflow.WorkflowSnapshot) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeStarted,
		Data: EventData{
			Iteration:       iteration,
			TotalIters:      totalIters,
			StepIndex:       stepIndex,
			StepDescription: stepDescription,
			Input:           input,
			Workflow:        wf,
		},
	}
}

// NewNodeOutputDeltaEvent constructs a streaming output delta event.
func NewNodeOutputDeltaEvent(base BaseEvent, iteration, messageCount int, delta string, final bool, createdAt time.Time, sourceModel string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeOutputDelta,
		Data: EventData{
			Iteration:    iteration,
			MessageCount: messageCount,
			Delta:        delta,
			Final:        final,
			CreatedAt:    createdAt,
			SourceModel:  sourceModel,
		},
	}
}

// NewNodeOutputSummaryEvent constructs a node output summary event.
func NewNodeOutputSummaryEvent(base BaseEvent, iteration int, content string, toolCallCount int, metadata map[string]any) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeOutputSummary,
		Data: EventData{
			Iteration:     iteration,
			Content:       content,
			ToolCallCount: toolCallCount,
			Metadata:      metadata,
		},
	}
}

// NewLifecycleUpdatedEvent constructs a workflow lifecycle updated event.
func NewLifecycleUpdatedEvent(base BaseEvent, workflowID string, wfEventType workflow.EventType, phase workflow.WorkflowPhase, node *workflow.NodeSnapshot, wf *workflow.WorkflowSnapshot) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventLifecycleUpdated,
		Data: EventData{
			WorkflowID:        workflowID,
			WorkflowEventType: wfEventType,
			Phase:             phase,
			Node:              node,
			Workflow:          wf,
		},
	}
}

// NewNodeCompletedEvent constructs a node completed event.
func NewNodeCompletedEvent(base BaseEvent, stepIndex int, stepDescription string, stepResult any, status string, iteration, tokensUsed, toolsRun int, duration time.Duration, wf *workflow.WorkflowSnapshot) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeCompleted,
		Data: EventData{
			StepIndex:       stepIndex,
			StepDescription: stepDescription,
			StepResult:      stepResult,
			Status:          status,
			Iteration:       iteration,
			TokensUsed:      tokensUsed,
			ToolsRun:        toolsRun,
			Duration:        duration,
			Workflow:        wf,
		},
	}
}

// NewToolStartedEvent constructs a tool started event.
func NewToolStartedEvent(base BaseEvent, iteration int, callID, toolName string, arguments map[string]interface{}) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventToolStarted,
		Data: EventData{
			Iteration: iteration,
			CallID:    callID,
			ToolName:  toolName,
			Arguments: arguments,
		},
	}
}

// NewToolProgressEvent constructs a tool progress event.
func NewToolProgressEvent(base BaseEvent, callID, chunk string, isComplete bool) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventToolProgress,
		Data: EventData{
			CallID:     callID,
			Chunk:      chunk,
			IsComplete: isComplete,
		},
	}
}

// NewToolCompletedEvent constructs a tool completed event.
func NewToolCompletedEvent(base BaseEvent, callID, toolName, result string, err error, duration time.Duration, metadata map[string]any, attachments map[string]ports.Attachment) *Event {
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	return &Event{
		BaseEvent: base,
		Kind:      types.EventToolCompleted,
		Data: EventData{
			CallID:      callID,
			ToolName:    toolName,
			Result:      result,
			Error:       err,
			ErrorStr:    errStr,
			Duration:    duration,
			Metadata:    metadata,
			Attachments: attachments,
		},
	}
}

// NewResultFinalEvent constructs a result final event.
func NewResultFinalEvent(base BaseEvent, finalAnswer string, totalIterations, totalTokens int, stopReason string, duration time.Duration, isStreaming, streamFinished bool, attachments map[string]ports.Attachment) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventResultFinal,
		Data: EventData{
			FinalAnswer:     finalAnswer,
			TotalIterations: totalIterations,
			TotalTokens:     totalTokens,
			StopReason:      stopReason,
			Duration:        duration,
			IsStreaming:     isStreaming,
			StreamFinished:  streamFinished,
			Attachments:     attachments,
		},
	}
}

// NewResultCancelledEvent constructs a cancellation notification event.
func NewResultCancelledEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	reason, requestedBy string,
	ts time.Time,
) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventResultCancelled,
		Data: EventData{
			Reason:      reason,
			RequestedBy: requestedBy,
		},
	}
}

// NewNodeFailedEvent constructs a node failed event.
func NewNodeFailedEvent(base BaseEvent, iteration int, phase string, err error, recoverable bool) *Event {
	var errStr string
	if err != nil {
		errStr = err.Error()
	}
	return &Event{
		BaseEvent: base,
		Kind:      types.EventNodeFailed,
		Data: EventData{
			Iteration:   iteration,
			PhaseLabel:  phase,
			Error:       err,
			ErrorStr:    errStr,
			Recoverable: recoverable,
		},
	}
}

// NewPreAnalysisEmojiEvent constructs a pre-analysis emoji event.
func NewPreAnalysisEmojiEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	reactEmoji string,
	ts time.Time,
) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticPreanalysisEmoji,
		Data: EventData{
			ReactEmoji: reactEmoji,
		},
	}
}
