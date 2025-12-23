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
	if sessionDBURL == "" {
		return nil, fmt.Errorf("web mode requires session database (set ALEX_SESSION_DATABASE_URL or AUTH_DATABASE_URL)")
	}
	diConfig.SessionDatabaseURL = sessionDBURL
	diConfig.RequireSessionDatabase = true
	diConfig.ToolMode = string(presets.ToolModeWeb)
	return di.BuildContainer(diConfig)
}
