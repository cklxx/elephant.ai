package bootstrap

import (
	"alex/internal/delivery/channels"
)

// ChannelRegistry holds registered channel plugins and their typed
// configuration objects. Each channel registers a PluginFactory (lifecycle)
// and an optional typed config (for cross-cutting consumers like notifiers).
type ChannelRegistry struct {
	plugins []channels.PluginFactory
	configs map[string]any // channel name -> typed config (e.g. LarkGatewayConfig)
}

// NewChannelRegistry creates an empty registry.
func NewChannelRegistry() *ChannelRegistry {
	return &ChannelRegistry{
		configs: make(map[string]any),
	}
}

// Register adds a channel plugin factory to the registry.
func (r *ChannelRegistry) Register(p channels.PluginFactory) {
	r.plugins = append(r.plugins, p)
}

// SetConfig stores a typed config for a channel name.
func (r *ChannelRegistry) SetConfig(name string, cfg any) {
	r.configs[name] = cfg
}

// Config returns the typed config for a channel name, or nil if not set.
func (r *ChannelRegistry) Config(name string) (any, bool) {
	v, ok := r.configs[name]
	return v, ok
}

// Plugins returns the registered plugin factories.
func (r *ChannelRegistry) Plugins() []channels.PluginFactory {
	return r.plugins
}
