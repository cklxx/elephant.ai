package main

import (
	"fmt"
	"io"
	"os"
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"
	configadmin "alex/internal/shared/config/admin"
)

func executeConfigCommand(args []string, out io.Writer) error {
	envLookup := runtimeEnvLookup()
	overridesPath := managedOverridesPath(envLookup)
	subcommand := ""
	if len(args) > 0 {
		subcommand = strings.ToLower(strings.TrimSpace(args[0]))
	}

	switch subcommand {
	case "", "show", "list":
		return printConfigSummary(out, overridesPath)
	case "set":
		key, value, err := parseSetArgs(args[1:])
		if err != nil {
			return err
		}
		if err := mutateOverrides(envLookup, key, value, setOverrideField); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "已更新 %s (写入 %s#overrides)\n\n", normalizeOverrideKey(key), overridesPath); err != nil {
			return fmt.Errorf("write update message: %w", err)
		}
		return printConfigSummary(out, overridesPath)
	case "clear", "unset", "delete", "rm":
		if len(args) < 2 {
			return fmt.Errorf("usage: alex config clear <field>")
		}
		key := strings.TrimSpace(args[1])
		if key == "" {
			return fmt.Errorf("usage: alex config clear <field>")
		}
		if err := mutateOverrides(envLookup, key, "", clearOverrideField); err != nil {
			return err
		}
		if _, err := fmt.Fprintf(out, "已清除 %s (写入 %s#overrides)\n\n", normalizeOverrideKey(key), overridesPath); err != nil {
			return fmt.Errorf("write clear message: %w", err)
		}
		return printConfigSummary(out, overridesPath)
	case "path", "file":
		if _, err := fmt.Fprintln(out, overridesPath); err != nil {
			return fmt.Errorf("write overrides path: %w", err)
		}
		return nil
	case "validate", "check":
		return validateRuntimeConfiguration(args[1:], out)
	case "help", "-h", "--help":
		printConfigUsage(out)
		return nil
	default:
		printConfigUsage(out)
		return fmt.Errorf("unknown config subcommand: %s", subcommand)
	}
}

type overrideMutation func(*runtimeconfig.Overrides, string, string) error

func mutateOverrides(envLookup runtimeconfig.EnvLookup, key, value string, fn overrideMutation) error {
	overrides, err := loadManagedOverrides(envLookup)
	if err != nil {
		return fmt.Errorf("load managed overrides: %w", err)
	}
	if err := fn(&overrides, key, value); err != nil {
		return err
	}
	if err := saveManagedOverrides(envLookup, overrides); err != nil {
		return fmt.Errorf("save managed overrides: %w", err)
	}
	return nil
}

func printConfigSummary(out io.Writer, overridesPath string) error {
	cfg, meta, err := loadRuntimeConfigSnapshot()
	if err != nil {
		return fmt.Errorf("load runtime configuration: %w", err)
	}
	if _, err := fmt.Fprintln(out, "Current Configuration:"); err != nil {
		return fmt.Errorf("write header: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Provider:       %s\n", cfg.LLMProvider); err != nil {
		return fmt.Errorf("write provider: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Model:          %s\n", cfg.LLMModel); err != nil {
		return fmt.Errorf("write model: %w", err)
	}
	if cfg.LLMVisionModel != "" {
		if _, err := fmt.Fprintf(out, "  Vision Model:   %s\n", cfg.LLMVisionModel); err != nil {
			return fmt.Errorf("write vision model: %w", err)
		}
	}
	if _, err := fmt.Fprintf(out, "  Base URL:       %s\n", cfg.BaseURL); err != nil {
		return fmt.Errorf("write base url: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Max Tokens:     %d\n", cfg.MaxTokens); err != nil {
		return fmt.Errorf("write max tokens: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Max Iterations: %d\n", cfg.MaxIterations); err != nil {
		return fmt.Errorf("write max iterations: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Temperature:    %.2f\n", cfg.Temperature); err != nil {
		return fmt.Errorf("write temperature: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Top P:          %.2f\n", cfg.TopP); err != nil {
		return fmt.Errorf("write top p: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Profile:        %s\n", runtimeconfig.NormalizeRuntimeProfile(cfg.Profile)); err != nil {
		return fmt.Errorf("write profile: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Environment:    %s\n", cfg.Environment); err != nil {
		return fmt.Errorf("write environment: %w", err)
	}
	if _, err := fmt.Fprintf(out, "  Verbose:        %t\n", cfg.Verbose); err != nil {
		return fmt.Errorf("write verbose: %w", err)
	}
	if len(cfg.StopSequences) > 0 {
		if _, err := fmt.Fprintf(out, "  Stop Seqs:      %s\n", strings.Join(cfg.StopSequences, ", ")); err != nil {
			return fmt.Errorf("write stop sequences: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(out, "  Stop Seqs:      (not set)"); err != nil {
			return fmt.Errorf("write stop sequences missing: %w", err)
		}
	}
	if cfg.APIKey != "" {
		if _, err := fmt.Fprintln(out, "  API Key:        (set)"); err != nil {
			return fmt.Errorf("write api key set: %w", err)
		}
	} else {
		if _, err := fmt.Fprintln(out, "  API Key:        (not set)"); err != nil {
			return fmt.Errorf("write api key missing: %w", err)
		}
	}
	if _, err := fmt.Fprintf(out, "  Loaded At:      %s\n", meta.LoadedAt().Format(time.RFC3339)); err != nil {
		return fmt.Errorf("write loaded at: %w", err)
	}
	if _, err := fmt.Fprintf(out, "\nConfig file: %s\n", overridesPath); err != nil {
		return fmt.Errorf("write config file path: %w", err)
	}
	if _, err := fmt.Fprintln(out, "就绪检查:"); err != nil {
		return fmt.Errorf("write readiness heading: %w", err)
	}
	if _, err := fmt.Fprintln(out, readinessSummary(configadmin.DeriveReadinessTasks(cfg))); err != nil {
		return fmt.Errorf("write readiness summary: %w", err)
	}
	return nil
}

func printConfigUsage(out io.Writer) {
	lines := []string{
		"Config command usage:",
		"  alex config                       Show current configuration snapshot",
		"  alex config set <field> <value>   Persist a managed override (e.g. llm_model gpt-4o-mini)",
		"  alex config set field=value       Alternate set syntax",
		"  alex config clear <field>         Remove an override",
		"  alex config validate [--profile]  Validate runtime configuration",
		"  alex config path                  Print the runtime config file location",
		"",
		"Supported fields: llm_provider, llm_model, llm_vision_model, base_url, api_key, ark_api_key, tavily_api_key, profile, environment, max_tokens, max_iterations, temperature, top_p, verbose, stop_sequences, agent_preset, tool_preset.",
	}

	for _, line := range lines {
		if _, err := fmt.Fprintln(out, line); err != nil {
			fmt.Fprintf(os.Stderr, "print config usage: %v\n", err)
			return
		}
	}
}

func parseSetArgs(args []string) (string, string, error) {
	if len(args) == 0 {
		return "", "", fmt.Errorf("usage: alex config set <field> <value>")
	}
	if len(args) == 1 {
		if strings.Contains(args[0], "=") {
			parts := strings.SplitN(args[0], "=", 2)
			key := strings.TrimSpace(parts[0])
			value := ""
			if len(parts) > 1 {
				value = strings.TrimSpace(parts[1])
			}
			if key == "" || value == "" {
				return "", "", fmt.Errorf("usage: alex config set <field>=<value>")
			}
			return key, value, nil
		}
		return "", "", fmt.Errorf("usage: alex config set <field> <value>")
	}
	key := strings.TrimSpace(args[0])
	value := strings.TrimSpace(strings.Join(args[1:], " "))
	if key == "" || value == "" {
		return "", "", fmt.Errorf("usage: alex config set <field> <value>")
	}
	return key, value, nil
}

func readinessSummary(tasks []configadmin.ReadinessTask) string {
	if len(tasks) == 0 {
		return "  ✓ 所有关键配置均已就绪"
	}
	var builder strings.Builder
	for _, task := range tasks {
		fmt.Fprintf(&builder, "  [%s] %s\n", strings.ToUpper(string(task.Severity)), task.Label)
		if hint := strings.TrimSpace(task.Hint); hint != "" {
			fmt.Fprintf(&builder, "      ↳ %s\n", hint)
		}
	}
	return strings.TrimRight(builder.String(), "\n")
}

func validateRuntimeConfiguration(args []string, out io.Writer) error {
	cfg, _, err := loadRuntimeConfigSnapshot()
	if err != nil {
		return fmt.Errorf("load runtime configuration: %w", err)
	}

	profile := strings.TrimSpace(cfg.Profile)
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--profile", "-p":
			if i+1 >= len(args) {
				return fmt.Errorf("usage: alex config validate [--profile quickstart|standard|production]")
			}
			profile = strings.TrimSpace(args[i+1])
			i++
		default:
			return fmt.Errorf("usage: alex config validate [--profile quickstart|standard|production]")
		}
	}

	cfg.Profile = runtimeconfig.NormalizeRuntimeProfile(profile)
	report := runtimeconfig.ValidateRuntimeConfig(cfg)

	if _, err := fmt.Fprintf(out, "Validation Profile: %s\n", report.Profile); err != nil {
		return fmt.Errorf("write validation profile: %w", err)
	}
	if len(report.Errors) == 0 && len(report.Warnings) == 0 {
		if _, err := fmt.Fprintln(out, "STATUS: OK"); err != nil {
			return fmt.Errorf("write validation status: %w", err)
		}
	}
	for _, item := range report.Errors {
		if _, err := fmt.Fprintf(out, "ERROR %s: %s\n", item.ID, item.Message); err != nil {
			return fmt.Errorf("write validation error: %w", err)
		}
		if hint := strings.TrimSpace(item.Hint); hint != "" {
			if _, err := fmt.Fprintf(out, "  hint: %s\n", hint); err != nil {
				return fmt.Errorf("write validation error hint: %w", err)
			}
		}
	}
	for _, item := range report.Warnings {
		if _, err := fmt.Fprintf(out, "WARNING %s: %s\n", item.ID, item.Message); err != nil {
			return fmt.Errorf("write validation warning: %w", err)
		}
		if hint := strings.TrimSpace(item.Hint); hint != "" {
			if _, err := fmt.Fprintf(out, "  hint: %s\n", hint); err != nil {
				return fmt.Errorf("write validation warning hint: %w", err)
			}
		}
	}
	if len(report.DisabledTools) > 0 {
		if _, err := fmt.Fprintln(out, "Disabled tools:"); err != nil {
			return fmt.Errorf("write disabled tools heading: %w", err)
		}
		for _, item := range report.DisabledTools {
			if _, err := fmt.Fprintf(out, "  - %s: %s\n", item.Name, item.Reason); err != nil {
				return fmt.Errorf("write disabled tool: %w", err)
			}
		}
	}

	if report.HasErrors() {
		return fmt.Errorf("config validation failed with %d error(s)", len(report.Errors))
	}
	return nil
}
