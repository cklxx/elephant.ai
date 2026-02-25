package main

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

func TestReadLivePIDFileAlive(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "lark-supervisor.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	pid, exists, alive := readLivePIDFile(pidFile, true)
	if !exists {
		t.Fatal("expected pid file to exist")
	}
	if !alive {
		t.Fatal("expected current process pid to be alive")
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
}

func TestReadLivePIDFileCleansStale(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "lark-supervisor.pid")
	if err := os.WriteFile(pidFile, []byte("999999"), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	pid, exists, alive := readLivePIDFile(pidFile, true)
	if !exists {
		t.Fatal("expected pid file read attempt to report exists")
	}
	if alive || pid != 0 {
		t.Fatalf("expected stale pid result, got pid=%d alive=%v", pid, alive)
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid file removed, stat err=%v", err)
	}
}
