package hook

import (
	"context"
	"sort"
	"sync"
)

// HookRuntime manages registered plugins and dispatches hook calls.
type HookRuntime struct {
	mu      sync.RWMutex
	plugins []Plugin
}

// NewHookRuntime creates a new empty HookRuntime.
func NewHookRuntime() *HookRuntime {
	return &HookRuntime{}
}

// Register adds a plugin, maintaining reverse-priority order (highest first).
func (r *HookRuntime) Register(p Plugin) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.plugins = append(r.plugins, p)
	sort.SliceStable(r.plugins, func(i, j int) bool {
		return r.plugins[i].Priority() > r.plugins[j].Priority()
	})
}

// Plugins returns a snapshot of registered plugins.
func (r *HookRuntime) Plugins() []Plugin {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := make([]Plugin, len(r.plugins))
	copy(out, r.plugins)
	return out
}

// CallFirst calls fn for each plugin (highest priority first) that implements
// the target hook interface. Returns the first non-nil, non-error result.
// The caller uses type assertion in fn to check if the plugin implements the hook.
// fn returns (result, handled, error). If handled is true the loop stops.
func CallFirst[T any](ctx context.Context, r *HookRuntime, fn func(Plugin) (T, bool, error)) (T, error) {
	plugins := r.Plugins()
	var zero T
	for _, p := range plugins {
		if ctx.Err() != nil {
			return zero, ctx.Err()
		}
		result, handled, err := fn(p)
		if err != nil {
			return zero, err
		}
		if handled {
			return result, nil
		}
	}
	return zero, nil
}

// CallMany calls fn for all plugins that return a result. Collects all results.
// Errors are isolated -- one plugin's error doesn't stop others.
// fn returns (result, handled, error). Results are collected when handled is true.
func CallMany[T any](ctx context.Context, r *HookRuntime, fn func(Plugin) (T, bool, error)) ([]T, []error) {
	plugins := r.Plugins()
	var results []T
	var errs []error
	for _, p := range plugins {
		if ctx.Err() != nil {
			errs = append(errs, ctx.Err())
			break
		}
		result, handled, err := fn(p) //nolint:errcheck
		if err != nil {
			errs = append(errs, err)
			continue
		}
		if handled {
			results = append(results, result)
		}
	}
	return results, errs
}
