package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	runtimeconfig "alex/internal/shared/config"

	"gopkg.in/yaml.v3"
)

const (
	setupRuntimeCLI     = "cli"
	setupRuntimeLark    = "lark"
	setupRuntimeFullDev = "full-dev"

	setupPersistenceFile   = "file"
	setupPersistenceMemory = "memory"
)

type setupSelectionInput struct {
	RuntimeMode     string
	LarkAppID       string
	LarkAppSecret   string
	PersistenceMode string
	PersistenceDir  string
}

type setupSelection struct {
	RuntimeMode     string
	LarkEnabled     bool
	LarkAppID       string
	LarkAppSecret   string
	PersistenceMode string
	PersistenceDir  string
}

func resolveSetupSelection(in io.Reader, out io.Writer, input setupSelectionInput) (setupSelection, error) {
	selection := setupSelection{
		RuntimeMode:     strings.TrimSpace(strings.ToLower(input.RuntimeMode)),
		LarkAppID:       strings.TrimSpace(input.LarkAppID),
		LarkAppSecret:   strings.TrimSpace(input.LarkAppSecret),
		PersistenceMode: strings.TrimSpace(strings.ToLower(input.PersistenceMode)),
		PersistenceDir:  strings.TrimSpace(input.PersistenceDir),
	}

	if selection.RuntimeMode == "" {
		if isInteractiveTerminal(in, out) {
			mode, err := chooseSetupRuntimeMode(in, out)
			if err != nil {
				return setupSelection{}, err
			}
			selection.RuntimeMode = mode
		} else {
			selection.RuntimeMode = setupRuntimeCLI
		}
	}

	switch selection.RuntimeMode {
	case setupRuntimeCLI, setupRuntimeLark, setupRuntimeFullDev:
	default:
		return setupSelection{}, fmt.Errorf("--runtime must be one of cli|lark|full-dev")
	}

	selection.LarkEnabled = selection.RuntimeMode == setupRuntimeLark || selection.RuntimeMode == setupRuntimeFullDev
	if !selection.LarkEnabled {
		selection.PersistenceMode = ""
		selection.PersistenceDir = ""
		return selection, nil
	}

	if selection.LarkAppID == "" {
		if !isInteractiveTerminal(in, out) {
			return setupSelection{}, fmt.Errorf("--lark-app-id is required when runtime includes lark")
		}
		value, err := promptLine(in, out, "Lark app_id")
		if err != nil {
			return setupSelection{}, err
		}
		selection.LarkAppID = strings.TrimSpace(value)
	}
	if selection.LarkAppSecret == "" {
		if !isInteractiveTerminal(in, out) {
			return setupSelection{}, fmt.Errorf("--lark-app-secret is required when runtime includes lark")
		}
		value, err := promptLine(in, out, "Lark app_secret")
		if err != nil {
			return setupSelection{}, err
		}
		selection.LarkAppSecret = strings.TrimSpace(value)
	}
	if selection.LarkAppID == "" || selection.LarkAppSecret == "" {
		return setupSelection{}, fmt.Errorf("lark app_id and app_secret are required when runtime includes lark")
	}

	if selection.PersistenceMode == "" {
		if isInteractiveTerminal(in, out) {
			mode, err := chooseSetupPersistenceMode(in, out)
			if err != nil {
				return setupSelection{}, err
			}
			selection.PersistenceMode = mode
		} else {
			selection.PersistenceMode = setupPersistenceFile
		}
	}

	switch selection.PersistenceMode {
	case setupPersistenceFile, setupPersistenceMemory:
	default:
		return setupSelection{}, fmt.Errorf("--persistence-mode must be one of file|memory")
	}

	if selection.PersistenceMode == setupPersistenceFile {
		if selection.PersistenceDir == "" {
			selection.PersistenceDir = "~/.alex/lark"
		}
	} else {
		selection.PersistenceDir = ""
	}

	return selection, nil
}

func chooseSetupRuntimeMode(in io.Reader, out io.Writer) (string, error) {
	if _, err := fmt.Fprintln(out, "\nSelect runtime mode:"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(out, "  1) cli"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(out, "  2) lark"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(out, "  3) full-dev"); err != nil {
		return "", err
	}
	choice, err := readChoice(in, out, 2, 3)
	if err != nil {
		return "", err
	}
	switch choice {
	case 1:
		return setupRuntimeCLI, nil
	case 2:
		return setupRuntimeLark, nil
	default:
		return setupRuntimeFullDev, nil
	}
}

func chooseSetupPersistenceMode(in io.Reader, out io.Writer) (string, error) {
	if _, err := fmt.Fprintln(out, "\nSelect Lark persistence mode:"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(out, "  1) file (recommended)"); err != nil {
		return "", err
	}
	if _, err := fmt.Fprintln(out, "  2) memory"); err != nil {
		return "", err
	}
	choice, err := readChoice(in, out, 1, 2)
	if err != nil {
		return "", err
	}
	if choice == 2 {
		return setupPersistenceMemory, nil
	}
	return setupPersistenceFile, nil
}

func promptLine(in io.Reader, out io.Writer, label string) (string, error) {
	reader := bufio.NewReader(in)
	for {
		if _, err := fmt.Fprintf(out, "%s: ", label); err != nil {
			return "", err
		}
		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}
		value := strings.TrimSpace(line)
		if value != "" {
			return value, nil
		}
		if err == io.EOF {
			return "", fmt.Errorf("%s is required", label)
		}
	}
}

func applyLarkSetupConfig(envLookup runtimeconfig.EnvLookup, selection setupSelection) (string, error) {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	configPath, _ := runtimeconfig.ResolveConfigPath(envLookup, os.UserHomeDir)
	if strings.TrimSpace(configPath) == "" {
		return "", fmt.Errorf("resolve config path failed")
	}

	root := map[string]any{}
	if data, err := os.ReadFile(configPath); err == nil {
		if len(strings.TrimSpace(string(data))) > 0 {
			if err := yaml.Unmarshal(data, &root); err != nil {
				return "", fmt.Errorf("parse config file %s: %w", configPath, err)
			}
		}
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("read config file %s: %w", configPath, err)
	}

	channels := ensureStringMap(root, "channels")
	larkCfg := ensureStringMap(channels, "lark")
	larkCfg["enabled"] = true
	larkCfg["app_id"] = selection.LarkAppID
	larkCfg["app_secret"] = selection.LarkAppSecret

	persistence := ensureStringMap(larkCfg, "persistence")
	persistence["mode"] = selection.PersistenceMode
	if selection.PersistenceMode == setupPersistenceFile {
		persistence["dir"] = selection.PersistenceDir
	} else {
		delete(persistence, "dir")
	}

	output, err := yaml.Marshal(root)
	if err != nil {
		return "", fmt.Errorf("marshal config file %s: %w", configPath, err)
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return "", fmt.Errorf("create config dir: %w", err)
	}
	if err := os.WriteFile(configPath, output, 0o600); err != nil {
		return "", fmt.Errorf("write config file %s: %w", configPath, err)
	}
	return configPath, nil
}

func ensureStringMap(root map[string]any, key string) map[string]any {
	if root == nil {
		return map[string]any{}
	}
	raw, ok := root[key]
	if !ok {
		child := map[string]any{}
		root[key] = child
		return child
	}
	if child, ok := raw.(map[string]any); ok {
		return child
	}
	if child, ok := raw.(map[string]interface{}); ok {
		converted := make(map[string]any, len(child))
		for k, v := range child {
			converted[k] = v
		}
		root[key] = converted
		return converted
	}
	converted := map[string]any{}
	root[key] = converted
	return converted
}
