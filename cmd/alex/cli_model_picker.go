package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"

	"golang.org/x/term"
)

func useModelPickerWith(
	out io.Writer,
	in io.Reader,
	creds runtimeconfig.CLICredentials,
	envLookup runtimeconfig.EnvLookup,
) error {
	ctx, cancel := context.WithTimeout(cliBaseContext(), 30*time.Second)
	defer cancel()

	catalog := loadSubscriptionCatalog(ctx, creds)
	providers := selectableCatalogProviders(catalog.Providers)
	if len(providers) == 0 {
		return fmt.Errorf("no selectable subscription providers found; run `alex model list` to inspect credential status")
	}

	provider, err := chooseCatalogProvider(in, out, providers)
	if err != nil {
		return err
	}
	models := orderedCatalogModels(provider)
	if len(models) == 0 {
		return fmt.Errorf("provider %q has no selectable models", provider.Provider)
	}
	model, err := chooseCatalogModel(in, out, provider, models)
	if err != nil {
		return err
	}
	return useModelWith(out, provider.Provider+"/"+model, creds, envLookup)
}

func loadSubscriptionCatalog(ctx context.Context, creds runtimeconfig.CLICredentials) subscription.Catalog {
	client := &http.Client{Timeout: 20 * time.Second}
	service := subscription.NewCatalogService(
		func() runtimeconfig.CLICredentials { return creds },
		client,
		0,
		subscription.WithLlamaServerTargetResolver(func(context.Context) (subscription.LlamaServerTarget, bool) {
			return resolveLlamaServerTarget()
		}),
	)
	return service.Catalog(ctx)
}

func selectableCatalogProviders(providers []subscription.CatalogProvider) []subscription.CatalogProvider {
	if len(providers) == 0 {
		return nil
	}
	out := make([]subscription.CatalogProvider, 0, len(providers))
	for _, provider := range providers {
		if strings.TrimSpace(provider.Provider) == "" {
			continue
		}
		if !provider.Selectable {
			continue
		}
		if len(orderedCatalogModels(provider)) == 0 {
			continue
		}
		out = append(out, provider)
	}
	return out
}

func orderedCatalogModels(provider subscription.CatalogProvider) []string {
	merged := make([]string, 0, len(provider.Models)+len(provider.RecommendedModels)+1)
	seen := make(map[string]struct{}, len(provider.Models)+len(provider.RecommendedModels)+1)
	appendModel := func(raw string) {
		model := strings.TrimSpace(raw)
		if model == "" {
			return
		}
		if _, ok := seen[model]; ok {
			return
		}
		seen[model] = struct{}{}
		merged = append(merged, model)
	}
	for _, rec := range provider.RecommendedModels {
		appendModel(rec.ID)
	}
	for _, model := range provider.Models {
		appendModel(model)
	}
	appendModel(provider.DefaultModel)
	return merged
}

func chooseCatalogProvider(in io.Reader, out io.Writer, providers []subscription.CatalogProvider) (subscription.CatalogProvider, error) {
	if len(providers) == 1 {
		return providers[0], nil
	}
	if !isInteractiveTerminal(in, out) {
		return subscription.CatalogProvider{}, fmt.Errorf(
			"multiple providers available; use `alex model use <provider>/<model>` explicitly or run in an interactive terminal",
		)
	}

	defaultIndex := 1
	for i, provider := range providers {
		switch strings.ToLower(strings.TrimSpace(provider.Provider)) {
		case "codex":
			defaultIndex = i + 1
			goto prompt
		case "anthropic", "claude":
			if defaultIndex == 1 {
				defaultIndex = i + 1
			}
		}
	}
prompt:
	if _, err := fmt.Fprintln(out, "\nSelect provider:"); err != nil {
		return subscription.CatalogProvider{}, err
	}
	for i, provider := range providers {
		label := provider.DisplayName
		if strings.TrimSpace(label) == "" {
			label = provider.Provider
		}
		if _, err := fmt.Fprintf(out, "  %d) %s (%s)\n", i+1, label, provider.Provider); err != nil {
			return subscription.CatalogProvider{}, err
		}
	}
	choice, err := readChoice(in, out, defaultIndex, len(providers))
	if err != nil {
		return subscription.CatalogProvider{}, err
	}
	return providers[choice-1], nil
}

func chooseCatalogModel(in io.Reader, out io.Writer, provider subscription.CatalogProvider, models []string) (string, error) {
	if len(models) == 1 {
		return models[0], nil
	}
	if !isInteractiveTerminal(in, out) {
		return "", fmt.Errorf(
			"provider %q has multiple models; use `alex model use %s/<model>` explicitly or run in an interactive terminal",
			provider.Provider, provider.Provider,
		)
	}

	defaultIndex := 1
	defaultModel := strings.TrimSpace(provider.DefaultModel)
	for i, model := range models {
		if model == defaultModel {
			defaultIndex = i + 1
			break
		}
	}
	if _, err := fmt.Fprintf(out, "\nSelect model for %s:\n", provider.Provider); err != nil {
		return "", err
	}
	for i, model := range models {
		suffix := ""
		if model == defaultModel {
			suffix = " (default)"
		}
		if _, err := fmt.Fprintf(out, "  %d) %s%s\n", i+1, model, suffix); err != nil {
			return "", err
		}
	}
	choice, err := readChoice(in, out, defaultIndex, len(models))
	if err != nil {
		return "", err
	}
	return models[choice-1], nil
}

func readChoice(in io.Reader, out io.Writer, defaultIndex, max int) (int, error) {
	if max <= 0 {
		return 0, fmt.Errorf("no options available")
	}
	if defaultIndex <= 0 || defaultIndex > max {
		defaultIndex = 1
	}
	reader := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprintf(out, "Choice [%d]: ", defaultIndex); err != nil {
			return 0, err
		}
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return 0, err
		}
		input := strings.TrimSpace(line)
		if input == "" {
			return defaultIndex, nil
		}
		value, convErr := strconv.Atoi(input)
		if convErr != nil || value < 1 || value > max {
			if _, wErr := fmt.Fprintf(out, "Please enter a number between 1 and %d.\n", max); wErr != nil {
				return 0, wErr
			}
			if err == io.EOF {
				return 0, fmt.Errorf("invalid selection: %q", input)
			}
			continue
		}
		return value, nil
	}
}

func isInteractiveTerminal(in io.Reader, out io.Writer) bool {
	inFile, inOK := in.(*os.File)
	outFile, outOK := out.(*os.File)
	if !inOK || !outOK {
		return false
	}
	return term.IsTerminal(int(inFile.Fd())) && term.IsTerminal(int(outFile.Fd()))
}
