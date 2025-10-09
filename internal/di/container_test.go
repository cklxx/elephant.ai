package di

import (
	"os"
	"testing"
	"time"
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
			Environment:   "development",
			Verbose:       false,
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

	t.Run("exposes MCP initialization status", func(t *testing.T) {
		config := Config{
			LLMProvider:   "ollama",
			LLMModel:      "llama2",
			MaxTokens:     100000,
			MaxIterations: 20,
			SessionDir:    "/tmp/alex-test-env-sessions",
			CostDir:       "/tmp/alex-test-env-costs",
			Environment:   "development",
			Verbose:       false,
		}

		container, err := BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer() error = %v", err)
		}
		defer func() { _ = container.Cleanup() }()

		deadline := time.Now().Add(2 * time.Second)
		for {
			status := container.MCPInitializationStatus()
			if status.Attempts > 0 {
				// We don't expect readiness in tests but attempts should be recorded
				break
			}
			if time.Now().After(deadline) {
				t.Fatal("expected MCP initialization attempts to be recorded")
			}
			time.Sleep(10 * time.Millisecond)
		}
	})
}
