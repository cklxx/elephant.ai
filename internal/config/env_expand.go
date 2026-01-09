package config

import (
	"os"
	"strings"
)

func expandEnvValue(lookup EnvLookup, value string) string {
	if strings.TrimSpace(value) == "" {
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
