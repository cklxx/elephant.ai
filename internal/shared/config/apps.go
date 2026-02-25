package config

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"alex/internal/infra/filestore"
	"gopkg.in/yaml.v3"
)

// LoadAppsConfig returns the apps section from the main config file.
func LoadAppsConfig(opts ...Option) (AppsConfig, string, error) {
	fileCfg, path, err := LoadFileConfig(opts...)
	if err != nil {
		return AppsConfig{}, path, err
	}
	if fileCfg.Apps == nil {
		return AppsConfig{}, path, nil
	}
	return *fileCfg.Apps, path, nil
}

// SaveAppsConfig writes the apps section into the main config file.
func SaveAppsConfig(apps AppsConfig, opts ...Option) (string, error) {
	options := loadOptions{
		envLookup: DefaultEnvLookup,
		readFile:  os.ReadFile,
		homeDir:   os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}

	configPath := strings.TrimSpace(options.configPath)
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

	if len(apps.Plugins) == 0 {
		delete(existing, "apps")
	} else {
		existing["apps"] = apps
	}

	encoded, err := yaml.Marshal(existing)
	if err != nil {
		return "", fmt.Errorf("encode config: %w", err)
	}
	encoded = append(encoded, '\n')

	if err := filestore.AtomicWrite(configPath, encoded, 0o600); err != nil {
		return "", fmt.Errorf("write config file: %w", err)
	}

	return configPath, nil
}
