package mcp

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"alex/internal/shared/logging"
)

// Config represents the MCP configuration file structure
type Config struct {
	MCPServers map[string]ServerConfig `json:"mcpServers"`
}

// ServerConfig represents the configuration for a single MCP server
type ServerConfig struct {
	Command  string            `json:"command"`
	Args     []string          `json:"args,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	Disabled bool              `json:"disabled,omitempty"`
}

// ConfigScope defines where the configuration is loaded from
type ConfigScope string

const (
	ScopeLocal   ConfigScope = "local"   // .mcp.json in current directory
	ScopeProject ConfigScope = "project" // .mcp.json in git root
	ScopeUser    ConfigScope = "user"    // ~/.alex/.mcp.json
)

// ConfigLoader loads and merges MCP configurations from different scopes
type ConfigLoader struct {
	logger logging.Logger
}

// NewConfigLoader creates a new configuration loader
func NewConfigLoader(opts ...ConfigLoaderOption) *ConfigLoader {
	loader := &ConfigLoader{
		logger: logging.NewComponentLogger("ConfigLoader"),
	}
	for _, opt := range opts {
		opt(loader)
	}
	return loader
}

// ConfigLoaderOption customises the MCP configuration loader behaviour.
type ConfigLoaderOption func(*ConfigLoader)

// Load loads and merges configurations from all scopes
// Priority: local > project > user (local overrides project, project overrides user)
func (l *ConfigLoader) Load() (*Config, error) {
	l.logger.Debug("Loading MCP configurations from all scopes")

	merged := &Config{
		MCPServers: make(map[string]ServerConfig),
	}

	// Load user config first (lowest priority)
	if userConfig, err := l.loadUserConfig(); err == nil {
		l.mergeConfig(merged, userConfig)
		l.logger.Debug("Loaded user config: %d servers", len(userConfig.MCPServers))
	} else {
		l.logger.Debug("No user config found: %v", err)
	}

	// Load project config (medium priority)
	if projectConfig, err := l.loadProjectConfig(); err == nil {
		l.mergeConfig(merged, projectConfig)
		l.logger.Debug("Loaded project config: %d servers", len(projectConfig.MCPServers))
	} else {
		l.logger.Debug("No project config found: %v", err)
	}

	// Load local config last (highest priority)
	if localConfig, err := l.loadLocalConfig(); err == nil {
		l.mergeConfig(merged, localConfig)
		l.logger.Debug("Loaded local config: %d servers", len(localConfig.MCPServers))
	} else {
		l.logger.Debug("No local config found: %v", err)
	}

	l.logger.Info("Total MCP servers configured: %d", len(merged.MCPServers))
	return merged, nil
}

// LoadFromPath loads configuration from a specific file path
func (l *ConfigLoader) LoadFromPath(path string) (*Config, error) {
	l.logger.Debug("Loading MCP config from: %s", path)

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var config Config
	if err := json.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	// Expand environment variables in all configs
	for name, serverConfig := range config.MCPServers {
		config.MCPServers[name] = l.expandEnvVars(serverConfig)
	}

	return &config, nil
}

// SaveToPath saves configuration to a specific file path
func (l *ConfigLoader) SaveToPath(path string, config *Config) error {
	l.logger.Debug("Saving MCP config to: %s", path)

	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to JSON with indentation
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	l.logger.Info("Saved MCP config to: %s", path)
	return nil
}

// loadUserConfig loads configuration from ~/.alex/.mcp.json
func (l *ConfigLoader) loadUserConfig() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home directory: %w", err)
	}

	configPath := filepath.Join(homeDir, ".alex", ".mcp.json")
	return l.LoadFromPath(configPath)
}

// loadProjectConfig loads configuration from git root/.mcp.json
func (l *ConfigLoader) loadProjectConfig() (*Config, error) {
	gitRoot, err := l.findGitRoot()
	if err != nil {
		return nil, fmt.Errorf("not in a git repository: %w", err)
	}

	configPath := filepath.Join(gitRoot, ".mcp.json")
	return l.LoadFromPath(configPath)
}

// loadLocalConfig loads configuration from ./.mcp.json
func (l *ConfigLoader) loadLocalConfig() (*Config, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, fmt.Errorf("failed to get current directory: %w", err)
	}

	configPath := filepath.Join(cwd, ".mcp.json")
	return l.LoadFromPath(configPath)
}

// findGitRoot finds the git repository root directory
func (l *ConfigLoader) findGitRoot() (string, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return "", err
	}

	dir := cwd
	for {
		gitDir := filepath.Join(dir, ".git")
		if info, err := os.Stat(gitDir); err == nil && info.IsDir() {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			// Reached filesystem root
			return "", fmt.Errorf("not in a git repository")
		}
		dir = parent
	}
}

// mergeConfig merges source config into target (source overwrites target)
func (l *ConfigLoader) mergeConfig(target *Config, source *Config) {
	for name, serverConfig := range source.MCPServers {
		target.MCPServers[name] = serverConfig
	}
}

// expandEnvVars expands environment variables in server configuration
func (l *ConfigLoader) expandEnvVars(config ServerConfig) ServerConfig {
	// Expand in command
	config.Command = l.expandString(config.Command)

	// Expand in args
	for i, arg := range config.Args {
		config.Args[i] = l.expandString(arg)
	}

	// Expand in env values
	if config.Env != nil {
		expanded := make(map[string]string, len(config.Env))
		for k, v := range config.Env {
			expanded[k] = l.expandString(v)
		}
		config.Env = expanded
	}

	return config
}

// expandString expands ${VAR} and $VAR environment variables
func (l *ConfigLoader) expandString(s string) string {
	return os.Expand(s, func(key string) string {
		value, ok := os.LookupEnv(key)
		if !ok || value == "" {
			l.logger.Warn("Environment variable not found: %s", key)
			return ""
		}
		return value
	})
}

// AddServer adds or updates a server configuration
func (c *Config) AddServer(name string, config ServerConfig) {
	if c.MCPServers == nil {
		c.MCPServers = make(map[string]ServerConfig)
	}
	c.MCPServers[name] = config
}

// RemoveServer removes a server configuration
func (c *Config) RemoveServer(name string) bool {
	if _, exists := c.MCPServers[name]; exists {
		delete(c.MCPServers, name)
		return true
	}
	return false
}

// GetServer retrieves a server configuration by name
func (c *Config) GetServer(name string) (ServerConfig, bool) {
	config, exists := c.MCPServers[name]
	return config, exists
}

// ListServers returns all server names
func (c *Config) ListServers() []string {
	names := make([]string, 0, len(c.MCPServers))
	for name := range c.MCPServers {
		names = append(names, name)
	}
	return names
}

// GetActiveServers returns all enabled server configurations
func (c *Config) GetActiveServers() map[string]ServerConfig {
	active := make(map[string]ServerConfig)
	for name, config := range c.MCPServers {
		if !config.Disabled {
			active[name] = config
		}
	}
	return active
}

// Validate validates the configuration
func (c *Config) Validate() error {
	if c.MCPServers == nil {
		return fmt.Errorf("no MCP servers configured")
	}

	for name, config := range c.MCPServers {
		if config.Command == "" {
			return fmt.Errorf("server '%s': command is required", name)
		}

		// Check if command contains invalid characters
		if strings.ContainsAny(config.Command, "\n\r") {
			return fmt.Errorf("server '%s': command contains invalid characters", name)
		}
	}

	return nil
}
