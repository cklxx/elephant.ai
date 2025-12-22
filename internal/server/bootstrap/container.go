package bootstrap

import (
	"strings"

	"alex/internal/agent/presets"
	"alex/internal/di"
)

// BuildContainer wires the shared DI container using the server runtime configuration.
func BuildContainer(config Config) (*di.Container, error) {
	diConfig := di.ConfigFromRuntimeConfig(config.Runtime)
	diConfig.EnableMCP = config.EnableMCP
	diConfig.EnvironmentSummary = config.EnvironmentSummary
	diConfig.SessionDatabaseURL = strings.TrimSpace(config.Auth.DatabaseURL)
	diConfig.ToolMode = string(presets.ToolModeWeb)
	return di.BuildContainer(diConfig)
}
