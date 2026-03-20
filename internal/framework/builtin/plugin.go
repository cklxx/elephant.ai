package builtin

import "alex/internal/core/hook"

// Plugin is the default built-in plugin that provides baseline implementations
// of all hook interfaces. Priority 0 (lowest).
type Plugin struct{}

// New creates a new builtin Plugin.
func New() *Plugin { return &Plugin{} }

// Name returns the plugin name.
func (p *Plugin) Name() string { return "builtin" }

// Priority returns 0 (lowest priority, acts as fallback).
func (p *Plugin) Priority() int { return 0 }

// Verify interface compliance.
var _ hook.Plugin = (*Plugin)(nil)
