package domain

import (
	"context"
	"strings"
	"time"

	"alex/internal/agent/ports"
)

// FinalAnswerSummarizer produces a compact, user-facing summary once the core
// ReAct loop has finished.
type FinalAnswerSummarizer struct {
	logger ports.Logger
	clock  ports.Clock
}

// NewFinalAnswerSummarizer constructs a summarizer with optional logger/clock
// overrides for tests.
func NewFinalAnswerSummarizer(logger ports.Logger, clock ports.Clock) *FinalAnswerSummarizer {
	if logger == nil {
		logger = ports.NoopLogger{}
	}
	if clock == nil {
		clock = ports.SystemClock{}
	}
	return &FinalAnswerSummarizer{logger: logger, clock: clock}
}

// Summarize composes a short final response from the completed task state and
// emits the completion via task_complete events for UI consumption.
func (s *FinalAnswerSummarizer) Summarize(
	ctx context.Context,
	env *ports.ExecutionEnvironment,
	result *TaskResult,
	listener EventListener,
) (*TaskResult, error) {
	if env == nil || env.State == nil || result == nil {
		return result, nil
	}

	summaryStart := s.clock.Now()

	llm := env.Services.LLM
	if llm == nil {
		s.emitFinal(ctx, env, result, result.Answer, summaryStart, listener)
		return result, nil
	}

	transcript := s.flattenTranscript(env.State.Messages)
	if transcript == "" {
		s.emitFinal(ctx, env, result, result.Answer, summaryStart, listener)
		return result, nil
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: summarizerSystemPrompt},
			{Role: "user", Content: s.composeUserPrompt(transcript)},
		},
		Temperature: 0.35,
		MaxTokens:   420,
		TopP:        1.0,
		Metadata: map[string]any{
			"intent":     "final_answer_summarization",
			"session_id": env.State.SessionID,
			"task_id":    env.State.TaskID,
		},
	}

	var builder strings.Builder
	var streamedChunks int
	streamCallbacks := ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			if delta.Delta != "" {
				builder.WriteString(delta.Delta)
				streamedChunks++
			}
			if delta.Final {
				return
			}
			if streamedChunks > 1 {
				if partial := strings.TrimSpace(builder.String()); partial != "" {
					s.emitStreamingUpdate(ctx, env, result, partial, summaryStart, listener)
				}
			}
		},
	}

	var resp *ports.CompletionResponse
	var err error
	if streamLLM, ok := llm.(ports.StreamingLLMClient); ok {
		resp, err = streamLLM.StreamComplete(ctx, req, streamCallbacks)
	} else {
		resp, err = llm.Complete(ctx, req)
		if err == nil && resp != nil && strings.TrimSpace(resp.Content) != "" {
			builder.WriteString(resp.Content)
		}
	}
	if err != nil {
		s.logger.Warn("Final summarization failed: %v", err)
		s.emitFinal(ctx, env, result, result.Answer, summaryStart, listener)
		return result, nil
	}

	finalContent := strings.TrimSpace(resp.Content)
	if finalContent == "" {
		finalContent = strings.TrimSpace(builder.String())
	}

	s.emitFinal(ctx, env, result, finalContent, summaryStart, listener)
	return result, nil
}

func (s *FinalAnswerSummarizer) emitStreamingUpdate(
	ctx context.Context,
	env *ports.ExecutionEnvironment,
	result *TaskResult,
	content string,
	start time.Time,
	listener EventListener,
) {
	if listener == nil {
		return
	}
	partial := strings.TrimSpace(content)
	if partial == "" {
		return
	}
	listener.OnEvent(&TaskCompleteEvent{
		BaseEvent:       s.baseEvent(ctx, env.State),
		FinalAnswer:     ensureAttachmentPlaceholders(partial, env.State.Attachments),
		TotalIterations: result.Iterations,
		TotalTokens:     result.TokensUsed,
		StopReason:      result.StopReason,
		Duration:        s.effectiveDuration(result, start),
	})
}

func (s *FinalAnswerSummarizer) emitFinal(
	ctx context.Context,
	env *ports.ExecutionEnvironment,
	result *TaskResult,
	content string,
	start time.Time,
	listener EventListener,
) {
	finalAnswer := strings.TrimSpace(content)
	if finalAnswer == "" {
		finalAnswer = strings.TrimSpace(result.Answer)
	}
	attachments := resolveContentAttachments(finalAnswer, env.State)
	finalAnswer = ensureAttachmentPlaceholders(finalAnswer, attachments)

	result.Answer = finalAnswer
	result.Duration = s.effectiveDuration(result, start)

	if listener == nil {
		return
	}

	listener.OnEvent(&TaskCompleteEvent{
		BaseEvent:       s.baseEvent(ctx, env.State),
		FinalAnswer:     finalAnswer,
		TotalIterations: result.Iterations,
		TotalTokens:     result.TokensUsed,
		StopReason:      result.StopReason,
		Duration:        result.Duration,
		Attachments:     attachments,
	})
}

func (s *FinalAnswerSummarizer) baseEvent(ctx context.Context, state *ports.TaskState) BaseEvent {
	level := ports.GetOutputContext(ctx).Level
	return newBaseEventWithIDs(level, state.SessionID, state.TaskID, state.ParentTaskID, s.clock.Now())
}

func (s *FinalAnswerSummarizer) effectiveDuration(result *TaskResult, start time.Time) time.Duration {
	if result != nil && result.Duration > 0 {
		return result.Duration
	}
	return s.clock.Now().Sub(start)
}

func (s *FinalAnswerSummarizer) flattenTranscript(messages []ports.Message) string {
	var builder strings.Builder
	for _, msg := range messages {
		if msg.Role == "system" || msg.Source == ports.MessageSourceSystemPrompt || msg.Source == ports.MessageSourceDebug {
			continue
		}
		content := strings.TrimSpace(msg.Content)
		if content == "" {
			continue
		}
		role := strings.ToUpper(msg.Role)
		if role == "" {
			role = "ASSISTANT"
		}
		builder.WriteString(role)
		builder.WriteString(": ")
		builder.WriteString(content)
		builder.WriteString("\n")
	}
	return strings.TrimSpace(builder.String())
}

func (s *FinalAnswerSummarizer) composeUserPrompt(transcript string) string {
	return "Conversation (non-system turns only):\n" + transcript + "\n\nWrite the final reply now."
}

const summarizerSystemPrompt = "" +
	"You are producing the final user-facing answer. Write a crisp summary with the essential steps, results, and explicit next actions." +
	" Prefer bullet points when possible, keep it under 160 words, and avoid repetition." +
	" Preserve any attachment placeholders like [file.png] exactly as provided."
