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
	var useYAML bool
	fs.StringVar(&provider, "provider", "", "provider id")
	fs.StringVar(&model, "model", "", "model id")
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
	if provider == "" && model != "" {
		return fmt.Errorf("--provider is required when --model is set")
	}
	if provider != "" && model == "" {
		return fmt.Errorf("--model is required when --provider is set")
	}

	if provider != "" && model != "" {
		if err := useModelWith(out, provider+"/"+model, creds, envLookup); err != nil {
			return err
		}
		if _, err := fmt.Fprintln(out, "Setup completed."); err != nil {
			return err
		}
		return nil
	}

	if err := useModelPickerWith(out, in, creds, envLookup); err != nil {
		return err
	}
	if _, err := fmt.Fprintln(out, "Setup completed."); err != nil {
		return err
	}
	return nil
}
