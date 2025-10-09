package config

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

// SaveFollowPreferences persists the default follow behaviour to the runtime configuration file.
// It merges the new defaults with any existing configuration values and returns the path that was updated.
func SaveFollowPreferences(followTranscript, followStream bool, opts ...Option) (string, error) {
	options := loadOptions{
		readFile: os.ReadFile,
		homeDir:  os.UserHomeDir,
	}
	for _, opt := range opts {
		opt(&options)
	}

	configPath := options.configPath
	if configPath == "" {
		home, err := options.homeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		configPath = filepath.Join(home, ".alex-config.json")
	}

	var existing map[string]any
	if options.readFile != nil {
		if data, err := options.readFile(configPath); err == nil {
			if len(data) > 0 {
				if err := json.Unmarshal(data, &existing); err != nil {
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

	existing["follow_transcript"] = followTranscript
	existing["follow_stream"] = followStream

	encoded, err := json.MarshalIndent(existing, "", "  ")
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
