package agent

import "context"

type configOverrideStoreCtxKey struct{}

// WithConfigOverrideStore stores the ConfigOverrideStore in context.
func WithConfigOverrideStore(ctx context.Context, store ConfigOverrideStore) context.Context {
	return context.WithValue(ctx, configOverrideStoreCtxKey{}, store)
}

// ConfigOverrideStoreFromContext retrieves the ConfigOverrideStore from context.
func ConfigOverrideStoreFromContext(ctx context.Context) ConfigOverrideStore {
	if ctx == nil {
		return nil
	}
	s, _ := ctx.Value(configOverrideStoreCtxKey{}).(ConfigOverrideStore)
	return s
}

// ConfigOverride holds staged configuration changes that will be applied
// at the next ReAct iteration boundary. All fields are optional pointers;
// nil means "no change".
type ConfigOverride struct {
	Provider      *string
	Model         *string
	Temperature   *float64
	TopP          *float64
	MaxTokens     *int
	MaxIterations *int
	StopSequences []string // nil = no change; empty slice = clear
}

// ConfigOverrideStore stages config overrides written by the update_config
// tool and consumed by the runtime at the iteration boundary.
type ConfigOverrideStore interface {
	// Stage merges a new override into the pending set. Fields in the new
	// override overwrite previously staged values for the same field.
	Stage(override ConfigOverride) error

	// Pending returns the currently staged override, or nil if nothing is
	// pending.
	Pending() *ConfigOverride

	// Clear discards any staged override.
	Clear()
}
