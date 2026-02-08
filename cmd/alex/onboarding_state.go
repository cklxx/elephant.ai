package main

import (
	"context"
	"strings"
	"time"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
)

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
