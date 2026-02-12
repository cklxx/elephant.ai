package main

import (
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"time"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
)

type setupProviderInput struct {
	Provider  string
	Model     string
	APIKey    string
	OpenLinks bool
}

type setupProviderSelection struct {
	Provider        string
	Model           string
	APIKey          string
	BaseURL         string
	Source          string
	UseSubscription bool
}

type setupLink struct {
	Label string
	URL   string
}

func resolveSetupProviderSelection(
	in io.Reader,
	out io.Writer,
	creds runtimeconfig.CLICredentials,
	input setupProviderInput,
) (setupProviderSelection, error) {
	ctx, cancel := context.WithTimeout(cliBaseContext(), 30*time.Second)
	defer cancel()

	catalog := loadSubscriptionCatalog(ctx, creds)
	providers := selectableCatalogProviders(catalog.Providers)
	if len(providers) == 0 {
		return setupProviderSelection{}, fmt.Errorf("no selectable providers found; run `alex model list` to inspect setup status")
	}

	selectedProvider, err := chooseSetupProvider(in, out, providers, input.Provider)
	if err != nil {
		return setupProviderSelection{}, err
	}

	models := orderedCatalogModels(selectedProvider)
	if len(models) == 0 {
		return setupProviderSelection{}, fmt.Errorf("provider %q has no selectable models", selectedProvider.Provider)
	}
	selectedModel, err := chooseSetupModel(in, out, selectedProvider, input.Model)
	if err != nil {
		return setupProviderSelection{}, err
	}

	providerID := normalizeSetupProviderID(selectedProvider.Provider)
	cred, credOK := matchCredential(creds, providerID)
	apiKey := strings.TrimSpace(input.APIKey)
	if apiKey == "" && credOK && strings.TrimSpace(cred.APIKey) != "" {
		return setupProviderSelection{
			Provider:        providerID,
			Model:           selectedModel,
			BaseURL:         strings.TrimSpace(selectedProvider.BaseURL),
			Source:          string(cred.Source),
			UseSubscription: true,
		}, nil
	}

	if runtimeconfig.ProviderRequiresAPIKey(providerID) && apiKey == "" {
		promptedKey, err := promptForProviderAPIKey(in, out, selectedProvider, input.OpenLinks)
		if err != nil {
			return setupProviderSelection{}, err
		}
		apiKey = promptedKey
	}

	baseURL := strings.TrimSpace(selectedProvider.BaseURL)
	if baseURL == "" {
		if preset, ok := subscription.LookupProviderPreset(providerID); ok {
			baseURL = strings.TrimSpace(preset.DefaultBaseURL)
		}
	}

	return setupProviderSelection{
		Provider: providerID,
		Model:    selectedModel,
		APIKey:   apiKey,
		BaseURL:  baseURL,
		Source:   "manual",
	}, nil
}

func chooseSetupProvider(
	in io.Reader,
	out io.Writer,
	providers []subscription.CatalogProvider,
	rawProvider string,
) (subscription.CatalogProvider, error) {
	provider := normalizeSetupProviderID(rawProvider)
	if provider == "" {
		if !isInteractiveTerminal(in, out) {
			return subscription.CatalogProvider{}, fmt.Errorf("--provider is required in non-interactive mode")
		}
		return chooseCatalogProvider(in, out, providers)
	}

	for _, item := range providers {
		if normalizeSetupProviderID(item.Provider) == provider {
			return item, nil
		}
	}
	return subscription.CatalogProvider{}, fmt.Errorf("unknown provider %q", provider)
}

func chooseSetupModel(
	in io.Reader,
	out io.Writer,
	provider subscription.CatalogProvider,
	rawModel string,
) (string, error) {
	model := strings.TrimSpace(rawModel)
	if model != "" {
		return model, nil
	}
	models := orderedCatalogModels(provider)
	if len(models) == 0 {
		return "", fmt.Errorf("provider %q has no selectable models", provider.Provider)
	}
	if !isInteractiveTerminal(in, out) {
		return pickSetupDefaultModel(provider, models), nil
	}
	return chooseCatalogModel(in, out, provider, models)
}

func pickSetupDefaultModel(provider subscription.CatalogProvider, models []string) string {
	defaultModel := strings.TrimSpace(provider.DefaultModel)
	if defaultModel != "" {
		for _, item := range models {
			if item == defaultModel {
				return defaultModel
			}
		}
	}
	return models[0]
}

func normalizeSetupProviderID(provider string) string {
	key := strings.ToLower(strings.TrimSpace(provider))
	switch key {
	case "claude":
		return "anthropic"
	default:
		return key
	}
}

func promptForProviderAPIKey(
	in io.Reader,
	out io.Writer,
	provider subscription.CatalogProvider,
	openLinks bool,
) (string, error) {
	if !isInteractiveTerminal(in, out) {
		return "", fmt.Errorf("--api-key is required for provider %q in non-interactive mode", provider.Provider)
	}
	providerLabel := strings.TrimSpace(provider.DisplayName)
	if providerLabel == "" {
		providerLabel = provider.Provider
	}
	keyURL := strings.TrimSpace(provider.KeyCreateURL)
	if keyURL != "" {
		if _, err := fmt.Fprintf(out, "Create %s API key: %s\n", providerLabel, keyURL); err != nil {
			return "", err
		}
		if openLinks {
			if err := openExternalURL(keyURL); err != nil {
				if _, writeErr := fmt.Fprintf(out, "  Unable to open browser automatically: %v\n", err); writeErr != nil {
					return "", writeErr
				}
			}
		}
	}
	value, err := promptLine(in, out, providerLabel+" API key")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(value), nil
}

func applySetupProviderOverrides(envLookup runtimeconfig.EnvLookup, selection setupProviderSelection) error {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		return fmt.Errorf("load managed overrides: %w", err)
	}

	overrides.LLMProvider = stringPtr(selection.Provider)
	overrides.LLMModel = stringPtr(selection.Model)
	overrides.APIKey = stringPtr(selection.APIKey)
	if strings.TrimSpace(selection.BaseURL) != "" {
		overrides.BaseURL = stringPtr(strings.TrimSpace(selection.BaseURL))
	}

	if err := saveManagedOverrides(envLookup, overrides); err != nil {
		return fmt.Errorf("save managed overrides: %w", err)
	}

	store := subscription.NewSelectionStore(subscription.ResolveSelectionStorePath(envLookup, nil))
	if err := store.Clear(cliBaseContext(), subscription.SelectionScope{Channel: "cli"}); err != nil {
		return fmt.Errorf("clear cli selection store: %w", err)
	}
	return nil
}

func subscriptionSelection(selection setupProviderSelection) subscription.Selection {
	source := strings.TrimSpace(selection.Source)
	if source == "" {
		source = "manual"
	}
	return subscription.Selection{
		Mode:     "cli",
		Provider: strings.TrimSpace(selection.Provider),
		Model:    strings.TrimSpace(selection.Model),
		Source:   source,
	}
}

func printLarkBotSetupHelper(out io.Writer, openLinks bool) {
	links := []setupLink{
		{
			Label: "Feishu Open Platform",
			URL:   "https://open.feishu.cn/app",
		},
		{
			Label: "Feishu Bot Permission Guide",
			URL:   "https://open.feishu.cn/document/develop-robots/add-bot-to-external-group",
		},
	}

	_, _ = fmt.Fprintln(out, "\nLark bot quick links:")
	for _, link := range links {
		_, _ = fmt.Fprintf(out, "  - %s: %s\n", link.Label, link.URL)
		if !openLinks {
			continue
		}
		if err := openExternalURL(link.URL); err != nil {
			_, _ = fmt.Fprintf(out, "    (auto-open skipped: %v)\n", err)
		}
	}

	_, _ = fmt.Fprintln(out, "\nLark callback endpoints to configure:")
	_, _ = fmt.Fprintln(out, "  - Message/Event callback: /api/lark/callback")
	_, _ = fmt.Fprintln(out, "  - Card callback: /api/lark/card/callback")
	_, _ = fmt.Fprintln(out, "\nCopyable bot config template:")
	_, _ = fmt.Fprintln(out, "channels:")
	_, _ = fmt.Fprintln(out, "  lark:")
	_, _ = fmt.Fprintln(out, "    enabled: true")
	_, _ = fmt.Fprintln(out, "    app_id: \"cli_xxx\"")
	_, _ = fmt.Fprintln(out, "    app_secret: \"xxx\"")
	_, _ = fmt.Fprintln(out, "    persistence:")
	_, _ = fmt.Fprintln(out, "      mode: file")
	_, _ = fmt.Fprintln(out, "      dir: ~/.alex/lark")
}

func openExternalURL(rawURL string) error {
	url := strings.TrimSpace(rawURL)
	if url == "" {
		return fmt.Errorf("empty URL")
	}

	if openCmd, err := exec.LookPath("open"); err == nil && strings.TrimSpace(openCmd) != "" {
		return exec.Command(openCmd, url).Start()
	}
	if openCmd, err := exec.LookPath("xdg-open"); err == nil && strings.TrimSpace(openCmd) != "" {
		return exec.Command(openCmd, url).Start()
	}
	return fmt.Errorf("no opener command found (`open`/`xdg-open`)")
}
