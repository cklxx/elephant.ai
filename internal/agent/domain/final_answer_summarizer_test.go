package domain

import (
	"context"
	"strings"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/agent/ports/mocks"
)

func TestFinalAnswerSummarizerSummarizesWithoutStreaming(t *testing.T) {
	clock := ports.ClockFunc(func() time.Time { return time.Unix(1_700_000_000, 0) })
	summarizer := NewFinalAnswerSummarizer(ports.NoopLogger{}, clock)

	state := &ports.TaskState{
		Messages: []ports.Message{
			{Role: "system", Content: "SYSTEM_PROMPT", Source: ports.MessageSourceSystemPrompt},
			{Role: "user", Content: "Find the latest report", Source: ports.MessageSourceUserInput},
			{Role: "assistant", Content: "Report ready [report.pdf]", Source: ports.MessageSourceAssistantReply},
		},
		SessionID:            "sess-123",
		TaskID:               "task-abc",
                Attachments: map[string]ports.Attachment{
                        "report.pdf": {Name: "report.pdf", MediaType: "application/pdf", URI: "https://cdn.example/report.pdf"},
                },
                AttachmentIterations: map[string]int{"report.pdf": 1},
        }

	var capturedRequest ports.CompletionRequest
	mockLLM := &mocks.MockLLMClient{
		CompleteFunc: func(ctx context.Context, req ports.CompletionRequest) (*ports.CompletionResponse, error) {
			capturedRequest = req
			return &ports.CompletionResponse{Content: "Short summary [report.pdf]"}, nil
		},
	}

	env := &ports.ExecutionEnvironment{State: state, Services: ports.ServiceBundle{LLM: mockLLM}}
	result := &TaskResult{Answer: "Original final answer", StopReason: "final_answer", Iterations: 2, TokensUsed: 12, SessionID: "sess-123", TaskID: "task-abc", Duration: 5 * time.Second}

	var events []*WorkflowResultFinalEvent
	listener := EventListenerFunc(func(evt AgentEvent) {
		if tc, ok := evt.(*WorkflowResultFinalEvent); ok {
			events = append(events, tc)
		}
	})

	updated, err := summarizer.Summarize(context.Background(), env, result, listener)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}

	if !strings.Contains(updated.Answer, "Short summary") {
		t.Fatalf("expected summarized answer to include summary text, got %q", updated.Answer)
	}
	if !strings.Contains(updated.Answer, "report.pdf") {
		t.Fatalf("expected summarized answer to preserve attachment reference, got %q", updated.Answer)
	}
	if updated.Duration != 5*time.Second {
		t.Fatalf("expected duration to be preserved, got %v", updated.Duration)
	}

        if len(events) != 1 {
                t.Fatalf("expected a single completion event without duplicate streaming payloads, got %d", len(events))
        }
        finalEvent := events[0]
        if finalEvent.StopReason != "final_answer" {
                t.Fatalf("expected stop reason to propagate, got %q", finalEvent.StopReason)
        }
        if finalEvent.IsStreaming {
                t.Fatalf("expected non-streaming completion to disable IsStreaming")
        }
        if !finalEvent.StreamFinished {
                t.Fatalf("expected non-streaming completion to be marked finished")
        }
        if len(finalEvent.Attachments) != 1 {
                t.Fatalf("expected attachment to be forwarded, got %d", len(finalEvent.Attachments))
        }
        if finalEvent.Attachments["report.pdf"].Name != "report.pdf" {
                t.Fatalf("unexpected attachment payload: %+v", finalEvent.Attachments["report.pdf"])
        }
        if strings.Contains(finalEvent.FinalAnswer, "(https://cdn.example/report.pdf)") {
                t.Fatalf("expected summarizer to avoid replacing attachment references in final answer, got %q", finalEvent.FinalAnswer)
        }

	if len(capturedRequest.Messages) != 2 {
		t.Fatalf("expected summarizer prompt to include 2 messages, got %d", len(capturedRequest.Messages))
	}
	if strings.Contains(capturedRequest.Messages[1].Content, "SYSTEM_PROMPT") {
		t.Fatalf("expected system message to be excluded from transcript: %q", capturedRequest.Messages[1].Content)
	}
}

func TestFinalAnswerSummarizerStreamsDeltas(t *testing.T) {
	clock := ports.ClockFunc(func() time.Time { return time.Unix(1_800_000_000, 0) })
	summarizer := NewFinalAnswerSummarizer(ports.NoopLogger{}, clock)

	state := &ports.TaskState{
		Messages: []ports.Message{
			{Role: "user", Content: "Need a recap", Source: ports.MessageSourceUserInput},
			{Role: "assistant", Content: "Working on it [report.pdf]", Source: ports.MessageSourceAssistantReply},
		},
		SessionID:            "sess-999",
		TaskID:               "task-999",
                Attachments: map[string]ports.Attachment{
                        "report.pdf": {Name: "report.pdf", MediaType: "application/pdf", URI: "https://cdn.example/report.pdf"},
                },
                AttachmentIterations: map[string]int{"report.pdf": 1},
        }

	streamCalled := false
	mockLLM := &mocks.MockLLMClient{
		StreamCompleteFunc: func(ctx context.Context, req ports.CompletionRequest, callbacks ports.CompletionStreamCallbacks) (*ports.CompletionResponse, error) {
			streamCalled = true
			if callbacks.OnContentDelta != nil {
				callbacks.OnContentDelta(ports.ContentDelta{Delta: "Partial "})
				callbacks.OnContentDelta(ports.ContentDelta{Delta: "answer [report.pdf]"})
				callbacks.OnContentDelta(ports.ContentDelta{Final: true})
			}
			return &ports.CompletionResponse{Content: "Partial answer [report.pdf]"}, nil
		},
	}

	env := &ports.ExecutionEnvironment{State: state, Services: ports.ServiceBundle{LLM: mockLLM}}
	result := &TaskResult{Answer: "Original final answer", StopReason: "final_answer", Iterations: 2, TokensUsed: 12, SessionID: "sess-999", TaskID: "task-999"}

	var events []*WorkflowResultFinalEvent
	listener := EventListenerFunc(func(evt AgentEvent) {
		if tc, ok := evt.(*WorkflowResultFinalEvent); ok {
			events = append(events, tc)
		}
	})

	updated, err := summarizer.Summarize(context.Background(), env, result, listener)
	if err != nil {
		t.Fatalf("Summarize returned error: %v", err)
	}

	if !streamCalled {
		t.Fatalf("expected StreamComplete to be used")
	}
	if updated.Answer == "Original final answer" {
		t.Fatalf("expected summarized answer to overwrite original")
	}
        if len(events) != 2 {
                t.Fatalf("expected one streaming update and one final event, got %d", len(events))
        }
        if events[0].FinalAnswer == "" {
                t.Fatalf("expected streaming updates to carry partial content")
        }
        if strings.Contains(events[0].FinalAnswer, "(https://cdn.example/report.pdf)") {
                t.Fatalf("expected streaming update to leave attachment references untouched, got %q", events[0].FinalAnswer)
        }
        if !events[0].IsStreaming || events[0].StreamFinished {
                t.Fatalf("expected first update to be streaming and unfinished")
        }
        last := events[len(events)-1]
        if len(last.Attachments) != 1 {
                t.Fatalf("expected final event to include attachments, got %d", len(last.Attachments))
        }
        if !strings.Contains(last.FinalAnswer, "report.pdf") {
                t.Fatalf("expected final answer to preserve attachment reference, got %q", last.FinalAnswer)
        }
        if strings.Contains(last.FinalAnswer, "(https://cdn.example/report.pdf)") {
                t.Fatalf("expected final streaming result to avoid replacing attachments, got %q", last.FinalAnswer)
        }
        if last.IsStreaming {
                t.Fatalf("expected final event to be non-streaming")
        }
        if !last.StreamFinished {
                t.Fatalf("expected final event to mark streaming as finished")
        }
}
