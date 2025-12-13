package swe_bench

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"

	runtimeconfig "alex/internal/config"
	"gopkg.in/yaml.v3"
)

// ConfigManager handles batch configuration management
type ConfigManager struct {
	defaultConfig *BatchConfig
}

// NewConfigManager creates a new configuration manager
func NewConfigManager() *ConfigManager {
	return &ConfigManager{
		defaultConfig: DefaultBatchConfig(),
	}
}

// LoadConfig loads configuration from a YAML file
func (cm *ConfigManager) LoadConfig(path string) (*BatchConfig, error) {
	// Start with default configuration
	config := *cm.defaultConfig

	// Read configuration file if provided
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("failed to read config file %s: %w", path, err)
		}

		if err := yaml.Unmarshal(data, &config); err != nil {
			return nil, fmt.Errorf("failed to parse config file %s: %w", path, err)
		}
	}

	// Apply environment variable overrides
	if err := cm.applyEnvOverrides(&config); err != nil {
		return nil, fmt.Errorf("failed to apply environment overrides: %w", err)
	}

	// Validate configuration
	if err := cm.ValidateConfig(&config); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	return &config, nil
}

// SaveConfig saves configuration to a YAML file
func (cm *ConfigManager) SaveConfig(config *BatchConfig, path string) error {
	// Create directory if it doesn't exist
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create directory %s: %w", dir, err)
	}

	// Marshal configuration to YAML
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write to file
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", path, err)
	}

	return nil
}

// ValidateConfig validates a batch configuration
func (cm *ConfigManager) ValidateConfig(config *BatchConfig) error {
	if config == nil {
		return fmt.Errorf("config cannot be nil")
	}

	// Validate agent configuration
	if config.Agent.Model.Name == "" {
		return fmt.Errorf("agent.model.name is required")
	}

	if config.Agent.Model.Temperature < 0 || config.Agent.Model.Temperature > 2 {
		return fmt.Errorf("agent.model.temperature must be between 0 and 2")
	}

	if config.Agent.MaxTurns <= 0 {
		config.Agent.MaxTurns = 20 // Default value
	}

	if config.Agent.Timeout <= 0 {
		config.Agent.Timeout = 300 // Default 5 minutes
	}

	// Validate dataset configuration
	if err := cm.validateDatasetConfig(&config.Instances); err != nil {
		return fmt.Errorf("invalid dataset config: %w", err)
	}

	// Validate execution configuration
	if config.NumWorkers <= 0 {
		config.NumWorkers = 1
	}

	if config.NumWorkers > 20 {
		return fmt.Errorf("num_workers cannot exceed 20")
	}

	if config.OutputPath == "" {
		config.OutputPath = "./batch_results"
	}

	cleanedOutput, err := sanitizeOutputPath(config.OutputPath)
	if err != nil {
		return err
	}
	config.OutputPath = cleanedOutput

	if config.MaxRetries < 0 {
		config.MaxRetries = 0
	}

	return nil
}

// validateDatasetConfig validates dataset configuration
func (cm *ConfigManager) validateDatasetConfig(config *DatasetConfig) error {
	if config.Type == "" {
		return fmt.Errorf("dataset type is required")
	}

	switch config.Type {
	case "swe_bench":
		if config.Subset == "" {
			config.Subset = "lite"
		}
		if config.Subset != "lite" && config.Subset != "full" && config.Subset != "verified" {
			return fmt.Errorf("invalid swe_bench subset: %s (must be 'lite', 'full', or 'verified')", config.Subset)
		}
		if config.Split == "" {
			config.Split = "dev"
		}
		if config.Split != "dev" && config.Split != "test" && config.Split != "train" {
			return fmt.Errorf("invalid split: %s (must be 'dev', 'test', or 'train')", config.Split)
		}

	case "file":
		if config.FilePath == "" {
			return fmt.Errorf("file_path is required for file-based datasets")
		}
		if _, err := os.Stat(config.FilePath); os.IsNotExist(err) {
			return fmt.Errorf("file does not exist: %s", config.FilePath)
		}

	case "huggingface":
		if config.HFDataset == "" {
			return fmt.Errorf("hf_dataset is required for Hugging Face datasets")
		}

	default:
		return fmt.Errorf("unsupported dataset type: %s", config.Type)
	}

	// Validate instance filtering
	if config.InstanceLimit < 0 {
		return fmt.Errorf("instance_limit cannot be negative")
	}

	if len(config.InstanceSlice) == 2 {
		if config.InstanceSlice[0] < 0 || config.InstanceSlice[1] < 0 {
			return fmt.Errorf("instance_slice values cannot be negative")
		}
		if config.InstanceSlice[0] >= config.InstanceSlice[1] {
			return fmt.Errorf("instance_slice start must be less than end")
		}
	} else if len(config.InstanceSlice) != 0 {
		return fmt.Errorf("instance_slice must contain exactly 2 values [start, end]")
	}

	return nil
}

// applyEnvOverrides applies environment variable overrides to configuration
func (cm *ConfigManager) applyEnvOverrides(config *BatchConfig) error {
	lookup := runtimeconfig.AliasEnvLookup(runtimeconfig.DefaultEnvLookup, sweBenchEnvAliases)

	runtimeCfg, meta, err := runtimeconfig.Load(
		runtimeconfig.WithEnv(lookup),
	)
	if err != nil {
		return fmt.Errorf("load runtime configuration: %w", err)
	}

	if meta.Source("llm_model") != runtimeconfig.SourceDefault && runtimeCfg.LLMModel != "" {
		config.Agent.Model.Name = runtimeCfg.LLMModel
	}
	if runtimeCfg.TemperatureProvided && meta.Source("temperature") != runtimeconfig.SourceDefault {
		config.Agent.Model.Temperature = runtimeCfg.Temperature
	}
	if meta.Source("max_tokens") != runtimeconfig.SourceDefault && runtimeCfg.MaxTokens != 0 {
		config.Agent.Model.MaxTokens = runtimeCfg.MaxTokens
	}

	// Execution configuration
	if workersStr, ok := lookup("NUM_WORKERS"); ok && workersStr != "" {
		workers, err := strconv.Atoi(workersStr)
		if err != nil {
			return fmt.Errorf("invalid NUM_WORKERS override %q: %w", workersStr, err)
		}
		config.NumWorkers = workers
	}

	if outputPath, ok := lookup("OUTPUT_PATH"); ok && outputPath != "" {
		config.OutputPath = outputPath
	}

	// Dataset configuration
	if datasetType, ok := lookup("DATASET_TYPE"); ok && datasetType != "" {
		config.Instances.Type = datasetType
	}

	if subset, ok := lookup("DATASET_SUBSET"); ok && subset != "" {
		config.Instances.Subset = subset
	}

	if split, ok := lookup("DATASET_SPLIT"); ok && split != "" {
		config.Instances.Split = split
	}

	return nil
}

// MergeConfigs merges two configurations, with override taking precedence
func (cm *ConfigManager) MergeConfigs(base *BatchConfig, override *BatchConfig) *BatchConfig {
	if base == nil {
		return override
	}
	if override == nil {
		return base
	}

	// Start with a copy of the base configuration
	result := *base

	// Override agent configuration
	if override.Agent.Model.Name != "" {
		result.Agent.Model.Name = override.Agent.Model.Name
	}
	if override.Agent.Model.Temperature != 0 {
		result.Agent.Model.Temperature = override.Agent.Model.Temperature
	}
	if override.Agent.Model.MaxTokens != 0 {
		result.Agent.Model.MaxTokens = override.Agent.Model.MaxTokens
	}
	if override.Agent.MaxTurns != 0 {
		result.Agent.MaxTurns = override.Agent.MaxTurns
	}
	if override.Agent.CostLimit != 0 {
		result.Agent.CostLimit = override.Agent.CostLimit
	}
	if override.Agent.Timeout != 0 {
		result.Agent.Timeout = override.Agent.Timeout
	}

	// Override dataset configuration
	if override.Instances.Type != "" {
		result.Instances.Type = override.Instances.Type
	}
	if override.Instances.Subset != "" {
		result.Instances.Subset = override.Instances.Subset
	}
	if override.Instances.Split != "" {
		result.Instances.Split = override.Instances.Split
	}
	if override.Instances.FilePath != "" {
		result.Instances.FilePath = override.Instances.FilePath
	}
	if override.Instances.HFDataset != "" {
		result.Instances.HFDataset = override.Instances.HFDataset
	}
	if override.Instances.InstanceLimit != 0 {
		result.Instances.InstanceLimit = override.Instances.InstanceLimit
	}
	if len(override.Instances.InstanceSlice) > 0 {
		result.Instances.InstanceSlice = override.Instances.InstanceSlice
	}
	if len(override.Instances.InstanceIDs) > 0 {
		result.Instances.InstanceIDs = override.Instances.InstanceIDs
	}
	if override.Instances.Shuffle {
		result.Instances.Shuffle = override.Instances.Shuffle
	}

	// Override execution configuration
	if override.NumWorkers != 0 {
		result.NumWorkers = override.NumWorkers
	}
	if override.OutputPath != "" {
		result.OutputPath = override.OutputPath
	}
	if override.ResumeFrom != "" {
		result.ResumeFrom = override.ResumeFrom
	}
	if override.EnableLogging {
		result.EnableLogging = override.EnableLogging
	}
	if override.MaxDelay != 0 {
		result.MaxDelay = override.MaxDelay
	}
	if override.FailFast {
		result.FailFast = override.FailFast
	}
	if override.MaxRetries != 0 {
		result.MaxRetries = override.MaxRetries
	}

	return &result
}

// GetDefaultConfigPath returns the default configuration file path
func (cm *ConfigManager) GetDefaultConfigPath() string {
	if homeDir, err := os.UserHomeDir(); err == nil {
		return filepath.Join(homeDir, ".alex", "batch_config.yaml")
	}
	return "batch_config.yaml"
}
