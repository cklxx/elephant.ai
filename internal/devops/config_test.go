package devops

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestApplyDefaults(t *testing.T) {
	cfg := &DevConfig{}
	applyDefaults(cfg)

	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want 8080", cfg.ServerPort)
	}
	if cfg.WebPort != 3000 {
		t.Errorf("WebPort = %d, want 3000", cfg.WebPort)
	}
	if cfg.CGOMode != "auto" {
		t.Errorf("CGOMode = %q, want auto", cfg.CGOMode)
	}
	if !cfg.AutoStopConflictingPorts {
		t.Error("AutoStopConflictingPorts should default to true")
	}
}

func TestApplyDefaultsSupervisor(t *testing.T) {
	cfg := &DevConfig{}
	applyDefaults(cfg)

	if cfg.Supervisor.TickInterval != 5*time.Second {
		t.Errorf("TickInterval = %v, want 5s", cfg.Supervisor.TickInterval)
	}
	if cfg.Supervisor.RestartMaxInWindow != 5 {
		t.Errorf("RestartMaxInWindow = %d, want 5", cfg.Supervisor.RestartMaxInWindow)
	}
	if cfg.Supervisor.CooldownDuration != 5*time.Minute {
		t.Errorf("CooldownDuration = %v, want 5m", cfg.Supervisor.CooldownDuration)
	}
}

func TestApplyEnv(t *testing.T) {
	os.Setenv("SERVER_PORT", "9090")
	os.Setenv("WEB_PORT", "4000")
	defer os.Unsetenv("SERVER_PORT")
	defer os.Unsetenv("WEB_PORT")

	cfg := &DevConfig{}
	applyDefaults(cfg)
	applyEnv(cfg)

	if cfg.ServerPort != 9090 {
		t.Errorf("ServerPort = %d, want 9090", cfg.ServerPort)
	}
	if cfg.WebPort != 4000 {
		t.Errorf("WebPort = %d, want 4000", cfg.WebPort)
	}
}

func TestLoadDevConfig(t *testing.T) {
	// Test with non-existent config file (should use defaults)
	cfg, err := LoadDevConfig("/nonexistent/config.yaml")
	if err != nil {
		t.Fatalf("LoadDevConfig error: %v", err)
	}

	if cfg.ServerPort != 8080 {
		t.Errorf("ServerPort = %d, want 8080", cfg.ServerPort)
	}
	if cfg.ProjectDir == "" {
		t.Error("ProjectDir should be set")
	}
}

func TestLoadDevConfigYAML(t *testing.T) {
	dir := t.TempDir()
	configFile := filepath.Join(dir, "config.yaml")
	content := `devops:
  server_port: 7070
  web_port: 5000
  cgo_mode: "off"
`
	if err := os.WriteFile(configFile, []byte(content), 0o644); err != nil {
		t.Fatalf("write config file: %v", err)
	}

	cfg, err := LoadDevConfig(configFile)
	if err != nil {
		t.Fatalf("LoadDevConfig error: %v", err)
	}

	if cfg.ServerPort != 7070 {
		t.Errorf("ServerPort = %d, want 7070", cfg.ServerPort)
	}
	if cfg.WebPort != 5000 {
		t.Errorf("WebPort = %d, want 5000", cfg.WebPort)
	}
	if cfg.CGOMode != "off" {
		t.Errorf("CGOMode = %q, want off", cfg.CGOMode)
	}
}

func TestServiceStateString(t *testing.T) {
	tests := []struct {
		state ServiceState
		want  string
	}{
		{StateStopped, "stopped"},
		{StateStarting, "starting"},
		{StateRunning, "running"},
		{StateHealthy, "healthy"},
		{StateStopping, "stopping"},
		{StateFailed, "failed"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.want {
			t.Errorf("ServiceState(%d).String() = %q, want %q", tt.state, got, tt.want)
		}
	}
}
