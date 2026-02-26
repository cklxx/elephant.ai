package config

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"alex/internal/shared/utils"
)

const dotEnvPathEnvVar = "ALEX_DOTENV_PATH"

var managedDotEnvState = struct {
	mu   sync.Mutex
	keys map[string]struct{}
}{
	keys: map[string]struct{}{},
}

// LoadDotEnv loads environment variables from a .env file without overriding
// existing process environment values.
func LoadDotEnv(paths ...string) error {
	resolved := defaultDotEnvPaths(paths, os.LookupEnv)
	for _, path := range resolved {
		values, err := readDotEnvFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		applyDotEnvValues(values, false)
	}
	return nil
}

// ReloadManagedDotEnv reloads environment variables from .env and updates only
// keys previously managed by LoadDotEnv, while preserving externally injected
// environment variables.
func ReloadManagedDotEnv(paths ...string) error {
	resolved := defaultDotEnvPaths(paths, os.LookupEnv)
	merged := make(map[string]string)
	for _, path := range resolved {
		values, err := readDotEnvFile(path)
		if err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
		for key, value := range values {
			if _, exists := merged[key]; exists {
				continue
			}
			merged[key] = value
		}
	}
	applyDotEnvValues(merged, true)
	return nil
}

// DefaultDotEnvPaths returns the dotenv paths that should be loaded/watched.
func DefaultDotEnvPaths(envLookup EnvLookup) []string {
	return defaultDotEnvPaths(nil, envLookup)
}

func defaultDotEnvPaths(paths []string, envLookup EnvLookup) []string {
	if len(paths) == 0 {
		if envLookup == nil {
			envLookup = os.LookupEnv
		}
		if value, ok := envLookup(dotEnvPathEnvVar); ok && utils.HasContent(value) {
			paths = append(paths, strings.TrimSpace(value))
		} else {
			paths = append(paths, ".env")
		}
	}
	resolved := make([]string, 0, len(paths))
	for _, path := range paths {
		if utils.IsBlank(path) {
			continue
		}
		resolved = append(resolved, expandDotEnvPath(path))
	}
	return resolved
}

func expandDotEnvPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "." {
		return ".env"
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err == nil && utils.HasContent(home) {
			suffix := strings.TrimPrefix(trimmed, "~")
			suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
			return filepath.Join(home, suffix)
		}
	}
	return trimmed
}

func readDotEnvFile(path string) (map[string]string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parseDotEnv(data), nil
}

func applyDotEnvValues(values map[string]string, reloadManaged bool) {
	if len(values) == 0 && !reloadManaged {
		return
	}
	managedDotEnvState.mu.Lock()
	defer managedDotEnvState.mu.Unlock()

	if reloadManaged {
		for key := range managedDotEnvState.keys {
			if _, ok := values[key]; ok {
				continue
			}
			_ = os.Unsetenv(key)
			delete(managedDotEnvState.keys, key)
		}
	}

	for key, value := range values {
		if key == "" {
			continue
		}
		if reloadManaged {
			if _, managed := managedDotEnvState.keys[key]; managed {
				_ = os.Setenv(key, value)
				continue
			}
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
		managedDotEnvState.keys[key] = struct{}{}
	}
}

func parseDotEnv(data []byte) map[string]string {
	result := make(map[string]string)
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		key, value, ok := parseDotEnvLine(line)
		if !ok {
			continue
		}
		result[key] = value
	}
	return result
}

func parseDotEnvLine(line string) (string, string, bool) {
	if line == "" || strings.HasPrefix(line, "#") {
		return "", "", false
	}

	parts := strings.SplitN(line, "=", 2)
	if len(parts) != 2 {
		return "", "", false
	}

	key := strings.TrimSpace(parts[0])
	if key == "" {
		return "", "", false
	}

	value := strings.TrimSpace(parts[1])
	if value == "" {
		return key, "", true
	}

	if len(value) >= 2 {
		if value[0] == '"' && value[len(value)-1] == '"' {
			return key, unescapeDotEnvDoubleQuoted(value[1 : len(value)-1]), true
		}
		if value[0] == '\'' && value[len(value)-1] == '\'' {
			return key, value[1 : len(value)-1], true
		}
	}

	return key, stripDotEnvInlineComment(value), true
}

func stripDotEnvInlineComment(value string) string {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return trimmed
	}

	var prev rune
	for index, r := range trimmed {
		if r == '#' && (index == 0 || isWhitespaceRune(prev)) {
			return strings.TrimSpace(trimmed[:index])
		}
		prev = r
	}
	return trimmed
}

func isWhitespaceRune(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func unescapeDotEnvDoubleQuoted(value string) string {
	var builder strings.Builder
	builder.Grow(len(value))
	for i := 0; i < len(value); i++ {
		current := value[i]
		if current != '\\' || i+1 >= len(value) {
			builder.WriteByte(current)
			continue
		}
		next := value[i+1]
		switch next {
		case 'n':
			builder.WriteByte('\n')
		case 'r':
			builder.WriteByte('\r')
		case 't':
			builder.WriteByte('\t')
		case '\\':
			builder.WriteByte('\\')
		case '"':
			builder.WriteByte('"')
		default:
			builder.WriteByte(next)
		}
		i++
	}
	return builder.String()
}
