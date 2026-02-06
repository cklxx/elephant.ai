package react

import (
	"context"

	"alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
)

type backgroundEventSink struct {
	emitEvent      func(agent.AgentEvent)
	baseEvent      func(context.Context) domain.BaseEvent
	parentListener agent.EventListener
}

type backgroundEventSinkKey struct{}

func withBackgroundEventSink(ctx context.Context, sink backgroundEventSink) context.Context {
	if ctx == nil {
		return ctx
	}
	return context.WithValue(ctx, backgroundEventSinkKey{}, sink)
}

func getBackgroundEventSink(ctx context.Context) (backgroundEventSink, bool) {
	if ctx == nil {
		return backgroundEventSink{}, false
	}
	sink, ok := ctx.Value(backgroundEventSinkKey{}).(backgroundEventSink)
	return sink, ok
}
