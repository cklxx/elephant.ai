package shared

import "context"

type allowLocalFetchKey struct{}

// WithAllowLocalFetch allows local/loopback URLs for HTTP fetches in tools.
func WithAllowLocalFetch(ctx context.Context) context.Context {
	return context.WithValue(ctx, allowLocalFetchKey{}, true)
}

// AllowLocalFetch reports whether local/loopback URLs are allowed in tool fetches.
func AllowLocalFetch(ctx context.Context) bool {
	if ctx == nil {
		return false
	}
	if allowed, ok := ctx.Value(allowLocalFetchKey{}).(bool); ok {
		return allowed
	}
	return false
}
