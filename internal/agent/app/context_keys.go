package app

import "context"

type subagentCtxKey struct{}

// PresetContextKey is the context key used to override presets at runtime.
type PresetContextKey struct{}

// PresetConfig holds preset configuration passed via context.
type PresetConfig struct {
	AgentPreset string
	ToolPreset  string
}

func isSubagentContext(ctx context.Context) bool {
	return ctx.Value(subagentCtxKey{}) != nil
}
