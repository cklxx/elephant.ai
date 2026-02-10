package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

func defaultACPExecutorAddr(lookup EnvLookup) string {
	host := defaultACPHost(lookup)
	if port, ok := acpPortFromEnv(lookup); ok {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	if port, ok := readACPPortFile(); ok {
		return fmt.Sprintf("http://%s:%d", host, port)
	}
	return fmt.Sprintf("http://%s:%d", host, DefaultACPPort)
}

func defaultACPExecutorCWD() string {
	return "/workspace"
}

func defaultACPHost(lookup EnvLookup) string {
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	if value, ok := lookup("ACP_HOST"); ok {
		value = strings.TrimSpace(value)
		if value != "" {
			return value
		}
	}
	return DefaultACPHost
}

func acpPortFromEnv(lookup EnvLookup) (int, bool) {
	if lookup == nil {
		lookup = DefaultEnvLookup
	}
	value, ok := lookup("ACP_PORT")
	if !ok {
		return 0, false
	}
	value = strings.TrimSpace(value)
	if value == "" {
		return 0, false
	}
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}

func readACPPortFile() (int, bool) {
	configPath, _ := ResolveConfigPath(nil, nil)
	configPath = strings.TrimSpace(configPath)
	if configPath == "" {
		return 0, false
	}
	if !filepath.IsAbs(configPath) {
		abs, err := filepath.Abs(configPath)
		if err == nil {
			configPath = abs
		}
	}
	if resolved, err := filepath.EvalSymlinks(configPath); err == nil && strings.TrimSpace(resolved) != "" {
		configPath = resolved
	}
	path := filepath.Join(filepath.Dir(configPath), DefaultACPPortFile)
	data, err := os.ReadFile(path)
	if err != nil {
		return 0, false
	}
	value := strings.TrimSpace(string(data))
	port, err := strconv.Atoi(value)
	if err != nil || port <= 0 {
		return 0, false
	}
	return port, true
}
