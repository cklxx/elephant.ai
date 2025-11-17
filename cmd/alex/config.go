package main

import (
	"alex/internal/config"
)

type appConfig = config.RuntimeConfig

func loadConfig(opts ...config.Option) (appConfig, error) {
	cfg, _, err := config.Load(opts...)
	if err != nil {
		return appConfig{}, err
	}
	return cfg, nil
}
