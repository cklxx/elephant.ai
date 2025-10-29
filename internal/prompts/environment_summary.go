package prompts

import (
	"fmt"
	"sort"
	"strings"
)

var sensitiveFragments = []string{"key", "secret", "token", "password", "authorization", "cookie"}

// FormatEnvironmentSummary renders a concise summary of host and sandbox environments.
func FormatEnvironmentSummary(hostEnv, sandboxEnv map[string]string) string {
	keys := make([]string, 0, len(hostEnv)+len(sandboxEnv))
	seen := map[string]struct{}{}

	for k := range hostEnv {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}
	for k := range sandboxEnv {
		if _, ok := seen[k]; !ok {
			seen[k] = struct{}{}
			keys = append(keys, k)
		}
	}

	sort.Strings(keys)

	var builder strings.Builder
	builder.WriteString("Environment context:\n")
	for _, key := range keys {
		hostVal := redactValue(key, hostEnv[key])
		sandboxVal := redactValue(key, sandboxEnv[key])

		switch {
		case hostVal != "" && sandboxVal != "":
			builder.WriteString(fmt.Sprintf("- %s: host=%q, sandbox=%q\n", key, hostVal, sandboxVal))
		case sandboxVal != "":
			builder.WriteString(fmt.Sprintf("- %s: sandbox=%q\n", key, sandboxVal))
		default:
			builder.WriteString(fmt.Sprintf("- %s: host=%q\n", key, hostVal))
		}
	}

	return strings.TrimSpace(builder.String())
}

func redactValue(key, value string) string {
	if value == "" {
		return value
	}
	lower := strings.ToLower(key)
	for _, fragment := range sensitiveFragments {
		if strings.Contains(lower, fragment) {
			if len(value) <= 4 {
				return "***"
			}
			return value[:2] + "***" + value[len(value)-2:]
		}
	}
	return value
}
