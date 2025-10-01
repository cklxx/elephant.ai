package mcp

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfig_AddServer(t *testing.T) {
	config := &Config{
		MCPServers: make(map[string]ServerConfig),
	}

	config.AddServer("test", ServerConfig{
		Command: "test-command",
		Args:    []string{"arg1", "arg2"},
	})

	server, exists := config.GetServer("test")
	if !exists {
		t.Fatal("Expected server to exist")
	}

	if server.Command != "test-command" {
		t.Errorf("Expected command 'test-command', got %s", server.Command)
	}
	if len(server.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(server.Args))
	}
}

func TestConfig_RemoveServer(t *testing.T) {
	config := &Config{
		MCPServers: map[string]ServerConfig{
			"test": {Command: "test-command"},
		},
	}

	// Remove existing server
	removed := config.RemoveServer("test")
	if !removed {
		t.Error("Expected RemoveServer to return true")
	}

	_, exists := config.GetServer("test")
	if exists {
		t.Error("Expected server to be removed")
	}

	// Remove non-existent server
	removed = config.RemoveServer("nonexistent")
	if removed {
		t.Error("Expected RemoveServer to return false for non-existent server")
	}
}

func TestConfig_ListServers(t *testing.T) {
	config := &Config{
		MCPServers: map[string]ServerConfig{
			"server1": {Command: "cmd1"},
			"server2": {Command: "cmd2"},
			"server3": {Command: "cmd3"},
		},
	}

	names := config.ListServers()
	if len(names) != 3 {
		t.Errorf("Expected 3 servers, got %d", len(names))
	}

	// Check all names are present (order doesn't matter)
	nameSet := make(map[string]bool)
	for _, name := range names {
		nameSet[name] = true
	}

	for _, expected := range []string{"server1", "server2", "server3"} {
		if !nameSet[expected] {
			t.Errorf("Expected server %s in list", expected)
		}
	}
}

func TestConfig_GetActiveServers(t *testing.T) {
	config := &Config{
		MCPServers: map[string]ServerConfig{
			"active1":  {Command: "cmd1", Disabled: false},
			"disabled": {Command: "cmd2", Disabled: true},
			"active2":  {Command: "cmd3"},
		},
	}

	active := config.GetActiveServers()
	if len(active) != 2 {
		t.Errorf("Expected 2 active servers, got %d", len(active))
	}

	if _, exists := active["active1"]; !exists {
		t.Error("Expected active1 to be in active servers")
	}
	if _, exists := active["active2"]; !exists {
		t.Error("Expected active2 to be in active servers")
	}
	if _, exists := active["disabled"]; exists {
		t.Error("Expected disabled server to not be in active servers")
	}
}

func TestConfig_Validate(t *testing.T) {
	tests := []struct {
		name      string
		config    *Config
		expectErr bool
	}{
		{
			name: "valid config",
			config: &Config{
				MCPServers: map[string]ServerConfig{
					"test": {Command: "test-cmd"},
				},
			},
			expectErr: false,
		},
		{
			name: "empty servers",
			config: &Config{
				MCPServers: nil,
			},
			expectErr: true,
		},
		{
			name: "missing command",
			config: &Config{
				MCPServers: map[string]ServerConfig{
					"test": {Command: ""},
				},
			},
			expectErr: true,
		},
		{
			name: "invalid characters in command",
			config: &Config{
				MCPServers: map[string]ServerConfig{
					"test": {Command: "cmd\nwith\nnewlines"},
				},
			},
			expectErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.config.Validate()
			if tt.expectErr && err == nil {
				t.Error("Expected validation error, got nil")
			}
			if !tt.expectErr && err != nil {
				t.Errorf("Expected no validation error, got %v", err)
			}
		})
	}
}

func TestConfigLoader_LoadFromPath(t *testing.T) {
	// Create temporary config file
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	configJSON := `{
		"mcpServers": {
			"test-server": {
				"command": "test-command",
				"args": ["arg1", "arg2"],
				"env": {
					"TEST_VAR": "test_value"
				}
			}
		}
	}`

	if err := os.WriteFile(configPath, []byte(configJSON), 0644); err != nil {
		t.Fatalf("Failed to write test config: %v", err)
	}

	// Load config
	loader := NewConfigLoader()
	config, err := loader.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load config: %v", err)
	}

	// Verify loaded config
	server, exists := config.GetServer("test-server")
	if !exists {
		t.Fatal("Expected test-server to exist")
	}

	if server.Command != "test-command" {
		t.Errorf("Expected command 'test-command', got %s", server.Command)
	}
	if len(server.Args) != 2 {
		t.Errorf("Expected 2 args, got %d", len(server.Args))
	}
	if server.Env["TEST_VAR"] != "test_value" {
		t.Errorf("Expected TEST_VAR='test_value', got %s", server.Env["TEST_VAR"])
	}
}

func TestConfigLoader_SaveToPath(t *testing.T) {
	tempDir := t.TempDir()
	configPath := filepath.Join(tempDir, "test-config.json")

	// Create config
	config := &Config{
		MCPServers: map[string]ServerConfig{
			"test": {
				Command: "test-cmd",
				Args:    []string{"arg1"},
				Env: map[string]string{
					"VAR1": "value1",
				},
			},
		},
	}

	// Save config
	loader := NewConfigLoader()
	if err := loader.SaveToPath(configPath, config); err != nil {
		t.Fatalf("Failed to save config: %v", err)
	}

	// Verify file exists
	if _, err := os.Stat(configPath); os.IsNotExist(err) {
		t.Fatal("Config file was not created")
	}

	// Load it back and verify
	loaded, err := loader.LoadFromPath(configPath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	server, exists := loaded.GetServer("test")
	if !exists {
		t.Fatal("Expected server to exist in loaded config")
	}
	if server.Command != "test-cmd" {
		t.Errorf("Expected command 'test-cmd', got %s", server.Command)
	}
}

func TestConfigLoader_ExpandEnvVars(t *testing.T) {
	// Set test environment variable
	os.Setenv("TEST_ENV_VAR", "test_value")
	defer os.Unsetenv("TEST_ENV_VAR")

	loader := NewConfigLoader()

	config := ServerConfig{
		Command: "${TEST_ENV_VAR}/command",
		Args:    []string{"--flag=${TEST_ENV_VAR}", "plain"},
		Env: map[string]string{
			"KEY": "${TEST_ENV_VAR}",
		},
	}

	expanded := loader.expandEnvVars(config)

	if expanded.Command != "test_value/command" {
		t.Errorf("Expected command to be expanded, got %s", expanded.Command)
	}
	if expanded.Args[0] != "--flag=test_value" {
		t.Errorf("Expected arg to be expanded, got %s", expanded.Args[0])
	}
	if expanded.Env["KEY"] != "test_value" {
		t.Errorf("Expected env var to be expanded, got %s", expanded.Env["KEY"])
	}
}

func TestConfigLoader_ExpandString(t *testing.T) {
	os.Setenv("TEST_VAR", "value")
	defer os.Unsetenv("TEST_VAR")

	loader := NewConfigLoader()

	tests := []struct {
		input    string
		expected string
	}{
		{"${TEST_VAR}", "value"},
		{"prefix-${TEST_VAR}-suffix", "prefix-value-suffix"},
		{"$TEST_VAR", "value"},
		{"no variables", "no variables"},
		{"${NONEXISTENT}", ""},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			result := loader.expandString(tt.input)
			if result != tt.expected {
				t.Errorf("Expected %q, got %q", tt.expected, result)
			}
		})
	}
}
