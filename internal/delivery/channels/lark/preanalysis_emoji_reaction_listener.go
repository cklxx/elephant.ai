package lark

import (
	"context"
	"strings"
	"sync"
	"time"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/utils"
)

// preanalysisEmojiReactionListener reacts to the original inbound Lark message
// with the emoji chosen by the LLM pre-analysis step. The translator passes
// *domain.Event (not *domain.WorkflowEventEnvelope) for preanalysis emoji events.
type preanalysisEmojiReactionListener struct {
	inner     agent.EventListener
	gateway   *Gateway
	ctx       context.Context
	messageID string

	mu      sync.Mutex
	reacted bool
}

func newPreanalysisEmojiReactionListener(
	ctx context.Context,
	inner agent.EventListener,
	gateway *Gateway,
	messageID string,
) *preanalysisEmojiReactionListener {
	return &preanalysisEmojiReactionListener{
		inner:     inner,
		gateway:   gateway,
		ctx:       ctx,
		messageID: strings.TrimSpace(messageID),
	}
}

func (l *preanalysisEmojiReactionListener) OnEvent(event agent.AgentEvent) {
	l.maybeReact(event)
	if l.inner != nil {
		l.inner.OnEvent(event)
	}
}

func (l *preanalysisEmojiReactionListener) maybeReact(event agent.AgentEvent) {
	if l == nil || l.gateway == nil || utils.IsBlank(l.messageID) {
		return
	}

	e, ok := event.(*domain.Event)
	if !ok || e == nil {
		return
	}
	if e.Kind != types.EventDiagnosticPreanalysisEmoji {
		return
	}

	emoji := strings.TrimSpace(e.Data.ReactEmoji)
	if emoji == "" {
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
		l.gateway.addReaction(ctx, l.messageID, emoji)
	}()
}
