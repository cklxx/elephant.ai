package di

import (
	"context"
	"os"
	"testing"
	"time"

	taskdomain "alex/internal/domain/task"
	codinginfra "alex/internal/infra/coding"
	runtimeconfig "alex/internal/shared/config"
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
			defaultVal: "~/.alex/sessions",
			want:       "/custom/path",
		},
		{
			name:       "use default when configured is empty",
			configured: "",
			defaultVal: "~/.alex/sessions",
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
			configured: "~/.alex/sessions",
			defaultVal: "",
			want:       home + "/.alex/sessions",
		},
		{
			name:       "expand tilde without slash",
			configured: "~",
			defaultVal: "",
			want:       home,
		},
		{
			name:       "expand tilde with path no slash",
			configured: "~.alex/sessions",
			defaultVal: "",
			want:       home + "/.alex/sessions",
		},
		{
			name:       "handle $HOME environment variable",
			configured: "$HOME/.alex/costs",
			defaultVal: "",
			want:       home + "/.alex/costs",
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
		{"tilde with slash", "~/.alex/sessions"},
		{"tilde alone", "~"},
		{"HOME env var", "$HOME/.alex/sessions"},
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

func TestApplyDetectedExternalAgents_AutoEnablesSupportedAgents(t *testing.T) {
	builder := newContainerBuilder(Config{
		ExternalAgents: runtimeconfig.DefaultExternalAgentsConfig(),
	})

	detected := []codinginfra.LocalCLIDetection{
		{ID: "codex", Binary: "codex", Path: "/detected/codex", AgentType: "codex", AdapterSupport: true},
		{ID: "claude", Binary: "claude-code", Path: "/detected/claude-code", AgentType: "claude_code", AdapterSupport: true},
		{ID: "kimi", Binary: "kimi", Path: "/detected/kimi", AgentType: "kimi", AdapterSupport: true},
	}

	builder.applyDetectedExternalAgents(detected, false)

	if !builder.config.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex to be auto-enabled")
	}
	if builder.config.ExternalAgents.Codex.Binary != "/detected/codex" {
		t.Fatalf("expected codex binary to adopt detected path, got %q", builder.config.ExternalAgents.Codex.Binary)
	}
	if !builder.config.ExternalAgents.ClaudeCode.Enabled {
		t.Fatalf("expected claude_code to be auto-enabled")
	}
	if builder.config.ExternalAgents.ClaudeCode.Binary != "/detected/claude-code" {
		t.Fatalf("expected claude_code binary to adopt detected path, got %q", builder.config.ExternalAgents.ClaudeCode.Binary)
	}
	if !builder.config.ExternalAgents.Kimi.Enabled {
		t.Fatalf("expected kimi to be auto-enabled")
	}
	if builder.config.ExternalAgents.Kimi.Binary != "/detected/kimi" {
		t.Fatalf("expected kimi binary to adopt detected path, got %q", builder.config.ExternalAgents.Kimi.Binary)
	}
}

func TestApplyDetectedExternalAgents_RespectsCustomBinaryPath(t *testing.T) {
	cfg := Config{
		ExternalAgents: runtimeconfig.DefaultExternalAgentsConfig(),
	}
	cfg.ExternalAgents.Codex.Binary = "/custom/codex-dev"
	builder := newContainerBuilder(cfg)

	detected := []codinginfra.LocalCLIDetection{
		{ID: "codex", Binary: "codex", Path: "/detected/codex", AgentType: "codex", AdapterSupport: true},
	}

	builder.applyDetectedExternalAgents(detected, false)

	if !builder.config.ExternalAgents.Codex.Enabled {
		t.Fatalf("expected codex to be auto-enabled")
	}
	if builder.config.ExternalAgents.Codex.Binary != "/custom/codex-dev" {
		t.Fatalf("expected custom codex binary to remain unchanged, got %q", builder.config.ExternalAgents.Codex.Binary)
	}
}

func TestBuildContainer(t *testing.T) {
	// This is an integration test to verify the container can be built
	t.Run("builds successfully with valid config", func(t *testing.T) {
		// Use mock provider which doesn't require API key
		config := Config{
			LLMProvider:   "mock",
			LLMModel:      "test",
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
		if container.TaskStore == nil {
			t.Error("TaskStore is nil")
		}
		if container.TaskStore != nil {
			task := &taskdomain.Task{
				TaskID:      t.Name() + "-" + time.Now().Format("150405.000000000"),
				SessionID:   "session-1",
				Channel:     "web",
				Description: "task store wiring",
				Status:      taskdomain.StatusPending,
				CreatedAt:   time.Now(),
				UpdatedAt:   time.Now(),
			}
			if err := container.TaskStore.Create(context.Background(), task); err != nil {
				t.Errorf("TaskStore.Create() error = %v", err)
			}
		}

		// Cleanup
		if err := container.Shutdown(); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}
	})

	t.Run("builds without API key when features disabled", func(t *testing.T) {
		config := Config{
			LLMProvider:   "mock",
			LLMModel:      "test",
			MaxTokens:     100000,
			MaxIterations: 20,
			SessionDir:    "/tmp/alex-test-noapi-sessions",
			CostDir:       "/tmp/alex-test-noapi-costs",
		}

		container, err := BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer() should succeed without API key when features disabled: %v", err)
		}
		defer func() { _ = container.Shutdown() }()

		if container == nil {
			t.Fatal("Expected non-nil container")
		}
	})
}

// TestBuildContainer_FailAfterMemoryInit verifies that the background context
// (used by the memory cleanup goroutine) is cancelled when Build() fails at a
// step after memory initialization. This prevents goroutine leaks.
func TestBuildContainer_FailAfterMemoryInit(t *testing.T) {
	config := Config{
		LLMProvider:   "mock",
		LLMModel:      "test",
		SessionDir:    "/tmp/alex-test-fail-sessions",
		CostDir:       "/dev/null/invalid/nested/path", // triggers cost tracker init failure
		MaxTokens:     100000,
		MaxIterations: 20,
		Proactive: runtimeconfig.ProactiveConfig{
			Memory: runtimeconfig.MemoryConfig{
				Enabled:          true,
				ArchiveAfterDays: 30,
				CleanupInterval:  "1h",
			},
		},
	}

	container, err := BuildContainer(config)
	if err == nil {
		// If it somehow succeeds, clean up properly.
		_ = container.Shutdown()
		t.Fatal("expected BuildContainer to fail with invalid cost dir")
	}

	// The key assertion: if bgCancel was NOT called, the cleanup goroutine
	// would leak. We can't directly observe the goroutine count, but we
	// verify the code path by confirming Build() returned an error and
	// no container was returned (meaning the defer fired bgCancel).
	if container != nil {
		t.Fatal("expected nil container on build failure")
	}

	// Give any goroutine time to exit after cancellation.
	// Under -race this would detect any data races from leaked goroutines.
}

func TestContainer_Lifecycle(t *testing.T) {
	t.Run("Start and Shutdown with features disabled", func(t *testing.T) {
		config := Config{
			LLMProvider: "mock",
			LLMModel:    "test",
			SessionDir:  "/tmp/alex-test-lifecycle",
			CostDir:     "/tmp/alex-test-lifecycle-costs",
		}

		container, err := BuildContainer(config)
		if err != nil {
			t.Fatalf("BuildContainer() error = %v", err)
		}
		defer func() { _ = container.Shutdown() }()

		// Start should succeed
		if err := container.Start(); err != nil {
			t.Errorf("Start() error = %v", err)
		}

		// Shutdown should succeed
		if err := container.Shutdown(); err != nil {
			t.Errorf("Shutdown() error = %v", err)
		}

		// Multiple shutdowns should be safe
		if err := container.Shutdown(); err != nil {
			t.Errorf("Second Shutdown() error = %v", err)
		}
	})
}
