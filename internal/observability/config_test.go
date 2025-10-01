package observability

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestDefaultConfig(t *testing.T) {
	config := DefaultConfig()

	assert.Equal(t, "info", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format)
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, 9090, config.Metrics.PrometheusPort)
	assert.False(t, config.Tracing.Enabled)
	assert.Equal(t, "jaeger", config.Tracing.Exporter)
	assert.Equal(t, 1.0, config.Tracing.SampleRate)
}

func TestLoadConfig_NonExistent(t *testing.T) {
	// Should return defaults when file doesn't exist
	config, err := LoadConfig("/nonexistent/path/config.yaml")
	require.NoError(t, err)

	// Should have default values
	assert.Equal(t, "info", config.Logging.Level)
}

func TestLoadConfig_ValidFile(t *testing.T) {
	// Create temporary config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
observability:
  logging:
    level: debug
    format: text
  metrics:
    enabled: true
    prometheus_port: 8080
  tracing:
    enabled: true
    exporter: otlp
    sample_rate: 0.5
    service_name: alex-test
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, "debug", config.Logging.Level)
	assert.Equal(t, "text", config.Logging.Format)
	assert.True(t, config.Metrics.Enabled)
	assert.Equal(t, 8080, config.Metrics.PrometheusPort)
	assert.True(t, config.Tracing.Enabled)
	assert.Equal(t, "otlp", config.Tracing.Exporter)
	assert.Equal(t, 0.5, config.Tracing.SampleRate)
	assert.Equal(t, "alex-test", config.Tracing.ServiceName)
}

func TestLoadConfig_PartialFile(t *testing.T) {
	// Create temporary config file with partial settings
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	configContent := `
observability:
  logging:
    level: warn
  metrics:
    enabled: false
`
	err := os.WriteFile(configPath, []byte(configContent), 0644)
	require.NoError(t, err)

	// Load config
	config, err := LoadConfig(configPath)
	require.NoError(t, err)

	// Should merge with defaults
	assert.Equal(t, "warn", config.Logging.Level)
	assert.Equal(t, "json", config.Logging.Format) // Default
	assert.False(t, config.Metrics.Enabled)
	assert.Equal(t, 9090, config.Metrics.PrometheusPort) // Default
}

func TestSaveConfig(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	config := Config{
		Logging: LoggingConfig{
			Level:  "debug",
			Format: "text",
		},
		Metrics: MetricsConfig{
			Enabled:        true,
			PrometheusPort: 8080,
		},
		Tracing: TracingConfig{
			Enabled:        true,
			Exporter:       "jaeger",
			JaegerEndpoint: "http://localhost:14268/api/traces",
			SampleRate:     0.8,
			ServiceName:    "alex",
			ServiceVersion: "1.0.0",
		},
	}

	// Save config
	err := SaveConfig(config, configPath)
	require.NoError(t, err)

	// Verify file exists
	_, err = os.Stat(configPath)
	require.NoError(t, err)

	// Load and verify
	loadedConfig, err := LoadConfig(configPath)
	require.NoError(t, err)

	assert.Equal(t, config.Logging.Level, loadedConfig.Logging.Level)
	assert.Equal(t, config.Metrics.PrometheusPort, loadedConfig.Metrics.PrometheusPort)
	assert.Equal(t, config.Tracing.SampleRate, loadedConfig.Tracing.SampleRate)
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")

	// Write invalid YAML
	err := os.WriteFile(configPath, []byte("invalid: yaml: content: ["), 0644)
	require.NoError(t, err)

	// Should return error
	_, err = LoadConfig(configPath)
	assert.Error(t, err)
}

func TestSaveConfig_CreateDirectory(t *testing.T) {
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "subdir", "config.yaml")

	config := DefaultConfig()

	// Should create directory
	err := SaveConfig(config, configPath)
	require.NoError(t, err)

	// Verify directory and file exist
	_, err = os.Stat(filepath.Dir(configPath))
	require.NoError(t, err)
	_, err = os.Stat(configPath)
	require.NoError(t, err)
}
