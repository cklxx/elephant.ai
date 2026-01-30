package context

import "context"

type memoryPolicyKey struct{}

// MemoryPolicy controls per-request memory behavior overrides.
type MemoryPolicy struct {
	Enabled         bool
	AutoRecall      bool
	AutoCapture     bool
	CaptureMessages bool
}

// WithMemoryPolicy attaches a MemoryPolicy to the context.
func WithMemoryPolicy(ctx context.Context, policy MemoryPolicy) context.Context {
	return context.WithValue(ctx, memoryPolicyKey{}, policy)
}

// MemoryPolicyFromContext returns the policy and true if one is set.
func MemoryPolicyFromContext(ctx context.Context) (MemoryPolicy, bool) {
	if ctx == nil {
		return MemoryPolicy{}, false
	}
	value, ok := ctx.Value(memoryPolicyKey{}).(MemoryPolicy)
	return value, ok
}

// ResolveMemoryPolicy returns the effective policy, defaulting to enabled.
func ResolveMemoryPolicy(ctx context.Context) MemoryPolicy {
	if policy, ok := MemoryPolicyFromContext(ctx); ok {
		return policy
	}
	return MemoryPolicy{
		Enabled:     true,
		AutoRecall:  true,
		AutoCapture: true,
	}
}
