package context

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

type unattendedCtxKey struct{}

// MarkUnattendedContext marks the context to indicate unattended (kernel) execution.
// Agents running in unattended mode must never ask for user confirmation.
func MarkUnattendedContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, unattendedCtxKey{}, true)
}

// IsUnattendedContext checks if the context is marked as unattended execution.
func IsUnattendedContext(ctx context.Context) bool {
	return ctx.Value(unattendedCtxKey{}) != nil
}

// PresetContextKey is the context key used to override presets at runtime.
type PresetContextKey struct{}

// PresetConfig holds preset configuration passed via context.
type PresetConfig struct {
	AgentPreset string
	ToolPreset  string
}
