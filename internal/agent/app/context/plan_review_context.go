package context

import "context"

type planReviewKey struct{}

// WithPlanReviewEnabled annotates whether plan review is enabled for this request.
func WithPlanReviewEnabled(ctx context.Context, enabled bool) context.Context {
	return context.WithValue(ctx, planReviewKey{}, enabled)
}

// PlanReviewEnabled returns true when plan review is enabled in the context.
func PlanReviewEnabled(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if enabled, ok := ctx.Value(planReviewKey{}).(bool); ok {
		return enabled
	}
	return false
}
