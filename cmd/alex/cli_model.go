package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	runtimeconfig "alex/internal/config"
	"alex/internal/subscription"
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
	ctx, cancel := context.WithTimeout(cliBaseContext(), 30*time.Second)
	defer cancel()

	client := &http.Client{Timeout: 20 * time.Second}
	svc := subscription.NewCatalogService(
		func() runtimeconfig.CLICredentials { return creds },
		client, 0,
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
			"  Antigravity: ~/.gemini/oauth_creds.json 或 ~/.antigravity/oauth_creds.json",
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

	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		return fmt.Errorf("load overrides: %w", err)
	}

	overrides.LLMProvider = stringPtr(provider)
	overrides.LLMModel = stringPtr(model)
	if cred.BaseURL != "" {
		overrides.BaseURL = stringPtr(cred.BaseURL)
	}
	if cred.APIKey != "" {
		overrides.APIKey = stringPtr(cred.APIKey)
	}

	if err := saveManagedOverrides(envLookup, overrides); err != nil {
		return fmt.Errorf("save overrides: %w", err)
	}

	path := managedOverridesPath(envLookup)
	if _, err := fmt.Fprintf(out, "Switched to %s/%s (%s)\n", provider, model, cred.Source); err != nil {
		return err
	}
	if cred.BaseURL != "" {
		if _, err := fmt.Fprintf(out, "  Base URL: %s\n", cred.BaseURL); err != nil {
			return err
		}
	}
	if _, err := fmt.Fprintf(out, "  Config:   %s\n", path); err != nil {
		return err
	}
	return nil
}

func clearModel(out io.Writer) error {
	return clearModelWith(out, runtimeEnvLookup())
}

func clearModelWith(out io.Writer, envLookup runtimeconfig.EnvLookup) error {
	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		return fmt.Errorf("load overrides: %w", err)
	}

	overrides.LLMProvider = nil
	overrides.LLMModel = nil
	overrides.BaseURL = nil
	overrides.APIKey = nil

	if err := saveManagedOverrides(envLookup, overrides); err != nil {
		return fmt.Errorf("save overrides: %w", err)
	}

	if _, err := fmt.Fprintln(out, "Subscription selection cleared; reverted to YAML defaults."); err != nil {
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
	case creds.Antigravity.Provider:
		if creds.Antigravity.APIKey != "" {
			return creds.Antigravity, true
		}
	case "ollama":
		return runtimeconfig.CLICredential{
			Provider: "ollama",
			Source:   "ollama",
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
		"  alex model use antigravity/gemini-3-pro-high",
		"  alex model use anthropic/claude-sonnet-4-20250514",
		"  alex model use ollama/llama3:latest",
	}
	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			return
		}
	}
}
