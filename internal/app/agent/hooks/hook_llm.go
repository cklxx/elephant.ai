package hooks

import (
	"context"
	"strings"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/app/agent/llmclient"
	"alex/internal/domain/agent/ports"
	portsllm "alex/internal/domain/agent/ports/llm"
	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

// resolveProfile selects the LLM profile for auxiliary hook calls.
// It returns (primary, fallback, canFallback, ok).
func resolveProfile(
	ctx context.Context,
	defaultProfile runtimeconfig.LLMProfile,
) (runtimeconfig.LLMProfile, runtimeconfig.LLMProfile, bool, bool) {
	defaultOK := !utils.IsBlank(defaultProfile.Provider) && !utils.IsBlank(defaultProfile.Model)

	// Prefer per-request LLM selection from context (e.g. model override).
	if selection, ok := appcontext.GetLLMSelection(ctx); ok {
		provider := strings.TrimSpace(selection.Provider)
		model := strings.TrimSpace(selection.Model)
		if provider != "" && model != "" {
			profile := runtimeconfig.LLMProfile{
				Provider: provider,
				Model:    model,
				APIKey:   strings.TrimSpace(selection.APIKey),
				BaseURL:  strings.TrimSpace(selection.BaseURL),
				Headers:  llmclient.CloneHeaders(selection.Headers),
			}
			if selection.Pinned && defaultOK && !sameProviderModel(profile, defaultProfile) {
				return profile, defaultProfile, true, true
			}
			return profile, runtimeconfig.LLMProfile{}, false, true
		}
	}

	// Fall back to the shared runtime profile.
	if !defaultOK {
		return runtimeconfig.LLMProfile{}, runtimeconfig.LLMProfile{}, false, false
	}
	return defaultProfile, runtimeconfig.LLMProfile{}, false, true
}

// completeWithProfile creates an isolated LLM client from the profile and
// executes a completion request with the given timeout.
func completeWithProfile(
	ctx context.Context,
	factory portsllm.LLMClientFactory,
	profile runtimeconfig.LLMProfile,
	req ports.CompletionRequest,
	timeout func(context.Context) (context.Context, context.CancelFunc),
) (*ports.CompletionResponse, error) {
	client, _, err := llmclient.GetIsolatedClientFromProfile(factory, profile, nil, false)
	if err != nil {
		return nil, err
	}
	ctxTimeout, cancel := timeout(ctx)
	defer cancel()
	return client.Complete(ctxTimeout, req)
}
