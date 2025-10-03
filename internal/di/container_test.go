package di

import (
	"os"
	"testing"
)

func TestResolveStorageDir(t *testing.T) {
	// Get actual home directory for testing
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	tests := []struct {
		name       string
		configured string
		defaultVal string
		want       string
		wantPrefix string // For flexible matching
	}{
		{
			name:       "use configured absolute path when provided",
			configured: "/custom/path",
			defaultVal: "~/.alex-sessions",
			want:       "/custom/path",
		},
		{
			name:       "use default when configured is empty",
			configured: "",
			defaultVal: "~/.alex-sessions",
			wantPrefix: home, // Should expand to home directory
		},
		{
			name:       "handle absolute paths",
			configured: "/var/lib/alex",
			defaultVal: "~/.alex",
			want:       "/var/lib/alex",
		},
		{
			name:       "expand tilde with slash",
			configured: "~/.alex-sessions",
			defaultVal: "",
			want:       home + "/.alex-sessions",
		},
		{
			name:       "expand tilde without slash",
			configured: "~",
			defaultVal: "",
			want:       home,
		},
		{
			name:       "expand tilde with path no slash",
			configured: "~.alex-sessions",
			defaultVal: "",
			want:       home + "/.alex-sessions",
		},
		{
			name:       "handle $HOME environment variable",
			configured: "$HOME/.alex-costs",
			defaultVal: "",
			want:       home + "/.alex-costs",
		},
		{
			name:       "handle empty string edge case",
			configured: "",
			defaultVal: "",
			want:       "",
		},
		{
			name:       "handle relative path without tilde",
			configured: "relative/path",
			defaultVal: "",
			want:       "relative/path",
		},
		{
			name:       "expand complex path with env vars",
			configured: "$HOME/projects/alex/.sessions",
			defaultVal: "",
			want:       home + "/projects/alex/.sessions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := resolveStorageDir(tt.configured, tt.defaultVal)

			if tt.wantPrefix != "" {
				// Check if result starts with the expected prefix (for home dir tests)
				if len(result) < len(tt.wantPrefix) || result[:len(tt.wantPrefix)] != tt.wantPrefix {
					t.Errorf("resolveStorageDir() = %v, want prefix %v", result, tt.wantPrefix)
				}
			} else if tt.want != "" {
				if result != tt.want {
					t.Errorf("resolveStorageDir() = %v, want %v", result, tt.want)
				}
			}

			// Additional validation: ensure no path starts with /. (invalid)
			if len(result) > 1 && result[0] == '/' && result[1] == '.' && tt.want != "" {
				t.Errorf("resolveStorageDir() produced invalid path starting with '/.' = %v", result)
			}
		})
	}
}

func TestResolveStorageDir_DoesNotStripHomeIncorrectly(t *testing.T) {
	// Regression test for P0 blocker: "Storage path expansion strips $HOME incorrectly"
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("Failed to get home directory: %v", err)
	}

	testCases := []struct {
		name  string
		input string
	}{
		{"tilde with slash", "~/.alex-sessions"},
		{"tilde alone", "~"},
		{"HOME env var", "$HOME/.alex-sessions"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			result := resolveStorageDir(tc.input, "")

			// Should never produce /.alex-* paths
			if len(result) > 1 && result[0] == '/' && result[1] == '.' {
				t.Errorf("resolveStorageDir(%q) incorrectly produced path starting with '/.' = %v", tc.input, result)
			}

			// Should always start with home directory
			if len(result) < len(home) || result[:len(home)] != home {
				t.Errorf("resolveStorageDir(%q) = %v, expected to start with home directory %v", tc.input, result, home)
			}

			// Should not contain unexpanded ~ or $HOME
			if len(result) > 0 && result[0] == '~' {
				t.Errorf("resolveStorageDir(%q) failed to expand tilde = %v", tc.input, result)
			}
		})
	}
}

func TestGetAPIKey(t *testing.T) {
	tests := []struct {
		name     string
		provider string
		envVars  map[string]string
		want     string
	}{
		{
			name:     "openrouter uses OPENROUTER_API_KEY",
			provider: "openrouter",
			envVars: map[string]string{
				"OPENROUTER_API_KEY": "sk-or-test",
				"OPENAI_API_KEY":     "sk-openai-test",
			},
			want: "sk-or-test",
		},
		{
			name:     "deepseek uses DEEPSEEK_API_KEY",
			provider: "deepseek",
			envVars: map[string]string{
				"DEEPSEEK_API_KEY": "sk-ds-test",
				"OPENAI_API_KEY":   "sk-openai-test",
			},
			want: "sk-ds-test",
		},
		{
			name:     "openai uses OPENAI_API_KEY",
			provider: "openai",
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-openai-test",
			},
			want: "sk-openai-test",
		},
		{
			name:     "fallback to OPENAI_API_KEY when provider key not found",
			provider: "openrouter",
			envVars: map[string]string{
				"OPENAI_API_KEY": "sk-openai-test",
			},
			want: "sk-openai-test",
		},
		{
			name:     "ollama returns empty string",
			provider: "ollama",
			envVars:  map[string]string{},
			want:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear environment
			os.Clearenv()

			// Set test environment variables
			for key, val := range tt.envVars {
				_ = os.Setenv(key, val)
			}
			defer os.Clearenv()

			got := GetAPIKey(tt.provider)
			if got != tt.want {
				t.Errorf("GetAPIKey(%s) = %v, want %v", tt.provider, got, tt.want)
			}
		})
	}
}

func TestGetStorageDir(t *testing.T) {
	tests := []struct {
		name       string
		envVar     string
		envValue   string
		defaultVal string
		want       string
	}{
		{
			name:       "use environment variable when set",
			envVar:     "ALEX_SESSION_DIR",
			envValue:   "/custom/sessions",
			defaultVal: "~/.alex-sessions",
			want:       "/custom/sessions",
		},
		{
			name:       "use default when env var not set",
			envVar:     "ALEX_SESSION_DIR",
			envValue:   "",
			defaultVal: "~/.alex-sessions",
			want:       "~/.alex-sessions",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clear and set environment
			os.Clearenv()
			if tt.envValue != "" {
				_ = os.Setenv(tt.envVar, tt.envValue)
			}
			defer os.Clearenv()

			got := GetStorageDir(tt.envVar, tt.defaultVal)
			if got != tt.want {
				t.Errorf("GetStorageDir() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildContainer(t *testing.T) {
	// This is an integration test to verify the container can be built
	t.Run("builds successfully with valid config", func(t *testing.T) {
		// Use Ollama which doesn't require API key
		config := Config{
			LLMProvider:   "ollama",
			LLMModel:      "llama2",
			APIKey:        "",
			BaseURL:       "",
			MaxTokens:     100000,
			MaxIterations: 20,
			SessionDir:    "/tmp/alex-test-sessions",
			CostDir:       "/tmp/alex-test-costs",
		}

		container, err := BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer() error = %v", err)
		}

		if container == nil {
			t.Fatal("BuildContainer() returned nil container")
		}

		if container.AgentCoordinator == nil {
			t.Error("AgentCoordinator is nil")
		}

		if container.SessionStore == nil {
			t.Error("SessionStore is nil")
		}

		if container.CostTracker == nil {
			t.Error("CostTracker is nil")
		}

		if container.MCPRegistry == nil {
			t.Error("MCPRegistry is nil")
		}

		// Cleanup
		if err := container.Cleanup(); err != nil {
			t.Errorf("Cleanup() error = %v", err)
		}
	})

	t.Run("uses environment variables for storage paths", func(t *testing.T) {
		// Set environment variables
		_ = os.Setenv("ALEX_SESSION_DIR", "/tmp/alex-test-env-sessions")
		_ = os.Setenv("ALEX_COST_DIR", "/tmp/alex-test-env-costs")
		defer func() {
			_ = os.Unsetenv("ALEX_SESSION_DIR")
			_ = os.Unsetenv("ALEX_COST_DIR")
		}()

		config := Config{
			LLMProvider:   "ollama",
			LLMModel:      "llama2",
			MaxTokens:     100000,
			MaxIterations: 20,
			SessionDir:    GetStorageDir("ALEX_SESSION_DIR", "~/.alex-sessions"),
			CostDir:       GetStorageDir("ALEX_COST_DIR", "~/.alex-costs"),
		}

		container, err := BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer() error = %v", err)
		}
		defer func() {
			_ = container.Cleanup()
		}()

		if container == nil {
			t.Fatal("BuildContainer() returned nil container")
		}

		// Verify container was built successfully
		if container.AgentCoordinator == nil {
			t.Error("AgentCoordinator is nil")
		}
	})
}
