package mcp

import (
	"context"
	"os"
	"path/filepath"
	"reflect"
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

	// Keep this generous: in `go test ./...` packages run in parallel and
	// scheduling delays can be non-trivial under load.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	case <-ctx.Done():
		_ = pm.Stop(500 * time.Millisecond)
		t.Fatalf("timed out waiting for process exit: %v", ctx.Err())
	}
}

func TestProcessManagerRestartBackoffProgression(t *testing.T) {
	pm := NewProcessManager(ProcessConfig{
		Command: "__definitely_missing_command_for_backoff_test__",
	})

	var waits []time.Duration
	pm.waitFn = func(ctx context.Context, delay time.Duration) error {
		waits = append(waits, delay)
		return nil
	}

	err := pm.Restart(context.Background(), 5)
	if err == nil {
		t.Fatalf("expected restart to fail when command is missing")
	}

	want := []time.Duration{
		1 * time.Second,
		2 * time.Second,
		4 * time.Second,
		8 * time.Second,
		16 * time.Second,
	}
	if !reflect.DeepEqual(waits, want) {
		t.Fatalf("waits = %v, want %v", waits, want)
	}
}
