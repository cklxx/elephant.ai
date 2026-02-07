package bootstrap

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// EvalServerConfig holds the configuration for the evaluation server.
type EvalServerConfig struct {
	Port           string   `yaml:"port"`
	Environment    string   `yaml:"environment"`
	AllowedOrigins []string `yaml:"allowed_origins"`
	EvalOutputDir  string   `yaml:"eval_output_dir"`
	RLOutputDir    string   `yaml:"rl_output_dir"`
	SessionDir     string   `yaml:"session_dir"`
}

// DefaultConfig returns a config with sensible defaults.
func DefaultConfig() *EvalServerConfig {
	return &EvalServerConfig{
		Port:           "8081",
		Environment:    "evaluation",
		AllowedOrigins: []string{"http://localhost:3001"},
		EvalOutputDir:  "./evaluation_results",
		RLOutputDir:    "./rl_data",
		SessionDir:     "./.sessions",
	}
}

// LoadConfig reads the YAML config file and returns an EvalServerConfig.
func LoadConfig(path string) (*EvalServerConfig, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read config: %w", err)
	}
	cfg := DefaultConfig()
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("decode config: %w", err)
	}
	if cfg.Port == "" {
		cfg.Port = "8081"
	}
	return cfg, nil
}
