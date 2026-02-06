package main

import runtimeconfig "alex/internal/shared/config"

type appConfig = runtimeconfig.RuntimeConfig

func loadConfig() (appConfig, error) {
	cfg, _, err := loadRuntimeConfigSnapshot()
	if err != nil {
		return appConfig{}, err
	}
	return cfg, nil
}
