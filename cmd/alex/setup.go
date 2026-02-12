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
	var runtimeMode string
	var larkAppID string
	var larkAppSecret string
	var persistenceMode string
	var persistenceDir string
	var useYAML bool
	fs.StringVar(&provider, "provider", "", "provider id")
	fs.StringVar(&model, "model", "", "model id")
	fs.StringVar(&runtimeMode, "runtime", "", "runtime mode: cli|lark|full-dev")
	fs.StringVar(&larkAppID, "lark-app-id", "", "lark app id")
	fs.StringVar(&larkAppSecret, "lark-app-secret", "", "lark app secret")
	fs.StringVar(&persistenceMode, "persistence-mode", "", "lark persistence mode: file|memory")
	fs.StringVar(&persistenceDir, "persistence-dir", "", "lark persistence dir (used when persistence-mode=file)")
	fs.BoolVar(&useYAML, "use-yaml", false, "complete onboarding and keep YAML runtime defaults")
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
	runtimeMode = strings.TrimSpace(strings.ToLower(runtimeMode))
	larkAppID = strings.TrimSpace(larkAppID)
	larkAppSecret = strings.TrimSpace(larkAppSecret)
	persistenceMode = strings.TrimSpace(strings.ToLower(persistenceMode))
	persistenceDir = strings.TrimSpace(persistenceDir)
	if provider == "" && model != "" {
		return fmt.Errorf("--provider is required when --model is set")
	}
	if provider != "" && model == "" {
		return fmt.Errorf("--model is required when --provider is set")
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
	}

	if provider != "" && model != "" {
		if err := useModelWith(out, provider+"/"+model, creds, envLookup); err != nil {
			return err
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

	if err := useModelPickerWith(out, in, creds, envLookup); err != nil {
		return err
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
