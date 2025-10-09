package main

import (
	"alex/internal/config"
)

type appConfig = config.RuntimeConfig

func loadConfig() (appConfig, error) {
	cfg, _, err := config.Load()
	if err != nil {
		return appConfig{}, err
	}
	return cfg, nil
}
