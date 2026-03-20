package framework

import (
	"context"
	"fmt"

	"alex/internal/core/hook"
)

// Lifecycle is an optional interface for plugins that need start/stop.
type Lifecycle interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
}

// PluginManager manages plugin lifecycle.
type PluginManager struct {
	plugins []hook.Plugin
}

// Add registers a plugin.
func (pm *PluginManager) Add(p hook.Plugin) {
	pm.plugins = append(pm.plugins, p)
}

// StartAll starts all plugins that implement Lifecycle.
func (pm *PluginManager) StartAll(ctx context.Context) error {
	for _, p := range pm.plugins {
		if lc, ok := p.(Lifecycle); ok {
			if err := lc.Start(ctx); err != nil {
				return fmt.Errorf("starting plugin %q: %w", p.Name(), err)
			}
		}
	}
	return nil
}

// StopAll stops all plugins that implement Lifecycle (in reverse order).
func (pm *PluginManager) StopAll(ctx context.Context) error {
	var firstErr error
	for i := len(pm.plugins) - 1; i >= 0; i-- {
		p := pm.plugins[i]
		if lc, ok := p.(Lifecycle); ok {
			if err := lc.Stop(ctx); err != nil && firstErr == nil {
				firstErr = fmt.Errorf("stopping plugin %q: %w", p.Name(), err)
			}
		}
	}
	return firstErr
}
