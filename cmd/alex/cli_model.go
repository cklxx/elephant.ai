package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	"alex/internal/app/subscription"
	runtimeconfig "alex/internal/shared/config"
)

func (c *CLI) handleModel(args []string) error {
	return executeModelCommand(args, os.Stdout)
}

func executeModelCommand(args []string, out io.Writer) error {
	subcommand := ""
	if len(args) > 0 {
		subcommand = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch subcommand {
	case "", "list", "ls":
		return listModels(out)
	case "use", "select", "set":
		if len(args) < 2 {
			return fmt.Errorf("usage: alex model use <provider>/<model>")
		}
		return useModel(out, strings.TrimSpace(args[1]))
	case "clear", "reset":
		return clearModel(out)
	case "help", "-h", "--help":
		printModelUsage(out)
		return nil
	default:
		printModelUsage(out)
		return fmt.Errorf("unknown model subcommand: %s", subcommand)
	}
}

func listModels(out io.Writer) error {
	return listModelsFrom(out, runtimeconfig.LoadCLICredentials())
}

func listModelsFrom(out io.Writer, creds runtimeconfig.CLICredentials) error {
	client := &http.Client{Timeout: 20 * time.Second}
	return listModelsFromWith(out, creds, client,
		func(context.Context) (subscription.LlamaServerTarget, bool) {
			return resolveLlamaServerTarget()
		},
	)
}

func listModelsFromWith(
	out io.Writer,
	creds runtimeconfig.CLICredentials,
	client *http.Client,
	llamaResolver func(context.Context) (subscription.LlamaServerTarget, bool),
) error {
	ctx, cancel := context.WithTimeout(cliBaseContext(), 30*time.Second)
	defer cancel()

	if client == nil {
		client = &http.Client{Timeout: 20 * time.Second}
	}
	opts := []subscription.CatalogOption{}
	if llamaResolver != nil {
		opts = append(opts, subscription.WithLlamaServerTargetResolver(llamaResolver))
	}
	svc := subscription.NewCatalogService(
		func() runtimeconfig.CLICredentials { return creds },
		client, 0, opts...,
	)

	catalog := svc.Catalog(ctx)
	if len(catalog.Providers) == 0 {
		if _, err := fmt.Fprintln(out, "未发现可用的订阅模型"); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
		lines := []string{
			"支持的凭证路径:",
			"  Codex:       ~/.codex/auth.json",
			"  Claude:      ~/.claude/credentials.json 或 CLAUDE_CODE_OAUTH_TOKEN 环境变量",
			"  LlamaServer: LLAMA_SERVER_BASE_URL（默认 http://127.0.0.1:8082/v1）",
		}
		for _, line := range lines {
			if _, err := fmt.Fprintln(out, line); err != nil {
				return err
			}
		}
		return nil
	}

	if _, err := fmt.Fprintln(out, "可用的订阅模型:"); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out); err != nil {
		return err
	}

	for _, p := range catalog.Providers {
		if _, err := fmt.Fprintf(out, "  %s (%s)\n", p.Provider, p.Source); err != nil {
			return err
		}
		if p.BaseURL != "" {
			if _, err := fmt.Fprintf(out, "    Base URL: %s\n", p.BaseURL); err != nil {
				return err
			}
		}
		if p.Error != "" {
			if _, err := fmt.Fprintf(out, "    Error: %s\n", p.Error); err != nil {
				return err
			}
		}
		if len(p.Models) > 0 {
			if _, err := fmt.Fprintln(out, "    Models:"); err != nil {
				return err
			}
			for _, m := range p.Models {
				if _, err := fmt.Fprintf(out, "      - %s\n", m); err != nil {
					return err
				}
			}
		}
		if _, err := fmt.Fprintln(out); err != nil {
			return err
		}
	}

	lines := []string{
		"Usage:",
		"  alex model use <provider>/<model>   Select a subscription model",
		"  alex model clear                    Remove subscription selection",
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return err
		}
	}
	return nil
}

func useModel(out io.Writer, spec string) error {
	return useModelWith(out, spec, runtimeconfig.LoadCLICredentials(), runtimeEnvLookup())
}

func useModelWith(out io.Writer, spec string, creds runtimeconfig.CLICredentials, envLookup runtimeconfig.EnvLookup) error {
	parts := strings.SplitN(spec, "/", 2)
	if len(parts) != 2 || strings.TrimSpace(parts[0]) == "" || strings.TrimSpace(parts[1]) == "" {
		return fmt.Errorf("format: <provider>/<model>, e.g. codex/gpt-5.2-codex")
	}
	provider := strings.ToLower(strings.TrimSpace(parts[0]))
	model := strings.TrimSpace(parts[1])

	cred, ok := matchCredential(creds, provider)
	if !ok {
		return fmt.Errorf("no subscription credential found for %q", provider)
	}

	path := subscription.ResolveSelectionStorePath(envLookup, nil)
	store := subscription.NewSelectionStore(path)
	selection := subscription.Selection{
		Mode:     "cli",
		Provider: provider,
		Model:    model,
		Source:   string(cred.Source),
	}
	if err := store.Set(cliBaseContext(), subscription.SelectionScope{Channel: "cli"}, selection); err != nil {
		return fmt.Errorf("save selection: %w", err)
	}
	if _, err := fmt.Fprintf(out, "Switched to %s/%s (%s)\n", provider, model, cred.Source); err != nil {
		return err
	}
	if cred.BaseURL != "" {
		if _, err := fmt.Fprintf(out, "  Base URL: %s\n", cred.BaseURL); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "  Selection: %s\n", path); err != nil {
		return err
	}
	return nil
}

func clearModel(out io.Writer) error {
	return clearModelWith(out, runtimeEnvLookup())
}

func clearModelWith(out io.Writer, envLookup runtimeconfig.EnvLookup) error {
	path := subscription.ResolveSelectionStorePath(envLookup, nil)
	store := subscription.NewSelectionStore(path)
	if err := store.Clear(cliBaseContext(), subscription.SelectionScope{Channel: "cli"}); err != nil {
		return fmt.Errorf("clear selection: %w", err)
	}

	if _, err := fmt.Fprintln(out, "Subscription selection cleared; reverted to config defaults."); err != nil {
		return err
	}
	return nil
}

func matchCredential(creds runtimeconfig.CLICredentials, provider string) (runtimeconfig.CLICredential, bool) {
	switch provider {
	case creds.Codex.Provider:
		if creds.Codex.APIKey != "" {
			return creds.Codex, true
		}
	case creds.Claude.Provider:
		if creds.Claude.APIKey != "" {
			return creds.Claude, true
		}
	case "llama_server":
		return runtimeconfig.CLICredential{
			Provider: "llama_server",
			Source:   "llama_server",
		}, true
	}
	return runtimeconfig.CLICredential{}, false
}

func printModelUsage(out io.Writer) {
	lines := []string{
		"Model command usage:",
		"  alex model                         List available subscription models",
		"  alex model list                    List available subscription models",
		"  alex model use <provider>/<model>  Select a subscription model",
		"  alex model clear                   Remove subscription selection, revert to defaults",
		"",
		"Examples:",
		"  alex model use codex/gpt-5.2-codex",
		"  alex model use anthropic/claude-sonnet-4-20250514",
		"  alex model use llama_server/local-model",
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return
		}
	}
}

func resolveLlamaServerTarget() (subscription.LlamaServerTarget, bool) {
	lookup := runtimeconfig.DefaultEnvLookup

	baseURL := ""
	source := ""
	if value, ok := lookup("LLAMA_SERVER_BASE_URL"); ok {
		baseURL = strings.TrimSpace(value)
		if baseURL != "" {
			source = string(runtimeconfig.SourceEnv)
		}
	}
	if baseURL == "" {
		if host, ok := lookup("LLAMA_SERVER_HOST"); ok {
			host = strings.TrimSpace(host)
			if host != "" {
				if strings.HasPrefix(host, "http://") || strings.HasPrefix(host, "https://") {
					baseURL = host
				} else {
					baseURL = "http://" + host
				}
				source = string(runtimeconfig.SourceEnv)
			}
		}
	}
	if baseURL == "" {
		source = "llama_server"
	}
	return subscription.LlamaServerTarget{BaseURL: baseURL, Source: source}, true
}

