package config

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"alex/internal/llm"
)

// LayeredConfigManager implements the configuration layering strategy
// from the pragmatic optimization guide
type LayeredConfigManager struct {
	core     *CoreConfig
	project  *ProjectConfig  
	advanced *AdvancedConfig
	merged   *Config // The final merged configuration
}

// CoreConfig represents essential configuration (~/.alex-config.json)
// This is what 80% of users need to configure
type CoreConfig struct {
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url,omitempty"`
}

// ProjectConfig represents project-specific configuration (./alex.yaml)
// Optional configuration that varies by project
type ProjectConfig struct {
	Models ProjectModels `json:"models,omitempty"`
	Tools  ProjectTools  `json:"tools,omitempty"`
	Agent  AgentConfig   `json:"agent,omitempty"`
}

// ProjectModels defines model preferences for the project
type ProjectModels struct {
	Basic     string `json:"basic,omitempty"`
	Reasoning string `json:"reasoning,omitempty"`
}

// ProjectTools defines tool-specific configuration
type ProjectTools struct {
	SearchAPIKey string   `json:"search_api_key,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	MaxConcurrent int     `json:"max_concurrent,omitempty"`
}

// AgentConfig defines agent behavior for the project  
type AgentConfig struct {
	MaxTurns    int     `json:"max_turns,omitempty"`
	Temperature float64 `json:"temperature,omitempty"`
	MaxTokens   int     `json:"max_tokens,omitempty"`
}

// AdvancedConfig represents expert-level configuration (./alex-advanced.yaml)
// Only for users who need enterprise features
type AdvancedConfig struct {
	MCP      *MCPConfig      `json:"mcp,omitempty"`
	Security *SecurityConfig `json:"security,omitempty"`
	Logging  *LoggingConfig  `json:"logging,omitempty"`
}

// NewLayeredConfigManager creates a new layered configuration manager
func NewLayeredConfigManager() (*LayeredConfigManager, error) {
	lcm := &LayeredConfigManager{}
	
	if err := lcm.loadAllLayers(); err != nil {
		return nil, fmt.Errorf("failed to load configuration layers: %w", err)
	}
	
	lcm.merged = lcm.merge()
	return lcm, nil
}

// loadAllLayers loads configuration from all three layers
func (lcm *LayeredConfigManager) loadAllLayers() error {
	// Layer 1: Core configuration (required)
	if err := lcm.loadCoreConfig(); err != nil {
		return fmt.Errorf("failed to load core config: %w", err)
	}
	
	// Layer 2: Project configuration (optional)
	lcm.loadProjectConfig() // Don't fail if missing
	
	// Layer 3: Advanced configuration (optional)
	lcm.loadAdvancedConfig() // Don't fail if missing
	
	return nil
}

// loadCoreConfig loads the core configuration from ~/.alex-config.json
func (lcm *LayeredConfigManager) loadCoreConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	configPath := filepath.Join(homeDir, ".alex-config.json")
	
	// Check if config exists, if not create with defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return lcm.createDefaultCoreConfig(configPath)
	}
	
	data, err := os.ReadFile(configPath)
	if err != nil {
		return fmt.Errorf("failed to read core config: %w", err)
	}
	
	lcm.core = &CoreConfig{}
	if err := json.Unmarshal(data, lcm.core); err != nil {
		return fmt.Errorf("failed to parse core config: %w", err)
	}
	
	// Validate required fields
	if lcm.core.APIKey == "" {
		return fmt.Errorf("api_key is required in core configuration")
	}
	
	return nil
}

// createDefaultCoreConfig creates a default core configuration file
func (lcm *LayeredConfigManager) createDefaultCoreConfig(configPath string) error {
	defaultCore := &CoreConfig{
		APIKey:  "", // User must set this
		BaseURL: "https://openrouter.ai/api/v1", // Default to OpenRouter
	}
	
	data, err := json.MarshalIndent(defaultCore, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal default config: %w", err)
	}
	
	if err := os.WriteFile(configPath, data, 0600); err != nil {
		return fmt.Errorf("failed to write default config: %w", err)
	}
	
	return fmt.Errorf("created default config at %s - please set your API key", configPath)
}

// loadProjectConfig loads project-specific configuration from ./alex.yaml
func (lcm *LayeredConfigManager) loadProjectConfig() {
	// Try common project config file names
	projectFiles := []string{"alex.yaml", "alex.yml", ".alex.yaml", ".alex.yml"}
	
	for _, filename := range projectFiles {
		if data, err := os.ReadFile(filename); err == nil {
			lcm.project = &ProjectConfig{}
			// Try YAML first, fallback to JSON
			if err := lcm.parseYAMLOrJSON(data, lcm.project); err == nil {
				return // Successfully loaded
			}
		}
	}
	
	// No project config found - use defaults
	lcm.project = &ProjectConfig{}
}

// loadAdvancedConfig loads advanced configuration from ./alex-advanced.yaml
func (lcm *LayeredConfigManager) loadAdvancedConfig() {
	advancedFiles := []string{"alex-advanced.yaml", "alex-advanced.yml", ".alex-advanced.yaml"}
	
	for _, filename := range advancedFiles {
		if data, err := os.ReadFile(filename); err == nil {
			lcm.advanced = &AdvancedConfig{}
			if err := lcm.parseYAMLOrJSON(data, lcm.advanced); err == nil {
				return // Successfully loaded
			}
		}
	}
	
	// No advanced config found - use defaults
	lcm.advanced = &AdvancedConfig{}
}

// parseYAMLOrJSON attempts to parse data as YAML first, then JSON
func (lcm *LayeredConfigManager) parseYAMLOrJSON(data []byte, target interface{}) error {
	// For now, just use JSON parsing
	// TODO: Add YAML support when yaml package is available
	return json.Unmarshal(data, target)
}

// merge combines all configuration layers into a single Config
func (lcm *LayeredConfigManager) merge() *Config {
	merged := &Config{}
	
	// Start with defaults
	lcm.applyDefaults(merged)
	
	// Apply core configuration
	lcm.applyCoreConfig(merged)
	
	// Apply project configuration 
	lcm.applyProjectConfig(merged)
	
	// Apply advanced configuration
	lcm.applyAdvancedConfig(merged)
	
	return merged
}

// applyDefaults sets intelligent defaults
func (lcm *LayeredConfigManager) applyDefaults(config *Config) {
	// Default model configuration
	config.Model = "deepseek/deepseek-chat"
	config.Temperature = 0.7
	config.MaxTokens = 4000
	config.MaxTurns = 20
	config.BaseURL = "https://openrouter.ai/api/v1"
	
	// Default multi-model setup
	config.DefaultModelType = llm.BasicModel
	config.Models = map[llm.ModelType]*llm.ModelConfig{
		llm.BasicModel: {
			Model:       "deepseek/deepseek-chat",
			BaseURL:     "https://openrouter.ai/api/v1",
			Temperature: 0.7,
			MaxTokens:   4000,
		},
		llm.ReasoningModel: {
			Model:       "deepseek/deepseek-r1",
			BaseURL:     "https://openrouter.ai/api/v1", 
			Temperature: 0.3,
			MaxTokens:   8000,
		},
	}
}

// applyCoreConfig applies core configuration settings
func (lcm *LayeredConfigManager) applyCoreConfig(config *Config) {
	config.APIKey = lcm.core.APIKey
	
	if lcm.core.BaseURL != "" {
		config.BaseURL = lcm.core.BaseURL
		// Update all model configs with the base URL
		for _, modelConfig := range config.Models {
			modelConfig.BaseURL = lcm.core.BaseURL
			modelConfig.APIKey = lcm.core.APIKey
		}
	}
	
	// Set API key for all models
	for _, modelConfig := range config.Models {
		modelConfig.APIKey = lcm.core.APIKey
	}
}

// applyProjectConfig applies project-specific configuration
func (lcm *LayeredConfigManager) applyProjectConfig(config *Config) {
	if lcm.project == nil {
		return
	}
	
	// Apply model preferences
	if lcm.project.Models.Basic != "" {
		if config.Models[llm.BasicModel] != nil {
			config.Models[llm.BasicModel].Model = lcm.project.Models.Basic
		}
	}
	
	if lcm.project.Models.Reasoning != "" {
		if config.Models[llm.ReasoningModel] != nil {
			config.Models[llm.ReasoningModel].Model = lcm.project.Models.Reasoning
		}
	}
	
	// Apply agent configuration
	if lcm.project.Agent.MaxTurns > 0 {
		config.MaxTurns = lcm.project.Agent.MaxTurns
	}
	
	if lcm.project.Agent.Temperature > 0 {
		config.Temperature = lcm.project.Agent.Temperature
		// Update all model configs
		for _, modelConfig := range config.Models {
			modelConfig.Temperature = lcm.project.Agent.Temperature
		}
	}
	
	if lcm.project.Agent.MaxTokens > 0 {
		config.MaxTokens = lcm.project.Agent.MaxTokens
		// Update all model configs
		for _, modelConfig := range config.Models {
			modelConfig.MaxTokens = lcm.project.Agent.MaxTokens
		}
	}
	
	// Apply tool configuration
	if lcm.project.Tools.SearchAPIKey != "" {
		config.TavilyAPIKey = lcm.project.Tools.SearchAPIKey
	}
}

// applyAdvancedConfig applies advanced configuration for expert users
func (lcm *LayeredConfigManager) applyAdvancedConfig(config *Config) {
	if lcm.advanced == nil {
		return
	}
	
	// Apply MCP configuration
	if lcm.advanced.MCP != nil {
		config.MCP = lcm.advanced.MCP
	}
	
	// Apply security configuration - this would be in the legacy Config
	// For now, we don't have a direct mapping
}

// GetConfig returns the merged configuration
func (lcm *LayeredConfigManager) GetConfig() *Config {
	return lcm.merged
}

// GetConfigValue returns a configuration value by key path (for compatibility)
func (lcm *LayeredConfigManager) GetConfigValue(key string) (interface{}, error) {
	switch key {
	case "api_key":
		return lcm.merged.APIKey, nil
	case "base_url":
		return lcm.merged.BaseURL, nil
	case "model":
		return lcm.merged.Model, nil
	case "temperature":
		return lcm.merged.Temperature, nil
	case "max_tokens":
		return lcm.merged.MaxTokens, nil
	case "max_turns":
		return lcm.merged.MaxTurns, nil
	case "default_model_type":
		return lcm.merged.DefaultModelType, nil
	case "models":
		return lcm.merged.Models, nil
	default:
		return nil, fmt.Errorf("unknown configuration key: %s", key)
	}
}

// SetConfigValue sets a configuration value (for compatibility)
func (lcm *LayeredConfigManager) SetConfigValue(key string, value interface{}) error {
	// For now, only allow setting core configuration values
	// More sophisticated layered setting would require determining which layer to update
	switch key {
	case "api_key":
		if str, ok := value.(string); ok {
			lcm.core.APIKey = str
			lcm.merged = lcm.merge() // Re-merge
			return lcm.saveCoreConfig()
		}
	case "base_url":
		if str, ok := value.(string); ok {
			lcm.core.BaseURL = str
			lcm.merged = lcm.merge() // Re-merge
			return lcm.saveCoreConfig()
		}
	}
	
	return fmt.Errorf("cannot set configuration key %s or invalid value type", key)
}

// saveCoreConfig saves the core configuration back to disk
func (lcm *LayeredConfigManager) saveCoreConfig() error {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("failed to get home directory: %w", err)
	}
	
	configPath := filepath.Join(homeDir, ".alex-config.json")
	
	data, err := json.MarshalIndent(lcm.core, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal core config: %w", err)
	}
	
	return os.WriteFile(configPath, data, 0600)
}

// GetLayerInfo returns information about which layer each setting comes from
func (lcm *LayeredConfigManager) GetLayerInfo() map[string]string {
	info := make(map[string]string)
	
	info["api_key"] = "core"
	info["base_url"] = "core"
	
	if lcm.project.Agent.Temperature > 0 {
		info["temperature"] = "project"
	} else {
		info["temperature"] = "default"
	}
	
	if lcm.project.Models.Basic != "" {
		info["basic_model"] = "project"
	} else {
		info["basic_model"] = "default"
	}
	
	return info
}