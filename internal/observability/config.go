package observability

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config represents the complete observability configuration
type Config struct {
	Logging LoggingConfig `yaml:"logging"`
	Metrics MetricsConfig `yaml:"metrics"`
	Tracing TracingConfig `yaml:"tracing"`
}

// LoggingConfig configures logging
type LoggingConfig struct {
	Level  string `yaml:"level"`  // debug, info, warn, error
	Format string `yaml:"format"` // json, text
}

// DefaultConfig returns the default observability configuration
func DefaultConfig() Config {
	return Config{
		Logging: LoggingConfig{
			Level:  "info",
			Format: "json",
		},
		Metrics: MetricsConfig{
			Enabled:        true,
			PrometheusPort: 9090,
		},
		Tracing: TracingConfig{
			Enabled:        false,
			Exporter:       "otlp",
			OTLPEndpoint:   "localhost:4318",
			SampleRate:     1.0,
			ServiceName:    "alex",
			ServiceVersion: "1.0.0",
		},
	}
}

// LoadConfig loads observability configuration from file
func LoadConfig(configPath string) (Config, error) {
	// Start with defaults
	config := DefaultConfig()

	// If config path is empty, use default location
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err == nil {
			configPath = filepath.Join(homeDir, ".alex", "config.yaml")
		}
	}

	// If file doesn't exist, return defaults
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		return config, nil
	}

	// Read file
	data, err := os.ReadFile(configPath)
	if err != nil {
		return config, fmt.Errorf("failed to read config file: %w", err)
	}

	// Parse YAML
	var fileConfig struct {
		Observability Config `yaml:"observability"`
	}

	if err := yaml.Unmarshal(data, &fileConfig); err != nil {
		return config, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Debug: log what was parsed
	// fmt.Printf("DEBUG: Parsed config: %+v\n", fileConfig.Observability)

	// Merge with defaults (only override non-zero values)
	if fileConfig.Observability.Logging.Level != "" {
		config.Logging.Level = fileConfig.Observability.Logging.Level
	}
	if fileConfig.Observability.Logging.Format != "" {
		config.Logging.Format = fileConfig.Observability.Logging.Format
	}

	// Metrics config
	config.Metrics.Enabled = fileConfig.Observability.Metrics.Enabled
	if fileConfig.Observability.Metrics.PrometheusPort > 0 {
		config.Metrics.PrometheusPort = fileConfig.Observability.Metrics.PrometheusPort
	}

	// Tracing config - always override the Enabled flag from file
	config.Tracing.Enabled = fileConfig.Observability.Tracing.Enabled
	if fileConfig.Observability.Tracing.Exporter != "" {
		config.Tracing.Exporter = fileConfig.Observability.Tracing.Exporter
	}
	if fileConfig.Observability.Tracing.OTLPEndpoint != "" {
		config.Tracing.OTLPEndpoint = fileConfig.Observability.Tracing.OTLPEndpoint
	}
	if fileConfig.Observability.Tracing.ZipkinEndpoint != "" {
		config.Tracing.ZipkinEndpoint = fileConfig.Observability.Tracing.ZipkinEndpoint
	}
	// Sample rate: only override if explicitly set (> 0)
	// Note: This means you can't set sample_rate to exactly 0.0 via config file
	// If you need 0.0, set tracing.enabled to false instead
	if fileConfig.Observability.Tracing.SampleRate > 0 && fileConfig.Observability.Tracing.SampleRate <= 1.0 {
		config.Tracing.SampleRate = fileConfig.Observability.Tracing.SampleRate
	}
	if fileConfig.Observability.Tracing.ServiceName != "" {
		config.Tracing.ServiceName = fileConfig.Observability.Tracing.ServiceName
	}
	if fileConfig.Observability.Tracing.ServiceVersion != "" {
		config.Tracing.ServiceVersion = fileConfig.Observability.Tracing.ServiceVersion
	}

	return config, nil
}

// SaveConfig saves observability configuration to file
func SaveConfig(config Config, configPath string) error {
	if configPath == "" {
		homeDir, err := os.UserHomeDir()
		if err != nil {
			return fmt.Errorf("failed to get home directory: %w", err)
		}
		configPath = filepath.Join(homeDir, ".alex", "config.yaml")
	}

	// Create directory if needed
	dir := filepath.Dir(configPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	// Marshal to YAML
	data := struct {
		Observability Config `yaml:"observability"`
	}{
		Observability: config,
	}

	yamlData, err := yaml.Marshal(data)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	// Write file
	if err := os.WriteFile(configPath, yamlData, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}
