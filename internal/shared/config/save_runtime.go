package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SaveRuntimeField persists a single runtime configuration key to the YAML config file.
// It follows the same load-merge-write pattern as SaveFollowPreferences.
func SaveRuntimeField(key string, value any, opts ...Option) (string, error) {
	options := loadOptions{
		envLookup: DefaultEnvLookup,
		readFile:  os.ReadFile,
		homeDir:   os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}

	configPath := options.configPath
	if configPath == "" {
		configPath, _ = ResolveConfigPath(options.envLookup, options.homeDir)
	}
	if configPath == "" {
		return "", fmt.Errorf("resolve config path: %w", os.ErrNotExist)
	}

	var existing map[string]any
	if options.readFile != nil {
		if data, err := options.readFile(configPath); err == nil {
			if len(bytes.TrimSpace(data)) > 0 {
				if err := yaml.Unmarshal(data, &existing); err != nil {
					return "", fmt.Errorf("parse config file: %w", err)
				}
			}
		} else if !errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("read config file: %w", err)
		}
	}
	if existing == nil {
		existing = map[string]any{}
	}

	runtimeSection, ok := existing["runtime"].(map[string]any)
	if !ok || runtimeSection == nil {
		runtimeSection = map[string]any{}
		existing["runtime"] = runtimeSection
	}
	runtimeSection[key] = value

	encoded, err := yaml.Marshal(existing)
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return "", fmt.Errorf("ensure config directory: %w", err)
	}
	if err := os.WriteFile(configPath, encoded, 0o600); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}

	return configPath, nil
}
