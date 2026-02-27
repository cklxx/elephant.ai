package process

import (
	"fmt"
	"os"
	"strings"
)

// MergeEnv merges overrides into the current process environment.
//
// Semantics:
//   - A key mapped to "" removes (unsets) that variable from the inherited env.
//   - A key mapped to a non-empty value overrides any inherited value.
//   - Inherited variables not mentioned in overrides pass through unchanged.
func MergeEnv(overrides map[string]string) []string {
	inherited := os.Environ()
	env := make([]string, 0, len(inherited)+len(overrides))
	for _, entry := range inherited {
		key := entry
		if idx := strings.IndexByte(entry, '='); idx >= 0 {
			key = entry[:idx]
		}
		if _, has := overrides[key]; has {
			continue // will be set (or unset) by overrides below
		}
		env = append(env, entry)
	}

	for k, v := range overrides {
		if v != "" {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
	}
	return env
}
