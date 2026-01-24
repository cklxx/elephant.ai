package mcp

import (
	"context"
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
