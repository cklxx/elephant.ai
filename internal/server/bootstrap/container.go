package bootstrap

import "alex/internal/di"

// BuildContainer wires the shared DI container using the server runtime configuration.
func BuildContainer(config Config) (*di.Container, error) {
	diConfig := di.ConfigFromRuntimeConfig(config.Runtime)
	diConfig.EnableMCP = config.EnableMCP
	diConfig.EnvironmentSummary = config.EnvironmentSummary
	return di.BuildContainer(diConfig)
}
