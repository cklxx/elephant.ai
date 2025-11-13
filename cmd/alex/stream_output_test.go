package main

import (
	"bytes"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/tools/builtin"
)

func TestHandleSubtaskEventTracksProgress(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	startEvent := &builtin.SubtaskEvent{
		OriginalEvent: &domain.ToolCallStartEvent{
			CallID:   "call-1",
			ToolName: "test-tool",
			Arguments: map[string]interface{}{
				"path": "file.txt",
			},
		},
		SubtaskIndex:   0,
		TotalSubtasks:  2,
		SubtaskPreview: "process file contents to extract summary data",
	}

	handler.handleSubtaskEvent(startEvent)
	output := out.String()
	require.Contains(t, output, "Subagent: Running 2 tasks")
	require.Contains(t, output, "→ Task 1")
	out.Reset()

	completeEvent := &builtin.SubtaskEvent{
		OriginalEvent: &domain.ToolCallCompleteEvent{
			CallID:   "call-1",
			ToolName: "test-tool",
			Result:   "ok",
			Duration: time.Millisecond,
		},
		SubtaskIndex: 0,
	}

	handler.handleSubtaskEvent(completeEvent)
	require.Equal(t, "", out.String(), "tool completion should not emit output directly")

	taskCompleteEvent := &builtin.SubtaskEvent{
		OriginalEvent: &domain.TaskCompleteEvent{
			TotalTokens: 128,
		},
		SubtaskIndex:  0,
		TotalSubtasks: 2,
	}

	handler.handleSubtaskEvent(taskCompleteEvent)
	output = out.String()
	require.Contains(t, output, "✓ [1/2] Task 1")
	require.Contains(t, output, "| 128 tokens | 1 tool")
}

func TestHandleSubtaskEventHandlesErrors(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	errEvent := &builtin.SubtaskEvent{
		OriginalEvent: &domain.ErrorEvent{
			Error: errors.New("boom"),
		},
		SubtaskIndex:   1,
		TotalSubtasks:  3,
		SubtaskPreview: "failing subtask",
	}

	handler.handleSubtaskEvent(errEvent)
	output := out.String()
	require.Contains(t, output, "Subagent: Running 3 tasks")
	require.Contains(t, output, "✗ [1/3] Task 2")
	require.Contains(t, output, "boom")
}

func TestStreamingOutputHandlerStoresCompletionEvent(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	event := &domain.TaskCompleteEvent{TotalIterations: 4, TotalTokens: 512}

	handler.onTaskComplete(event)

	stored := handler.consumeTaskCompletion()
	require.Equal(t, event, stored)

	// Subsequent calls should return nil
	require.Nil(t, handler.consumeTaskCompletion())
}

func TestStreamingOutputHandlerPrintsInterruptMessages(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	handler.printInterruptRequested()
	handler.printForcedExit()

	output := out.String()
	require.Contains(t, output, "Interrupt requested")
	require.Contains(t, output, "Force exit requested")
}

func TestStreamingOutputHandlerPrintCancellation(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	event := &domain.TaskCompleteEvent{TotalIterations: 3, TotalTokens: 256}
	handler.printCancellation(event)

	output := out.String()
	require.Contains(t, output, "Task interrupted")
	require.Contains(t, output, "3 iteration")
	require.Contains(t, output, "256 tokens")
}

func TestStreamingOutputHandlerAssistantMessageStream(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	handler.onAssistantMessage(&domain.AssistantMessageEvent{Delta: "Hello", Final: false})
	handler.onAssistantMessage(&domain.AssistantMessageEvent{Final: true})

	require.Contains(t, out.String(), "Hello")
	require.True(t, strings.HasSuffix(out.String(), "\n"))
	require.True(t, handler.streamedContent)
}

func TestStreamingOutputHandlerAssistantMessageBuffersMarkdownLines(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	handler.onAssistantMessage(&domain.AssistantMessageEvent{Delta: "Hello\nWorld", Final: false})
	handler.onAssistantMessage(&domain.AssistantMessageEvent{Final: true})

	require.Contains(t, out.String(), "Hello")
	require.Contains(t, out.String(), "World")
	require.True(t, strings.HasSuffix(out.String(), "\n"))
}

func TestStreamingOutputHandlerPrintCompletionResetsStreamedContent(t *testing.T) {
	handler := NewStreamingOutputHandler(nil, false)
	var out bytes.Buffer
	handler.SetOutputWriter(&out)

	handler.streamedContent = true
	streamedResult := &ports.TaskResult{
		Answer:     "streamed answer",
		Iterations: 1,
		TokensUsed: 5,
	}

	handler.printCompletion(streamedResult)

	firstOutput := out.String()
	require.NotContains(t, firstOutput, streamedResult.Answer)
	require.False(t, handler.streamedContent)

	out.Reset()

	nonStreamedResult := &ports.TaskResult{
		Answer:     "final answer",
		Iterations: 2,
		TokensUsed: 8,
	}

	handler.printCompletion(nonStreamedResult)

	secondOutput := out.String()
	require.Contains(t, secondOutput, nonStreamedResult.Answer)
	require.False(t, handler.streamedContent)
}
