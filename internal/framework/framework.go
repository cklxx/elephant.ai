package framework

import (
	"context"

	"alex/internal/core/envelope"
	"alex/internal/core/hook"
	"alex/internal/core/tape"
	"alex/internal/core/channel"
)

// Framework is the main entry point for processing inbound requests.
type Framework struct {
	hooks    *hook.HookRuntime
	tapes    *tape.TapeManager
	channels *channel.Manager
	plugins  *PluginManager
}

// Config holds Framework configuration.
type Config struct {
	TapeManager    *tape.TapeManager
	ChannelManager *channel.Manager
}

// New creates a Framework with the given config.
func New(cfg Config) *Framework {
	return &Framework{
		hooks:    hook.NewHookRuntime(),
		tapes:    cfg.TapeManager,
		channels: cfg.ChannelManager,
		plugins:  &PluginManager{},
	}
}

// ProcessInbound processes an inbound envelope through the 7-step turn lifecycle.
func (f *Framework) ProcessInbound(ctx context.Context, env envelope.Envelope) (*hook.TurnResult, error) {
	executor := &turnExecutor{
		hooks:    f.hooks,
		tapes:    f.tapes,
		channels: f.channels,
	}
	return executor.execute(ctx, env)
}

// Hooks returns the hook runtime for plugin registration.
func (f *Framework) Hooks() *hook.HookRuntime {
	return f.hooks
}

// RegisterPlugin adds a plugin to the framework.
func (f *Framework) RegisterPlugin(p hook.Plugin) {
	f.hooks.Register(p)
	f.plugins.Add(p)
}
