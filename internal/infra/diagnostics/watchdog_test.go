package diagnostics

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type mockLogger struct {
	warns []string
	infos []string
}

func (m *mockLogger) Debug(format string, args ...any) {}
func (m *mockLogger) Info(format string, args ...any)  { m.infos = append(m.infos, format) }
func (m *mockLogger) Warn(format string, args ...any)  { m.warns = append(m.warns, format) }
func (m *mockLogger) Error(format string, args ...any) {}

func TestWatchdogConfigDefaults(t *testing.T) {
	cfg := WatchdogConfig{}
	got := cfg.withDefaults()

	if got.Interval != 30*time.Second {
		t.Errorf("expected 30s interval, got %s", got.Interval)
	}
	if got.HeapThresholdBytes != 2*1024*1024*1024 {
		t.Errorf("expected 2GB heap threshold, got %d", got.HeapThresholdBytes)
	}
	if got.GoroutineThreshold != 10000 {
		t.Errorf("expected 10000 goroutine threshold, got %d", got.GoroutineThreshold)
	}
	if got.MaxDumps != 5 {
		t.Errorf("expected 5 max dumps, got %d", got.MaxDumps)
	}
}

func TestWatchdogCheck(t *testing.T) {
	logger := &mockLogger{}
	w := NewWatchdog(WatchdogConfig{
		// Set very high thresholds so we don't trigger alerts.
		HeapThresholdBytes: 100 * 1024 * 1024 * 1024, // 100GB
		GoroutineThreshold: 1000000,
	}, logger)

	var prevHeap uint64
	var prevGoroutines int
	w.check(&prevHeap, &prevGoroutines)

	// prevHeap should be updated.
	if prevHeap == 0 {
		t.Error("expected prevHeap to be updated after check")
	}
	if prevGoroutines == 0 {
		t.Error("expected prevGoroutines to be updated after check")
	}
}

func TestWatchdogRunCancellation(t *testing.T) {
	logger := &mockLogger{}
	w := NewWatchdog(WatchdogConfig{
		Interval: 10 * time.Millisecond,
	}, logger)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	done := make(chan struct{})
	go func() {
		w.Run(ctx)
		close(done)
	}()

	select {
	case <-done:
		// ok
	case <-time.After(2 * time.Second):
		t.Fatal("watchdog did not stop after context cancellation")
	}
}

func TestWatchdogDumpProfile(t *testing.T) {
	dir := t.TempDir()
	logger := &mockLogger{}
	w := NewWatchdog(WatchdogConfig{
		DumpDir: dir,
	}, logger)

	path, err := w.dumpProfile("heap", func(f *os.File) error {
		_, err := f.WriteString("fake heap data")
		return err
	})
	if err != nil {
		t.Fatalf("dumpProfile failed: %v", err)
	}

	if _, err := os.Stat(path); err != nil {
		t.Fatalf("dump file not found: %v", err)
	}

	// Verify it's in the right directory.
	if filepath.Dir(path) != dir {
		t.Errorf("dump file in wrong directory: %s", path)
	}
}

func TestWatchdogMaxDumps(t *testing.T) {
	logger := &mockLogger{}
	w := NewWatchdog(WatchdogConfig{
		DumpDir:  t.TempDir(),
		MaxDumps: 1,
	}, logger)

	// First dump should succeed.
	w.autoDumpHeap(3 * 1024 * 1024 * 1024)
	if w.dumpCount.Load() != 1 {
		t.Fatalf("expected dump count 1, got %d", w.dumpCount.Load())
	}

	// Second dump should be skipped.
	prevWarns := len(logger.warns)
	w.autoDumpHeap(3 * 1024 * 1024 * 1024)
	if w.dumpCount.Load() != 1 {
		t.Fatalf("expected dump count still 1, got %d", w.dumpCount.Load())
	}
	// Should have logged a "max dump count reached" warning.
	if len(logger.warns) <= prevWarns {
		t.Error("expected warning about max dump count")
	}
}

func TestFormatBytes(t *testing.T) {
	tests := []struct {
		input    uint64
		expected string
	}{
		{0, "0B"},
		{512, "512B"},
		{1024, "1.0KB"},
		{1024 * 1024, "1.0MB"},
		{1024 * 1024 * 1024, "1.0GB"},
		{2 * 1024 * 1024 * 1024, "2.0GB"},
	}
	for _, tt := range tests {
		got := formatBytes(tt.input)
		if got != tt.expected {
			t.Errorf("formatBytes(%d) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}
