package lark

import (
	"context"
	"strings"
	"sync"

	"alex/internal/domain/agent"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

// planClarifyListener sends plan/clarify tool outputs to Lark as text messages.
type planClarifyListener struct {
	inner   agent.EventListener
	gateway *Gateway
	ctx     context.Context
	chatID  string
	replyTo string
	logger  logging.Logger

	mu        sync.Mutex
	seenCalls map[string]struct{}

	tracker *awaitQuestionTracker
}

func newPlanClarifyListener(
	ctx context.Context,
	inner agent.EventListener,
	gateway *Gateway,
	chatID string,
	replyTo string,
	tracker *awaitQuestionTracker,
) *planClarifyListener {
	return &planClarifyListener{
		inner:     inner,
		gateway:   gateway,
		ctx:       ctx,
		chatID:    chatID,
		replyTo:   replyTo,
		logger:    logging.OrNop(nil),
		seenCalls: make(map[string]struct{}),
		tracker:   tracker,
	}
}

// OnEvent forwards the event to the inner listener and emits plan/clarify messages.
func (p *planClarifyListener) OnEvent(event agent.AgentEvent) {
	if p.inner != nil {
		p.inner.OnEvent(event)
	}

	payload, callID := p.extractMessage(event)
	if payload.message == "" || callID == "" {
		return
	}

	p.mu.Lock()
	if _, exists := p.seenCalls[callID]; exists {
		p.mu.Unlock()
		return
	}
	p.seenCalls[callID] = struct{}{}
	p.mu.Unlock()

	if p.gateway == nil || p.gateway.messenger == nil {
		return
	}

	content := textContent(payload.message)
	if payload.needsInput && len(payload.options) > 0 {
		content = textContent(formatNumberedOptions(payload.message, payload.options))
		// Store pending options so numeric replies can be resolved.
		slot := p.gateway.getOrCreateSlot(p.chatID)
		slot.mu.Lock()
		slot.pendingOptions = payload.options
		slot.mu.Unlock()
	}

	if _, err := p.gateway.dispatchMessage(p.ctx, p.chatID, p.replyTo, "text", content); err != nil {
		if p.logger != nil {
			p.logger.Warn("Lark plan/clarify dispatch failed: %v", err)
		}
		return
	}
	if payload.needsInput && p.tracker != nil {
		p.tracker.MarkSent()
	}
}

type planClarifyPayload struct {
	message    string
	needsInput bool
	options    []string
}

func (p *planClarifyListener) extractMessage(event agent.AgentEvent) (planClarifyPayload, string) {
	switch e := event.(type) {
	case *domain.Event:
		if e == nil || e.Kind != types.EventToolCompleted || e.Data.Error != nil {
			return planClarifyPayload{}, ""
		}
		payload := planClarifyMessageFromEvent(e)
		return payload, e.Data.CallID
	case *domain.WorkflowEventEnvelope:
		payload := planClarifyMessageFromEnvelope(e)
		return payload, envelopeCallID(e)
	default:
		return planClarifyPayload{}, ""
	}
}

func planClarifyMessageFromEvent(e *domain.Event) planClarifyPayload {
	name := strings.ToLower(strings.TrimSpace(e.Data.ToolName))
	switch name {
	case "plan":
		if msg := stringMeta(e.Data.Metadata, "overall_goal_ui"); msg != "" {
			return planClarifyPayload{message: msg}
		}
		return planClarifyPayload{message: strings.TrimSpace(e.Data.Result)}
	case "clarify":
		needsInput := boolMeta(e.Data.Metadata, "needs_user_input")
		if needsInput {
			if prompt, ok := awaitPromptFromResult(e.Data.Result, e.Data.Metadata); ok {
				return planClarifyPayload{
					message:    prompt.Question,
					needsInput: true,
					options:    prompt.Options,
				}
			}
		}
		if msg := stringMeta(e.Data.Metadata, "task_goal_ui"); msg != "" {
			return planClarifyPayload{message: msg, needsInput: needsInput}
		}
		return planClarifyPayload{message: strings.TrimSpace(e.Data.Result), needsInput: needsInput}
	default:
		return planClarifyPayload{}
	}
}

func planClarifyMessageFromEnvelope(e *domain.WorkflowEventEnvelope) planClarifyPayload {
	if e == nil || strings.TrimSpace(e.Event) != types.EventToolCompleted || e.Payload == nil {
		return planClarifyPayload{}
	}
	if envelopeHasError(e) {
		return planClarifyPayload{}
	}
	name := strings.ToLower(strings.TrimSpace(envelopeToolName(e)))
	metadata, _ := e.Payload["metadata"].(map[string]any)
	result, _ := e.Payload["result"].(string)
	switch name {
	case "plan":
		if msg := stringMeta(metadata, "overall_goal_ui"); msg != "" {
			return planClarifyPayload{message: msg}
		}
		return planClarifyPayload{message: strings.TrimSpace(result)}
	case "clarify":
		needsInput := boolMeta(metadata, "needs_user_input")
		if needsInput {
			if prompt, ok := awaitPromptFromResult(result, metadata); ok {
				return planClarifyPayload{
					message:    prompt.Question,
					needsInput: true,
					options:    prompt.Options,
				}
			}
		}
		if msg := stringMeta(metadata, "task_goal_ui"); msg != "" {
			return planClarifyPayload{message: msg, needsInput: needsInput}
		}
		return planClarifyPayload{message: strings.TrimSpace(result), needsInput: needsInput}
	default:
		return planClarifyPayload{}
	}
}

func awaitPromptFromResult(result string, metadata map[string]any) (agent.AwaitUserInputPrompt, bool) {
	messages := []ports.Message{{
		ToolResults: []ports.ToolResult{{
			Content:  result,
			Metadata: metadata,
		}},
	}}
	return agent.ExtractAwaitUserInputPrompt(messages)
}

func stringMeta(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	raw, ok := metadata[key]
	if !ok {
		return ""
	}
	value, ok := raw.(string)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func boolMeta(metadata map[string]any, key string) bool {
	if metadata == nil {
		return false
	}
	raw, ok := metadata[key]
	if !ok {
		return false
	}
	switch value := raw.(type) {
	case bool:
		return value
	case string:
		return strings.EqualFold(strings.TrimSpace(value), "true")
	default:
		return false
	}
}
