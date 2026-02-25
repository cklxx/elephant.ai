package config

import (
	"os"
	"path/filepath"
	"strings"
)

const (
	defaultConfigDir  = ".alex"
	defaultConfigName = "config.yaml"
)

// ResolveConfigPath returns the runtime configuration file path and its source label.
// Priority order:
//  1. Explicit ALEX_CONFIG_PATH.
//  2. $HOME/.alex/config.yaml.
//  3. ./configs/config.yaml (fallback when the home directory is unavailable).
func ResolveConfigPath(envLookup EnvLookup, homeDir func() (string, error)) (string, string) {
	if envLookup == nil {
		envLookup = DefaultEnvLookup
	}
	if value, ok := envLookup("ALEX_CONFIG_PATH"); ok {
		if trimmed := strings.TrimSpace(value); trimmed != "" {
			return trimmed, "ALEX_CONFIG_PATH"
		}
	}

	home := ""
	if homeDir != nil {
		if resolved, err := homeDir(); err == nil {
			home = strings.TrimSpace(resolved)
		}
	}
	if home == "" {
		if resolved, err := os.UserHomeDir(); err == nil {
			home = strings.TrimSpace(resolved)
		}
	}
	if home != "" {
		return filepath.Join(home, defaultConfigDir, defaultConfigName), "default"
	}

	return filepath.Join("configs", defaultConfigName), "fallback"
}
