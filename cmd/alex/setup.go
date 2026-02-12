package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"strings"

	runtimeconfig "alex/internal/shared/config"
)

func (c *CLI) handleSetup(args []string) error {
	return executeSetupCommand(args, os.Stdin, os.Stdout)
}

func executeSetupCommand(args []string, in io.Reader, out io.Writer) error {
	return executeSetupCommandWith(args, in, out, runtimeconfig.LoadCLICredentials(), runtimeEnvLookup())
}

func executeSetupCommandWith(
	args []string,
	in io.Reader,
	out io.Writer,
	creds runtimeconfig.CLICredentials,
	envLookup runtimeconfig.EnvLookup,
) error {
	fs := flag.NewFlagSet("setup", flag.ContinueOnError)
	fs.SetOutput(io.Discard)
	var provider string
	var model string
	var apiKey string
	var runtimeMode string
	var larkAppID string
	var larkAppSecret string
	var persistenceMode string
	var persistenceDir string
	var useYAML bool
	var openLinks bool
	fs.StringVar(&provider, "provider", "", "provider id")
	fs.StringVar(&model, "model", "", "model id")
	fs.StringVar(&apiKey, "api-key", "", "provider API key for single-key setup")
	fs.StringVar(&runtimeMode, "runtime", "", "runtime mode: cli|lark|full-dev")
	fs.StringVar(&larkAppID, "lark-app-id", "", "lark app id")
	fs.StringVar(&larkAppSecret, "lark-app-secret", "", "lark app secret")
	fs.StringVar(&persistenceMode, "persistence-mode", "", "lark persistence mode: file|memory")
	fs.StringVar(&persistenceDir, "persistence-dir", "", "lark persistence dir (used when persistence-mode=file)")
	fs.BoolVar(&useYAML, "use-yaml", false, "complete onboarding and keep YAML runtime defaults")
	fs.BoolVar(&openLinks, "open-links", true, "open provider/lark setup links automatically when available")
	if err := fs.Parse(args); err != nil {
		return fmt.Errorf("parse setup flags: %w", err)
	}

	if useYAML {
		if err := markOnboardingCompleteWithYAML(cliBaseContext(), envLookup); err != nil {
			return fmt.Errorf("save onboarding state: %w", err)
		}
		if _, err := fmt.Fprintln(out, "Setup completed with YAML defaults."); err != nil {
			return err
		}
		return nil
	}

	provider = strings.TrimSpace(provider)
	model = strings.TrimSpace(model)
	apiKey = strings.TrimSpace(apiKey)
	runtimeMode = strings.TrimSpace(strings.ToLower(runtimeMode))
	larkAppID = strings.TrimSpace(larkAppID)
	larkAppSecret = strings.TrimSpace(larkAppSecret)
	persistenceMode = strings.TrimSpace(strings.ToLower(persistenceMode))
	persistenceDir = strings.TrimSpace(persistenceDir)
	allowOpenLinks := openLinks && isInteractiveTerminal(in, out)
	if provider == "" && model != "" {
		return fmt.Errorf("--provider is required when --model is set")
	}
	if provider == "" && apiKey != "" {
		return fmt.Errorf("--provider is required when --api-key is set")
	}

	selection, err := resolveSetupSelection(in, out, setupSelectionInput{
		RuntimeMode:     runtimeMode,
		LarkAppID:       larkAppID,
		LarkAppSecret:   larkAppSecret,
		PersistenceMode: persistenceMode,
		PersistenceDir:  persistenceDir,
	})
	if err != nil {
		return err
	}

	if selection.LarkEnabled {
		configPath, err := applyLarkSetupConfig(envLookup, selection)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "Lark configuration saved to %s\n", configPath); err != nil {
			return err
		}
		printLarkBotSetupHelper(out, allowOpenLinks)
	}

	providerSelection, err := resolveSetupProviderSelection(in, out, creds, setupProviderInput{
		Provider:  provider,
		Model:     model,
		APIKey:    apiKey,
		OpenLinks: allowOpenLinks,
	})
	if err != nil {
		return err
	}

	if providerSelection.UseSubscription {
		if err := useModelWith(out, providerSelection.Provider+"/"+providerSelection.Model, creds, envLookup); err != nil {
			return err
		}
	} else {
		if err := applySetupProviderOverrides(envLookup, providerSelection); err != nil {
			return err
		}
		if err := markOnboardingCompleteFromSelection(cliBaseContext(), envLookup, subscriptionSelection(providerSelection)); err != nil {
			return fmt.Errorf("save onboarding state: %w", err)
		}
		if _, err := fmt.Fprintf(out, "Configured %s/%s via managed overrides.\n", providerSelection.Provider, providerSelection.Model); err != nil {
			return err
		}
		if providerSelection.BaseURL != "" {
			if _, err := fmt.Fprintf(out, "  Base URL: %s\n", providerSelection.BaseURL); err != nil {
				return err
			}
		}
		if _, err := fmt.Fprintf(out, "  Config file: %s\n", managedOverridesPath(envLookup)); err != nil {
			return err
		}
	}
	if err := markOnboardingSetupSelections(cliBaseContext(), envLookup, onboardingSetupSelection{
		RuntimeMode:     selection.RuntimeMode,
		PersistenceMode: selection.PersistenceMode,
		LarkConfigured:  selection.LarkEnabled,
	}); err != nil {
		return fmt.Errorf("save onboarding state: %w", err)
	}
	if _, err := fmt.Fprintln(out, "Setup completed."); err != nil {
		return err
	}
	return nil
}
