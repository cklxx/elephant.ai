package main

import (
	"context"
	"time"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

type onboardingSetupSelection struct {
	RuntimeMode     string
	PersistenceMode string
	LarkConfigured  bool
}

func resolveOnboardingStore(envLookup runtimeconfig.EnvLookup) *subscription.OnboardingStateStore {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	return subscription.NewOnboardingStateStore(
		subscription.ResolveOnboardingStatePath(envLookup, nil),
	)
}

func markOnboardingCompleteFromSelection(ctx context.Context, envLookup runtimeconfig.EnvLookup, selection subscription.Selection) error {
	// NormalizeOnboardingState (called by Set) handles TrimSpace/ToLower.
	return resolveOnboardingStore(envLookup).Set(ctx, subscription.OnboardingState{
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		SelectedProvider: selection.Provider,
		SelectedModel:    selection.Model,
		UsedSource:       selection.Source,
	})
}

func markOnboardingCompleteWithYAML(ctx context.Context, envLookup runtimeconfig.EnvLookup) error {
	return resolveOnboardingStore(envLookup).Set(ctx, subscription.OnboardingState{
		CompletedAt:           time.Now().UTC().Format(time.RFC3339),
		UsedSource:            "yaml",
		AdvancedOverridesUsed: true,
	})
}

func markOnboardingSetupSelections(ctx context.Context, envLookup runtimeconfig.EnvLookup, selection onboardingSetupSelection) error {
	store := resolveOnboardingStore(envLookup)
	state, _, err := store.Get(ctx)
	if err != nil {
		return err
	}

	state.SelectedRuntimeMode = selection.RuntimeMode
	if selection.PersistenceMode != "" {
		state.PersistenceMode = selection.PersistenceMode
	}
	state.LarkConfigured = selection.LarkConfigured
	if utils.IsBlank(state.CompletedAt) {
		state.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return store.Set(ctx, state)
}
