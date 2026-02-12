package main

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
)

type onboardingSetupSelection struct {
	RuntimeMode     string
	PersistenceMode string
	LarkConfigured  bool
}

func markOnboardingCompleteFromSelection(ctx context.Context, envLookup runtimeconfig.EnvLookup, selection subscription.Selection) error {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	store := subscription.NewOnboardingStateStore(
		subscription.ResolveOnboardingStatePath(envLookup, nil),
	)
	state := subscription.OnboardingState{
		CompletedAt:      time.Now().UTC().Format(time.RFC3339),
		SelectedProvider: strings.ToLower(strings.TrimSpace(selection.Provider)),
		SelectedModel:    strings.TrimSpace(selection.Model),
		UsedSource:       strings.TrimSpace(selection.Source),
	}
	return store.Set(ctx, state)
}

func markOnboardingCompleteWithYAML(ctx context.Context, envLookup runtimeconfig.EnvLookup) error {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	store := subscription.NewOnboardingStateStore(
		subscription.ResolveOnboardingStatePath(envLookup, nil),
	)
	state := subscription.OnboardingState{
		CompletedAt:           time.Now().UTC().Format(time.RFC3339),
		UsedSource:            "yaml",
		AdvancedOverridesUsed: true,
	}
	return store.Set(ctx, state)
}

func markOnboardingSetupSelections(ctx context.Context, envLookup runtimeconfig.EnvLookup, selection onboardingSetupSelection) error {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	store := subscription.NewOnboardingStateStore(
		subscription.ResolveOnboardingStatePath(envLookup, nil),
	)
	state, ok, err := store.Get(ctx)
	if err != nil {
		return err
	}
	if !ok {
		state = subscription.OnboardingState{}
	}

	runtimeMode := strings.ToLower(strings.TrimSpace(selection.RuntimeMode))
	persistenceMode := strings.ToLower(strings.TrimSpace(selection.PersistenceMode))
	state.SelectedRuntimeMode = runtimeMode
	if persistenceMode != "" {
		state.PersistenceMode = persistenceMode
	}
	state.LarkConfigured = selection.LarkConfigured
	if strings.TrimSpace(state.CompletedAt) == "" {
		state.CompletedAt = time.Now().UTC().Format(time.RFC3339)
	}
	return store.Set(ctx, state)
}
