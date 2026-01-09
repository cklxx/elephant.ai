package bootstrap

import (
	"fmt"
	"strings"

	"alex/internal/agent/presets"
	"alex/internal/di"
)

// BuildContainer wires the shared DI container using the server runtime configuration.
func BuildContainer(config Config) (*di.Container, error) {
	diConfig := di.ConfigFromRuntimeConfig(config.Runtime)
	diConfig.EnableMCP = config.EnableMCP
	diConfig.EnvironmentSummary = config.EnvironmentSummary
	diConfig.SessionDir = strings.TrimSpace(config.Session.Dir)
	sessionDBURL := strings.TrimSpace(config.Session.DatabaseURL)
	if sessionDBURL == "" {
		sessionDBURL = strings.TrimSpace(config.Auth.DatabaseURL)
	}
	requireSessionDB := strings.EqualFold(config.Runtime.Environment, "production")
	if sessionDBURL == "" && requireSessionDB {
		return nil, fmt.Errorf("session database required in production (set session.database_url or auth.database_url in config.yaml)")
	}
	diConfig.SessionDatabaseURL = sessionDBURL
	diConfig.RequireSessionDatabase = requireSessionDB
	diConfig.ToolMode = string(presets.ToolModeWeb)
	return di.BuildContainer(diConfig)
}
