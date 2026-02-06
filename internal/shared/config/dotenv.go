package config

import (
	"bufio"
	"bytes"
	"errors"
	"os"
	"path/filepath"
	"strings"
)

const dotEnvPathEnvVar = "ALEX_DOTENV_PATH"

// LoadDotEnv loads environment variables from a .env file without overriding
// existing process environment values.
func LoadDotEnv(paths ...string) error {
	if len(paths) == 0 {
		if value, ok := os.LookupEnv(dotEnvPathEnvVar); ok && strings.TrimSpace(value) != "" {
			paths = append(paths, strings.TrimSpace(value))
		} else {
			paths = append(paths, ".env")
		}
	}

	for _, path := range paths {
		if strings.TrimSpace(path) == "" {
			continue
		}
		expandedPath := expandDotEnvPath(path)
		if err := loadDotEnvFile(expandedPath); err != nil {
			if errors.Is(err, os.ErrNotExist) {
				continue
			}
			return err
		}
	}
	return nil
}

func expandDotEnvPath(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" || trimmed == "." {
		return ".env"
	}
	if strings.HasPrefix(trimmed, "~") {
		home, err := os.UserHomeDir()
		if err == nil && strings.TrimSpace(home) != "" {
			suffix := strings.TrimPrefix(trimmed, "~")
			suffix = strings.TrimPrefix(suffix, string(filepath.Separator))
			return filepath.Join(home, suffix)
		}
	}
	return trimmed
}

func loadDotEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	parsed := parseDotEnv(data)
	for key, value := range parsed {
		if key == "" {
			continue
		}
		if _, exists := os.LookupEnv(key); exists {
			continue
		}
		_ = os.Setenv(key, value)
	}
	return nil
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
