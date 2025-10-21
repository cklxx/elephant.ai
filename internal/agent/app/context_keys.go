package app

import "context"

type subagentCtxKey struct{}

// MarkSubagentContext marks the context to indicate execution within a subagent
// This triggers tool registry filtering to prevent nested subagent calls
func MarkSubagentContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, subagentCtxKey{}, true)
}

// IsSubagentContext checks if the context is marked as subagent execution
func IsSubagentContext(ctx context.Context) bool {
	return ctx.Value(subagentCtxKey{}) != nil
}

// isSubagentContext is the internal version for app layer
func isSubagentContext(ctx context.Context) bool {
	return IsSubagentContext(ctx)
}

// PresetContextKey is the context key used to override presets at runtime.
type PresetContextKey struct{}

// PresetConfig holds preset configuration passed via context.
type PresetConfig struct {
	AgentPreset string
	ToolPreset  string
}
