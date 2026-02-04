package config

import (
	"os"
	"path/filepath"
	"strings"
)

const defaultTestConfigName = "test.yaml"

// DefaultRuntimeConfigWatchPaths returns a stable, de-duplicated list of config
// paths that should be watched for runtime reload triggers.
//
// Order:
//  1. The resolved runtime config path (ALEX_CONFIG_PATH or default).
//  2. ~/.alex/config.yaml (when home is available).
//  3. ~/.alex/test.yaml (when home is available).
func DefaultRuntimeConfigWatchPaths(envLookup EnvLookup, homeDir func() (string, error)) []string {
	resolved, _ := ResolveConfigPath(envLookup, homeDir)

	home := ""
	if homeDir != nil {
		if resolvedHome, err := homeDir(); err == nil {
			home = strings.TrimSpace(resolvedHome)
		}
	}
	if home == "" {
		if resolvedHome, err := os.UserHomeDir(); err == nil {
			home = strings.TrimSpace(resolvedHome)
		}
	}

	defaultConfigPath := ""
	defaultTestPath := ""
	if home != "" {
		defaultConfigPath = filepath.Join(home, defaultConfigDir, defaultConfigName)
		defaultTestPath = filepath.Join(home, defaultConfigDir, defaultTestConfigName)
	}

	seen := make(map[string]struct{}, 3)
	paths := make([]string, 0, 3)
	add := func(path string) {
		path = strings.TrimSpace(path)
		if path == "" {
			return
		}
		if abs, err := filepath.Abs(path); err == nil {
			path = abs
		}
		path = filepath.Clean(path)
		if _, ok := seen[path]; ok {
			return
		}
		seen[path] = struct{}{}
		paths = append(paths, path)
	}

	add(resolved)
	add(defaultConfigPath)
	add(defaultTestPath)
	return paths
}
