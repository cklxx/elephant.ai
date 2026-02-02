package lark

import (
	"context"
	"strings"
	"sync"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
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

	e, ok := event.(*domain.WorkflowToolCompletedEvent)
	if !ok {
		return
	}
	if e == nil || e.Error != nil {
		return
	}
	message, needsInput := planClarifyMessage(e)
	if message == "" {
		return
	}

	p.mu.Lock()
	if _, exists := p.seenCalls[e.CallID]; exists {
		p.mu.Unlock()
		return
	}
	p.seenCalls[e.CallID] = struct{}{}
	p.mu.Unlock()

	if p.gateway == nil || p.gateway.messenger == nil {
		return
	}
	if _, err := p.gateway.dispatchMessage(p.ctx, p.chatID, p.replyTo, "text", textContent(message)); err != nil {
		if p.logger != nil {
			p.logger.Warn("Lark plan/clarify dispatch failed: %v", err)
		}
		return
	}
	if needsInput && p.tracker != nil {
		p.tracker.MarkSent()
	}
}

func planClarifyMessage(e *domain.WorkflowToolCompletedEvent) (string, bool) {
	name := strings.ToLower(strings.TrimSpace(e.ToolName))
	switch name {
	case "plan":
		if msg := stringMeta(e.Metadata, "overall_goal_ui"); msg != "" {
			return msg, false
		}
		return strings.TrimSpace(e.Result), false
	case "clarify":
		needsInput := boolMeta(e.Metadata, "needs_user_input")
		if msg := stringMeta(e.Metadata, "question_to_user"); msg != "" {
			return msg, needsInput
		}
		if msg := stringMeta(e.Metadata, "task_goal_ui"); msg != "" {
			return msg, needsInput
		}
		return strings.TrimSpace(e.Result), needsInput
	default:
		return "", false
	}
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
