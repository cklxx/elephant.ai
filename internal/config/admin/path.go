package admin

import (
	"os"
	"path/filepath"
	"strings"

	runtimeconfig "alex/internal/config"
)

// ResolveStorePath determines the managed override store path for the current environment.
//
// Priority order:
//  1. Explicit CONFIG_ADMIN_STORE_PATH (or ALEX_CONFIG_STORE_PATH via env aliases).
//  2. $HOME/.alex/runtime-overrides.json (works for both CLI + server when sharing a home dir).
//  3. ./configs/runtime-overrides.json (legacy fallback when the home directory is unavailable).
func ResolveStorePath(envLookup runtimeconfig.EnvLookup) string {
	if envLookup == nil {
		envLookup = runtimeconfig.DefaultEnvLookup
	}
	if path := firstNonEmptyEnv(envLookup, "CONFIG_ADMIN_STORE_PATH"); path != "" {
		return path
	}
	if home := firstNonEmptyEnv(envLookup, "HOME", "USERPROFILE"); home != "" {
		return filepath.Join(home, ".alex", "runtime-overrides.json")
	}
	if home, err := os.UserHomeDir(); err == nil {
		if trimmed := strings.TrimSpace(home); trimmed != "" {
			return filepath.Join(trimmed, ".alex", "runtime-overrides.json")
		}
	}
	return filepath.Join("configs", "runtime-overrides.json")
}

func firstNonEmptyEnv(lookup runtimeconfig.EnvLookup, keys ...string) string {
	if lookup == nil {
		lookup = runtimeconfig.DefaultEnvLookup
	}
	for _, key := range keys {
		if value, ok := lookup(key); ok {
			if trimmed := strings.TrimSpace(value); trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}
