package config

import (
	"os"
	"strings"
)

// SnapshotProcessEnv returns a map of the current process environment variables.
// The resulting map is a copy and safe for modification by callers.
func SnapshotProcessEnv() map[string]string {
	env := make(map[string]string)
	for _, kv := range os.Environ() {
		if kv == "" {
			continue
		}
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) != 2 {
			continue
		}
		env[parts[0]] = parts[1]
	}
	return env
}
