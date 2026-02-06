package context

import (
	"context"

	"alex/internal/app/subscription"
)

type llmSelectionKey struct{}

// WithLLMSelection stores a resolved LLM selection for request-scoped overrides.
func WithLLMSelection(ctx context.Context, selection subscription.ResolvedSelection) context.Context {
	return context.WithValue(ctx, llmSelectionKey{}, selection)
}

// GetLLMSelection returns a resolved LLM selection if one is present in context.
func GetLLMSelection(ctx context.Context) (subscription.ResolvedSelection, bool) {
	if ctx == nil {
		return subscription.ResolvedSelection{}, false
	}
	selection, ok := ctx.Value(llmSelectionKey{}).(subscription.ResolvedSelection)
	return selection, ok
}
