package bootstrap

import (
	"fmt"
	"strings"
	"time"

	"alex/internal/app/di"
	"alex/internal/shared/agent/presets"
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
	sessionDBURL := strings.TrimSpace(config.Session.DatabaseURL)
	if sessionDBURL == "" {
		sessionDBURL = strings.TrimSpace(config.Auth.DatabaseURL)
	}
	requireSessionDB := strings.EqualFold(config.Runtime.Environment, "production")
	if sessionDBURL == "" && requireSessionDB {
		return nil, fmt.Errorf("session database required in production (set session.database_url or auth.database_url in config.yaml)")
	}
	diConfig.SessionDatabaseURL = sessionDBURL
	if config.Session.PoolMaxConns != nil {
		diConfig.SessionPoolMaxConns = *config.Session.PoolMaxConns
	}
	if config.Session.PoolMinConns != nil {
		diConfig.SessionPoolMinConns = *config.Session.PoolMinConns
	}
	if config.Session.PoolMaxConnLifetimeSeconds != nil {
		diConfig.SessionPoolMaxConnLifetime = time.Duration(*config.Session.PoolMaxConnLifetimeSeconds) * time.Second
	}
	if config.Session.PoolMaxConnIdleSeconds != nil {
		diConfig.SessionPoolMaxConnIdleTime = time.Duration(*config.Session.PoolMaxConnIdleSeconds) * time.Second
	}
	if config.Session.PoolHealthCheckSeconds != nil {
		diConfig.SessionPoolHealthCheckPeriod = time.Duration(*config.Session.PoolHealthCheckSeconds) * time.Second
	}
	if config.Session.PoolConnectTimeoutSeconds != nil {
		diConfig.SessionPoolConnectTimeout = time.Duration(*config.Session.PoolConnectTimeoutSeconds) * time.Second
	}
	diConfig.RequireSessionDatabase = requireSessionDB
	diConfig.ToolMode = string(presets.ToolModeWeb)
	return di.BuildContainer(diConfig)
}
