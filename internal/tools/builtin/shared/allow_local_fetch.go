package builtin

import "context"

type allowLocalFetchKey struct{}

// WithAllowLocalFetch allows local/loopback URLs for HTTP fetches in tools.
func WithAllowLocalFetch(ctx context.Context) context.Context {
	return context.WithValue(ctx, allowLocalFetchKey{}, true)
}

func allowLocalFetch(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if allowed, ok := ctx.Value(allowLocalFetchKey{}).(bool); ok {
		return allowed
	}
	return false
}
