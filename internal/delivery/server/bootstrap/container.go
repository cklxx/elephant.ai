package bootstrap

import (
	"strings"

	"alex/internal/app/di"
	"alex/internal/domain/agent/presets"
)

// BuildContainer wires the shared DI container using the server runtime configuration.
func BuildContainer(config Config) (*di.Container, error) {
	diConfig := di.ConfigFromRuntimeConfig(config.Runtime)
	diConfig.EnableMCP = config.EnableMCP
	diConfig.EnvironmentSummary = config.EnvironmentSummary
	diConfig.SessionDir = strings.TrimSpace(config.Session.Dir)
	if strings.TrimSpace(diConfig.AgentPreset) == "" {
		diConfig.AgentPreset = string(presets.PresetArchitect)
	}
	if strings.TrimSpace(diConfig.ToolPreset) == "" {
		diConfig.ToolPreset = string(presets.ToolPresetArchitect)
	}
	diConfig.ToolMode = string(presets.ToolModeWeb)
	return di.BuildContainer(diConfig)
}
