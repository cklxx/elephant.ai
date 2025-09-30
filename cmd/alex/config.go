package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	"alex/internal/llm"
)

type AppConfig struct {
	LLMProvider   string                 `json:"llm_provider"`
	LLMModel      string                 `json:"llm_model"`
	Model         string                 `json:"model"` // Backward compatibility
	APIKey        string                 `json:"api_key"`
	BaseURL       string                 `json:"base_url"`
	MaxIterations int                    `json:"max_iterations"`
	MaxTokens     int                    `json:"max_tokens"`
	Models        map[string]ModelConfig `json:"models"` // Support old format
}

type ModelConfig struct {
	Model   string `json:"model"`
	APIKey  string `json:"api_key"`
	BaseURL string `json:"base_url"`
}

func loadConfig() AppConfig {
	// Default configuration
	config := AppConfig{
		LLMProvider:   "openrouter",
		LLMModel:      "deepseek/deepseek-chat",
		BaseURL:       "https://openrouter.ai/api/v1",
		MaxIterations: 150,
		MaxTokens:     100000,
	}

	// Try to load from environment
	if apiKey := os.Getenv("OPENAI_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	}
	if apiKey := os.Getenv("OPENROUTER_API_KEY"); apiKey != "" {
		config.APIKey = apiKey
	}
	if provider := os.Getenv("LLM_PROVIDER"); provider != "" {
		config.LLMProvider = provider
	}
	if model := os.Getenv("LLM_MODEL"); model != "" {
		config.LLMModel = model
	}

	// Try to load from config file
	home, err := os.UserHomeDir()
	if err == nil {
		configPath := filepath.Join(home, ".alex-config.json")
		if data, err := os.ReadFile(configPath); err == nil {
			var fileConfig AppConfig
			if err := json.Unmarshal(data, &fileConfig); err == nil {
				// Merge file config (file overrides defaults but not env vars)
				if fileConfig.APIKey != "" && config.APIKey == "" {
					config.APIKey = fileConfig.APIKey
				}
				if fileConfig.LLMProvider != "" {
					config.LLMProvider = fileConfig.LLMProvider
				}

				// Support both new and old config formats
				if fileConfig.LLMModel != "" {
					config.LLMModel = fileConfig.LLMModel
				} else if fileConfig.Model != "" {
					config.LLMModel = fileConfig.Model
				}

				if fileConfig.BaseURL != "" {
					config.BaseURL = fileConfig.BaseURL
				}
				if fileConfig.MaxIterations > 0 {
					config.MaxIterations = fileConfig.MaxIterations
				}
				if fileConfig.MaxTokens > 0 {
					config.MaxTokens = fileConfig.MaxTokens
				}

				// Support old "models.basic" format
				if fileConfig.Models != nil {
					if basicModel, ok := fileConfig.Models["basic"]; ok {
						if config.APIKey == "" && basicModel.APIKey != "" {
							config.APIKey = basicModel.APIKey
						}
						if basicModel.Model != "" {
							config.LLMModel = basicModel.Model
						}
						if basicModel.BaseURL != "" {
							config.BaseURL = basicModel.BaseURL
						}
					}
				}
			}
		}
	}

	// If no API key and not mock mode, use mock
	if config.APIKey == "" && config.LLMProvider != "mock" {
		config.LLMProvider = "mock"
	}

	return config
}

func (c AppConfig) toLLMConfig() llm.Config {
	return llm.Config{
		APIKey:  c.APIKey,
		BaseURL: c.BaseURL,
	}
}
