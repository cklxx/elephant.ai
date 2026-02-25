package config

import (
	"os"

	"alex/internal/shared/utils"
)

func expandEnvValue(lookup EnvLookup, value string) string {
	if utils.IsBlank(value) {
		return value
	}
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	return os.Expand(value, func(key string) string {
		if key == "" {
			return ""
		}
		if resolved, ok := lookup(key); ok {
			return resolved
		}
		return ""
	})
}
