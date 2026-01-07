package di

import (
	"os"
	"testing"

	"alex/internal/session/postgresstore"
	sessionstate "alex/internal/session/state_store"
	"alex/internal/testutil"
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
			Environment:   "development",
			Verbose:       false,
			EnableMCP:     false, // Disable to avoid external dependencies
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
			EnableMCP:     false,
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

func TestContainer_Lifecycle(t *testing.T) {
	t.Run("Start and Shutdown with features disabled", func(t *testing.T) {
		config := Config{
			LLMProvider: "mock",
			LLMModel:    "test",
			SessionDir:  "/tmp/alex-test-lifecycle",
			CostDir:     "/tmp/alex-test-lifecycle-costs",
			EnableMCP:   false,
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

		// Verify MCP not started
		if container.mcpStarted {
			t.Error("Expected MCP to not be started when disabled")
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

func TestBuildContainer_RequireSessionDatabase(t *testing.T) {
	t.Run("fails when session database is required but missing", func(t *testing.T) {
		config := Config{
			LLMProvider:            "mock",
			LLMModel:               "test",
			MaxTokens:              100000,
			MaxIterations:          20,
			SessionDir:             "/tmp/alex-test-require-db-sessions",
			CostDir:                "/tmp/alex-test-require-db-costs",
			EnableMCP:              false,
			RequireSessionDatabase: true,
			SessionDatabaseURL:     "",
		}

		if _, err := BuildContainer(config); err == nil {
			t.Fatal("BuildContainer() should fail when session database is required but missing")
		}
	})

	t.Run("fails when session database URL is invalid", func(t *testing.T) {
		config := Config{
			LLMProvider:            "mock",
			LLMModel:               "test",
			MaxTokens:              100000,
			MaxIterations:          20,
			SessionDir:             "/tmp/alex-test-invalid-db-sessions",
			CostDir:                "/tmp/alex-test-invalid-db-costs",
			EnableMCP:              false,
			RequireSessionDatabase: true,
			SessionDatabaseURL:     "not-a-database-url",
		}

		if _, err := BuildContainer(config); err == nil {
			t.Fatal("BuildContainer() should fail when session database URL is invalid")
		}
	})
}

func TestBuildContainer_UsesPostgresStores(t *testing.T) {
	_, dbURL, cleanup := testutil.NewPostgresTestPool(t)
	defer cleanup()

	config := Config{
		LLMProvider:            "mock",
		LLMModel:               "test",
		MaxTokens:              100000,
		MaxIterations:          20,
		SessionDir:             "/tmp/alex-test-postgres-sessions",
		CostDir:                "/tmp/alex-test-postgres-costs",
		EnableMCP:              false,
		RequireSessionDatabase: true,
		SessionDatabaseURL:     dbURL,
	}

	container, err := BuildContainer(config)
	if err != nil {
		t.Fatalf("BuildContainer() error = %v", err)
	}
	defer func() { _ = container.Shutdown() }()

	if container.SessionDB == nil {
		t.Fatal("expected session DB to be initialized")
	}

	if _, ok := container.SessionStore.(*postgresstore.Store); !ok {
		t.Fatalf("expected Postgres session store, got %T", container.SessionStore)
	}
	if _, ok := container.StateStore.(*sessionstate.PostgresStore); !ok {
		t.Fatalf("expected Postgres state store, got %T", container.StateStore)
	}
	if _, ok := container.HistoryStore.(*sessionstate.PostgresStore); !ok {
		t.Fatalf("expected Postgres history store, got %T", container.HistoryStore)
	}
}

func TestMCPInitializationTracker(t *testing.T) {
	tracker := newMCPInitializationTracker()

	// Initial state
	status := tracker.Snapshot()
	if status.Ready {
		t.Error("Expected Ready to be false initially")
	}
	if status.Attempts != 0 {
		t.Error("Expected Attempts to be 0 initially")
	}

	// Record attempt
	tracker.recordAttempt()
	status = tracker.Snapshot()
	if status.Attempts != 1 {
		t.Errorf("Expected Attempts to be 1, got %d", status.Attempts)
	}

	// Record failure
	testErr := &testError{msg: "test error"}
	tracker.recordFailure(testErr)
	status = tracker.Snapshot()
	if status.Ready {
		t.Error("Expected Ready to be false after failure")
	}
	if status.LastError == nil {
		t.Error("Expected LastError to be set")
	}

	// Record success
	tracker.recordSuccess()
	status = tracker.Snapshot()
	if !status.Ready {
		t.Error("Expected Ready to be true after success")
	}
	if status.LastError != nil {
		t.Error("Expected LastError to be nil after success")
	}
}

type testError struct {
	msg string
}

func (e *testError) Error() string {
	return e.msg
}
