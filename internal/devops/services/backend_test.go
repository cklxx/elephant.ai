package services

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/devops"
	"alex/internal/devops/health"
	devlog "alex/internal/devops/log"
	"alex/internal/devops/port"
	"alex/internal/devops/process"
)

func newTestBackendService(t *testing.T) (*BackendService, string) {
	t.Helper()
	dir := t.TempDir()
	sw := devlog.NewSectionWriter(os.Stdout, false)
	pm := process.NewManager(filepath.Join(dir, "pids"), filepath.Join(dir, "logs"))
	pa := port.NewAllocator()
	hc := health.NewChecker()

	cfg := BackendConfig{
		Port:       0,
		OutputBin:  filepath.Join(dir, "alex-server"),
		ProjectDir: dir,
		LogDir:     filepath.Join(dir, "logs"),
		CGOMode:    "off",
	}

	svc := NewBackendService(pm, pa, hc, sw, cfg)
	return svc, dir
}

func TestStagingPath(t *testing.T) {
	svc, _ := newTestBackendService(t)
	want := svc.config.OutputBin + ".staging"
	if got := svc.stagingPath(); got != want {
		t.Errorf("stagingPath() = %q, want %q", got, want)
	}
}

func TestPromoteAtomicRename(t *testing.T) {
	svc, dir := newTestBackendService(t)

	// Create a fake staging binary
	staging := filepath.Join(dir, "alex-server.staging")
	if err := os.WriteFile(staging, []byte("fake-binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if err := svc.Promote(staging); err != nil {
		t.Fatalf("Promote() error: %v", err)
	}

	// Staging file should be gone (renamed)
	if _, err := os.Stat(staging); !os.IsNotExist(err) {
		t.Error("staging file should not exist after Promote")
	}

	// Production binary should exist with correct content
	data, err := os.ReadFile(svc.config.OutputBin)
	if err != nil {
		t.Fatalf("production binary not found: %v", err)
	}
	if string(data) != "fake-binary" {
		t.Errorf("production binary content = %q, want %q", string(data), "fake-binary")
	}
}

func TestPromoteSetsSkipNextBuild(t *testing.T) {
	svc, dir := newTestBackendService(t)

	staging := filepath.Join(dir, "alex-server.staging")
	if err := os.WriteFile(staging, []byte("binary"), 0o755); err != nil {
		t.Fatal(err)
	}

	if svc.skipNextBuild {
		t.Fatal("skipNextBuild should be false before Promote")
	}

	if err := svc.Promote(staging); err != nil {
		t.Fatalf("Promote() error: %v", err)
	}

	if !svc.skipNextBuild {
		t.Error("skipNextBuild should be true after Promote")
	}
}

func TestSkipNextBuild(t *testing.T) {
	svc, dir := newTestBackendService(t)

	// Write a fake production binary so build() can "succeed" by skipping
	if err := os.WriteFile(svc.config.OutputBin, []byte("existing"), 0o755); err != nil {
		t.Fatal(err)
	}

	// Simulate the Promote path
	staging := filepath.Join(dir, "alex-server.staging")
	if err := os.WriteFile(staging, []byte("new-binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := svc.Promote(staging); err != nil {
		t.Fatal(err)
	}

	// build() should skip without error (no toolchain needed)
	if err := svc.build(context.Background()); err != nil {
		t.Fatalf("build() should skip after Promote, got: %v", err)
	}

	// The flag should be cleared
	if svc.skipNextBuild {
		t.Error("skipNextBuild should be cleared after build()")
	}

	// Production binary should still be the promoted one
	data, err := os.ReadFile(svc.config.OutputBin)
	if err != nil {
		t.Fatal(err)
	}
	if string(data) != "new-binary" {
		t.Errorf("production binary = %q, want %q (should not have been rebuilt)", string(data), "new-binary")
	}
}

func TestBackendImplementsBuildable(t *testing.T) {
	svc, _ := newTestBackendService(t)
	// Compile-time check: BackendService implements devops.Buildable
	var _ devops.Buildable = svc
}

func TestPromoteNonexistentStaging(t *testing.T) {
	svc, dir := newTestBackendService(t)
	nonexistent := filepath.Join(dir, "does-not-exist")
	if err := svc.Promote(nonexistent); err == nil {
		t.Error("Promote() with nonexistent staging should return error")
	}
}
