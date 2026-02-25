package admin

import (
	runtimeconfig "alex/internal/shared/config"
)

// ResolveStorePath determines the managed override store path for the current environment.
// Managed overrides are stored inside the unified config file.
func ResolveStorePath(envLookup runtimeconfig.EnvLookup) string {
	path, _ := runtimeconfig.ResolveConfigPath(envLookup, nil)
	return path
}
