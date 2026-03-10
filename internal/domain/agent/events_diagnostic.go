package domain

import (
	"fmt"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

// NewDiagnosticContextCompressionEvent creates a new context compression event.
func NewDiagnosticContextCompressionEvent(level agent.AgentLevel, sessionID, runID, parentRunID string, originalCount, compressedCount int, ts time.Time) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticContextCompression,
		Data: EventData{
			OriginalCount:   originalCount,
			CompressedCount: compressedCount,
			CompressionRate: percentageOf(compressedCount, originalCount),
		},
	}
}

// NewDiagnosticContextSnapshotEvent creates a lightweight summary of the LLM context.
// Instead of cloning all messages (which caused O(N²) memory growth during long runs),
// this stores only counts and a short preview of recent messages.
func NewDiagnosticContextSnapshotEvent(
	level agent.AgentLevel,
	sessionID, runID, parentRunID string,
	iteration int,
	llmTurnSeq int,
	requestID string,
	messages, excluded []ports.Message,
	ts time.Time,
) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticContextSnapshot,
		Data: EventData{
			Iteration:       iteration,
			LLMTurnSeq:      llmTurnSeq,
			RequestID:       requestID,
			ContextMsgCount: len(messages),
			ExcludedCount:   len(excluded),
			ContextPreview:  buildContextPreview(messages),
		},
	}
}

// buildContextPreview creates a short textual digest from the first and last
// few messages so that diagnostics remain useful without retaining the full
// message slice.
func buildContextPreview(messages []ports.Message) string {
	if len(messages) == 0 {
		return ""
	}
	const previewCount = 3
	const maxContentLen = 200

	var b strings.Builder
	writeMsg := func(label string, msg ports.Message) {
		content := strings.TrimSpace(msg.Content)
		if len(content) > maxContentLen {
			content = content[:maxContentLen] + "..."
		}
		if b.Len() > 0 {
			b.WriteString(" | ")
		}
		fmt.Fprintf(&b, "[%s] %s: %s", label, msg.Role, content)
	}

	// First N messages
	end := previewCount
	if end > len(messages) {
		end = len(messages)
	}
	for i := 0; i < end; i++ {
		writeMsg("first", messages[i])
	}

	// Last N messages (non-overlapping)
	start := len(messages) - previewCount
	if start < end {
		start = end
	}
	for i := start; i < len(messages); i++ {
		writeMsg("last", messages[i])
	}

	// Cap total preview length
	const maxPreviewLen = 2048
	s := b.String()
	if len(s) > maxPreviewLen {
		s = s[:maxPreviewLen] + "..."
	}
	return s
}

// NewDiagnosticToolFilteringEvent creates a new tool filtering event.
func NewDiagnosticToolFilteringEvent(level agent.AgentLevel, sessionID, runID, parentRunID, presetName string, originalCount, filteredCount int, filteredTools []string, ts time.Time) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(level, sessionID, runID, parentRunID, ts),
		Kind:      types.EventDiagnosticToolFiltering,
		Data: EventData{
			PresetName:      presetName,
			OriginalCount:   originalCount,
			FilteredCount:   filteredCount,
			FilteredTools:   filteredTools,
			ToolFilterRatio: percentageOf(filteredCount, originalCount),
		},
	}
}

// NewDiagnosticEnvironmentSnapshotEvent constructs a new environment snapshot event.
func NewDiagnosticEnvironmentSnapshotEvent(host map[string]string, captured time.Time) *Event {
	return &Event{
		BaseEvent: newBaseEventWithIDs(agent.LevelCore, "", "", "", captured),
		Kind:      types.EventDiagnosticEnvironmentSnapshot,
		Data: EventData{
			Host:     ports.CloneStringMap(host),
			Captured: captured,
		},
	}
}

// NewDiagnosticContextCheckpointEvent constructs a context checkpoint event.
func NewDiagnosticContextCheckpointEvent(base BaseEvent, phaseLabel string, prunedMessages, prunedTokens, summaryTokens, remainingTokens int) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventDiagnosticContextCheckpoint,
		Data: EventData{
			PhaseLabel:      phaseLabel,
			PrunedMessages:  prunedMessages,
			PrunedTokens:    prunedTokens,
			SummaryTokens:   summaryTokens,
			RemainingTokens: remainingTokens,
		},
	}
}

// NewProactiveContextRefreshEvent constructs a proactive context refresh event.
func NewProactiveContextRefreshEvent(base BaseEvent, iteration, memoriesInjected int) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventProactiveContextRefresh,
		Data: EventData{
			Iteration:        iteration,
			MemoriesInjected: memoriesInjected,
		},
	}
}

// NewBackgroundTaskDispatchedEvent constructs a background task dispatched event.
func NewBackgroundTaskDispatchedEvent(base BaseEvent, taskID, description, prompt, agentType string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventBackgroundTaskDispatched,
		Data: EventData{
			TaskID:      taskID,
			Description: description,
			Prompt:      prompt,
			AgentType:   agentType,
		},
	}
}

// NewBackgroundTaskCompletedEvent constructs a background task completed event.
func NewBackgroundTaskCompletedEvent(base BaseEvent, taskID, description, status, answer, errMsg string, duration time.Duration, iterations, tokensUsed int) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventBackgroundTaskCompleted,
		Data: EventData{
			TaskID:      taskID,
			Description: description,
			Status:      status,
			Answer:      answer,
			ErrorStr:    errMsg,
			Duration:    duration,
			Iterations:  iterations,
			TokensUsed:  tokensUsed,
		},
	}
}

// NewExternalAgentProgressEvent constructs an external agent progress event.
func NewExternalAgentProgressEvent(base BaseEvent, taskID, agentType string, iteration, maxIter, tokensUsed int, costUSD float64, currentTool, currentArgs string, filesTouched []string, lastActivity time.Time, elapsed time.Duration) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventExternalAgentProgress,
		Data: EventData{
			TaskID:       taskID,
			AgentType:    agentType,
			Iteration:    iteration,
			MaxIter:      maxIter,
			TokensUsed:   tokensUsed,
			CostUSD:      costUSD,
			CurrentTool:  currentTool,
			CurrentArgs:  currentArgs,
			FilesTouched: filesTouched,
			LastActivity: lastActivity,
			Elapsed:      elapsed,
		},
	}
}

// NewExternalInputRequestEvent constructs an external input request event.
func NewExternalInputRequestEvent(base BaseEvent, taskID, agentType, requestID, reqType, summary string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventExternalInputRequested,
		Data: EventData{
			TaskID:    taskID,
			AgentType: agentType,
			RequestID: requestID,
			Type:      reqType,
			Summary:   summary,
		},
	}
}

// NewExternalInputResponseEvent constructs an external input response event.
func NewExternalInputResponseEvent(base BaseEvent, taskID, requestID string, approved bool, optionID, message string) *Event {
	return &Event{
		BaseEvent: base,
		Kind:      types.EventExternalInputResponded,
		Data: EventData{
			TaskID:    taskID,
			RequestID: requestID,
			Approved:  approved,
			OptionID:  optionID,
			Message:   message,
		},
	}
}
