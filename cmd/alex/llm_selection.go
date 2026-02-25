package main

import (
	"context"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/logging"
)

func applyPinnedCLILLMSelection(ctx context.Context, envLookup runtimeconfig.EnvLookup, logger logging.Logger) context.Context {
	if ctx == nil {
		return nil
	}
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	logger = logging.OrNop(logger)

	storePath := subscription.ResolveSelectionStorePath(envLookup, nil)
	store := subscription.NewSelectionStore(storePath)
	selection, ok, err := store.Get(ctx, subscription.SelectionScope{Channel: "cli"})
	if err != nil {
		logger.Warn("Failed to load CLI LLM selection: %v", err)
		return ctx
	}
	if !ok {
		return ctx
	}

	resolver := subscription.NewSelectionResolver(func() runtimeconfig.CLICredentials {
		return runtimeconfig.LoadCLICredentials(runtimeconfig.WithEnv(envLookup))
	})
	resolved, ok := resolver.Resolve(selection)
	if !ok {
		logger.Warn("Ignoring invalid CLI LLM selection: provider=%q model=%q", selection.Provider, selection.Model)
		return ctx
	}
	return appcontext.WithLLMSelection(ctx, resolved)
}
