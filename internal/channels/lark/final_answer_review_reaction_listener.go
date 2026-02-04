package lark

import (
	"context"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/agent/types"
)

// finalAnswerReviewReactionListener reacts to the original inbound Lark message
// when the ReAct runtime triggers the synthetic `final_answer_review` step.
type finalAnswerReviewReactionListener struct {
	inner     agent.EventListener
	gateway   *Gateway
	ctx       context.Context
	messageID string
	emojiType string

	mu      sync.Mutex
	reacted bool
}

func newFinalAnswerReviewReactionListener(
	ctx context.Context,
	inner agent.EventListener,
	gateway *Gateway,
	messageID string,
	emojiType string,
) *finalAnswerReviewReactionListener {
	emojiType = strings.TrimSpace(emojiType)
	if emojiType == "" {
		emojiType = "GLANCE"
	}
	return &finalAnswerReviewReactionListener{
		inner:     inner,
		gateway:   gateway,
		ctx:       ctx,
		messageID: strings.TrimSpace(messageID),
		emojiType: emojiType,
	}
}

func (l *finalAnswerReviewReactionListener) OnEvent(event agent.AgentEvent) {
	l.maybeReact(event)
	if l.inner != nil {
		l.inner.OnEvent(event)
	}
}

func (l *finalAnswerReviewReactionListener) maybeReact(event agent.AgentEvent) {
	if l == nil || l.gateway == nil || strings.TrimSpace(l.messageID) == "" {
		return
	}

	env, ok := event.(*domain.WorkflowEventEnvelope)
	if !ok || env == nil {
		return
	}
	if strings.TrimSpace(env.Event) != types.EventToolStarted {
		return
	}
	if strings.ToLower(strings.TrimSpace(envelopeToolName(env))) != "final_answer_review" {
		return
	}

	l.mu.Lock()
	if l.reacted {
		l.mu.Unlock()
		return
	}
	l.reacted = true
	l.mu.Unlock()

	go func() {
		ctx := l.ctx
		if ctx == nil {
			ctx = context.Background()
		}
		ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()
		l.gateway.addReaction(ctx, l.messageID, l.emojiType)
	}()
}

