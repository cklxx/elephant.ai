package mcp

import (
	"encoding/json"
	"fmt"
	"time"
)

// MCPConfig represents the complete MCP configuration
type MCPConfig struct {
	Enabled         bool                     `json:"enabled"`
	Servers         map[string]*ServerConfig `json:"servers"`
	GlobalTimeout   time.Duration            `json:"global_timeout"`
	AutoRefresh     bool                     `json:"auto_refresh"`
	RefreshInterval time.Duration            `json:"refresh_interval"`
	Security        *SecurityConfig          `json:"security,omitempty"`
	Logging         *LoggingConfig           `json:"logging,omitempty"`
}

// SecurityConfig represents security settings for MCP
type SecurityConfig struct {
	AllowedCommands     []string          `json:"allowed_commands"`
	BlockedCommands     []string          `json:"blocked_commands"`
	AllowedPackages     []string          `json:"allowed_packages"`
	BlockedPackages     []string          `json:"blocked_packages"`
	RequireConfirmation bool              `json:"require_confirmation"`
	SandboxMode         bool              `json:"sandbox_mode"`
	MaxProcesses        int               `json:"max_processes"`
	MaxMemoryMB         int               `json:"max_memory_mb"`
	AllowedEnvironment  map[string]string `json:"allowed_environment"`
	RestrictedPaths     []string          `json:"restricted_paths"`
}

// LoggingConfig represents logging settings for MCP
type LoggingConfig struct {
	Level        string `json:"level"` // debug, info, warn, error
	LogRequests  bool   `json:"log_requests"`
	LogResponses bool   `json:"log_responses"`
	LogFile      string `json:"log_file,omitempty"`
}

// GetDefaultMCPConfig returns the default MCP configuration
func GetDefaultMCPConfig() *MCPConfig {
	return &MCPConfig{
		Enabled:         true,
		Servers:         make(map[string]*ServerConfig),
		GlobalTimeout:   30 * time.Second,
		AutoRefresh:     true,
		RefreshInterval: 5 * time.Minute,
		Security: &SecurityConfig{
			AllowedCommands: []string{
				"npx",
				"node",
				"python",
				"python3",
			},
			BlockedCommands: []string{
				"rm",
				"rmdir",
				"del",
				"format",
				"sudo",
				"su",
			},
			AllowedPackages: []string{
				"@modelcontextprotocol/server-*",
				"mcp-*",
			},
			RequireConfirmation: false,
			SandboxMode:         true,
			MaxProcesses:        10,
			MaxMemoryMB:         512,
			AllowedEnvironment: map[string]string{
				"NODE_ENV": "production",
				"PATH":     "",
			},
			RestrictedPaths: []string{
				"/etc",
				"/var",
				"/usr",
				"/bin",
				"/sbin",
				"/root",
				"/home",
				"/tmp",
				"/System",
				"/Library",
				"/Applications",
				"/Volumes",
			},
		},
		Logging: &LoggingConfig{
			Level:        "info",
			LogRequests:  true,
			LogResponses: false,
			LogFile:      "",
		},
	}
}

// AddServerConfig adds a new server configuration
func (c *MCPConfig) AddServerConfig(config *ServerConfig) error {
	if err := ValidateServerConfig(config); err != nil {
		return fmt.Errorf("invalid server config: %w", err)
	}

	if c.Servers == nil {
		c.Servers = make(map[string]*ServerConfig)
	}

	c.Servers[config.ID] = config
	return nil
}

// RemoveServerConfig removes a server configuration
func (c *MCPConfig) RemoveServerConfig(id string) {
	if c.Servers != nil {
		delete(c.Servers, id)
	}
}

// GetServerConfig returns a server configuration by ID
func (c *MCPConfig) GetServerConfig(id string) (*ServerConfig, bool) {
	if c.Servers == nil {
		return nil, false
	}
	config, exists := c.Servers[id]
	return config, exists
}

// ListServerConfigs returns all server configurations
func (c *MCPConfig) ListServerConfigs() []*ServerConfig {
	if c.Servers == nil {
		return nil
	}

	configs := make([]*ServerConfig, 0, len(c.Servers))
	for _, config := range c.Servers {
		configs = append(configs, config)
	}
	return configs
}

// GetEnabledServers returns only enabled server configurations
func (c *MCPConfig) GetEnabledServers() []*ServerConfig {
	configs := c.ListServerConfigs()
	enabled := make([]*ServerConfig, 0, len(configs))
	for _, config := range configs {
		if config.Enabled {
			enabled = append(enabled, config)
		}
	}
	return enabled
}

// ValidateConfig validates the MCP configuration
func (c *MCPConfig) ValidateConfig() error {
	// Validate timeout values
	if c.GlobalTimeout <= 0 {
		return fmt.Errorf("global_timeout must be positive")
	}

	if c.RefreshInterval <= 0 {
		return fmt.Errorf("refresh_interval must be positive")
	}

	// Validate security settings
	if c.Security != nil {
		if c.Security.MaxProcesses < 1 {
			return fmt.Errorf("max_processes must be at least 1")
		}
		if c.Security.MaxMemoryMB < 1 {
			return fmt.Errorf("max_memory_mb must be at least 1")
		}
	}

	// Validate logging settings
	if c.Logging != nil {
		validLevels := map[string]bool{
			"debug": true,
			"info":  true,
			"warn":  true,
			"error": true,
		}
		if !validLevels[c.Logging.Level] {
			return fmt.Errorf("invalid log level: %s", c.Logging.Level)
		}
	}

	// Validate all server configurations
	for _, serverConfig := range c.Servers {
		if err := ValidateServerConfig(serverConfig); err != nil {
			return fmt.Errorf("invalid server config %s: %w", serverConfig.ID, err)
		}
	}

	return nil
}

// ToJSON converts the configuration to JSON
func (c *MCPConfig) ToJSON() ([]byte, error) {
	return json.MarshalIndent(c, "", "    ")
}

// FromJSON loads configuration from JSON
func (c *MCPConfig) FromJSON(data []byte) error {
	return json.Unmarshal(data, c)
}

// Clone creates a deep copy of the configuration
func (c *MCPConfig) Clone() *MCPConfig {
	data, err := json.Marshal(c)
	if err != nil {
		return nil
	}

	var clone MCPConfig
	if err := json.Unmarshal(data, &clone); err != nil {
		return nil
	}

	return &clone
}

// Common MCP server configurations
var (
	// Common MCP server configurations that can be easily added
	CommonServerConfigs = map[string]*ServerConfig{
		"filesystem": {
			ID:          "filesystem",
			Name:        "Filesystem Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-filesystem",
			Args:        []string{"/tmp", "/var/tmp"},
			Env:         make(map[string]string),
			AutoStart:   true,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     true,
		},
		"memory": {
			ID:          "memory",
			Name:        "Memory Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-memory",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   true,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     true,
		},
		"github": {
			ID:      "github",
			Name:    "GitHub Server",
			Type:    SpawnerTypeNPX,
			Command: "@modelcontextprotocol/server-github",
			Args:    []string{},
			Env: map[string]string{
				"GITHUB_PERSONAL_ACCESS_TOKEN": "your-token-here",
			},
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"sqlite": {
			ID:          "sqlite",
			Name:        "SQLite Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-sqlite",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"postgres": {
			ID:      "postgres",
			Name:    "PostgreSQL Server",
			Type:    SpawnerTypeNPX,
			Command: "@modelcontextprotocol/server-postgres",
			Args:    []string{},
			Env: map[string]string{
				"POSTGRES_CONNECTION_STRING": "postgresql://user:pass@localhost:5432/db",
			},
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"brave": {
			ID:      "brave",
			Name:    "Brave Search Server",
			Type:    SpawnerTypeNPX,
			Command: "@modelcontextprotocol/server-brave-search",
			Args:    []string{},
			Env: map[string]string{
				"BRAVE_API_KEY": "your-api-key-here",
			},
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"youtube": {
			ID:          "youtube",
			Name:        "YouTube Transcript Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-youtube-transcript",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"puppeteer": {
			ID:          "puppeteer",
			Name:        "Puppeteer Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-puppeteer",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"docker": {
			ID:          "docker",
			Name:        "Docker Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-docker",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"kubernetes": {
			ID:          "kubernetes",
			Name:        "Kubernetes Server",
			Type:        SpawnerTypeNPX,
			Command:     "@modelcontextprotocol/server-kubernetes",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     false,
		},
		"chrome-devtools": {
			ID:          "chrome-devtools",
			Name:        "Chrome DevTools Server",
			Type:        SpawnerTypeNPX,
			Command:     "chrome-devtools-mcp@latest",
			Args:        []string{},
			Env:         make(map[string]string),
			AutoStart:   false,
			AutoRestart: true,
			Timeout:     30 * time.Second,
			Enabled:     true,
		},
	}
)

// AddCommonServerConfig adds a common server configuration
func (c *MCPConfig) AddCommonServerConfig(name string) error {
	config, exists := CommonServerConfigs[name]
	if !exists {
		return fmt.Errorf("unknown common server config: %s", name)
	}

	// Clone the config to avoid modifying the original
	clone := *config
	return c.AddServerConfig(&clone)
}

// ListCommonServerConfigs returns the names of available common server configurations
func ListCommonServerConfigs() []string {
	names := make([]string, 0, len(CommonServerConfigs))
	for name := range CommonServerConfigs {
		names = append(names, name)
	}
	return names
}
