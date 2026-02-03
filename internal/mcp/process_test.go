package mcp

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func channelClosed(ch <-chan struct{}) bool {
	select {
	case <-ch:
		return true
	default:
		return false
	}
}

func TestProcessManagerReinitializesStopChan(t *testing.T) {
	pm := NewProcessManager(ProcessConfig{
		Command: "sleep",
		Args:    []string{"0.05"},
	})

	if err := pm.Start(context.Background()); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	if pm.stopChan == nil || channelClosed(pm.stopChan) {
		t.Fatalf("expected stopChan to be open after start")
	}

	if err := pm.Stop(500 * time.Millisecond); err != nil {
		t.Fatalf("stop failed: %v", err)
	}

	if pm.stopChan == nil || !channelClosed(pm.stopChan) {
		t.Fatalf("expected stopChan to be closed after stop")
	}

	if err := pm.Start(context.Background()); err != nil {
		t.Fatalf("restart failed: %v", err)
	}

	if pm.stopChan == nil || channelClosed(pm.stopChan) {
		t.Fatalf("expected stopChan to be reinitialized after restart")
	}

	_ = pm.Stop(500 * time.Millisecond)
}

func TestProcessManager_InheritsEnvironmentWhenOverridesProvided(t *testing.T) {
	tmpDir := t.TempDir()
	scriptPath := filepath.Join(tmpDir, "run.sh")

	// If PATH isn't inherited, /usr/bin/env can't locate "sh" and the script exits non-zero.
	script := "#!/usr/bin/env sh\nexit 0\n"
	if err := os.WriteFile(scriptPath, []byte(script), 0o755); err != nil {
		t.Fatalf("write script: %v", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	pm := NewProcessManager(ProcessConfig{
		Command: scriptPath,
		Env: map[string]string{
			"TEST_VAR": "test",
		},
	})
	if err := pm.Start(ctx); err != nil {
		t.Fatalf("start failed: %v", err)
	}

	select {
	case err := <-pm.waitDone:
		if err != nil {
			t.Fatalf("expected script to exit 0, got %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("timed out waiting for process exit")
	}
}
