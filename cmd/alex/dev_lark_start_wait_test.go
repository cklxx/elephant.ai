package main

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestWaitForSupervisorPIDPublicationReturnsEarlyOnExit(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "lark-supervisor.pid")
	done := make(chan error, 1)
	done <- errors.New("exit status 1")

	killCalled := false
	_, err := waitForSupervisorPIDPublication(
		pidFile,
		"/tmp/lark-supervisor.log",
		2*time.Second,
		done,
		func() error {
			killCalled = true
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected startup failure error when supervisor exits early")
	}
	if !strings.Contains(err.Error(), "supervisor failed to start") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(err.Error(), "/tmp/lark-supervisor.log") {
		t.Fatalf("error should mention log path, got: %v", err)
	}
	if killCalled {
		t.Fatal("kill function should not be called when process already exited")
	}
}

func TestWaitForSupervisorPIDPublicationReturnsPublishedPID(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "lark-supervisor.pid")
	if err := os.WriteFile(pidFile, []byte(strconv.Itoa(os.Getpid())), 0o644); err != nil {
		t.Fatalf("write pid file: %v", err)
	}

	done := make(chan error)
	killCalled := false
	pid, err := waitForSupervisorPIDPublication(
		pidFile,
		"/tmp/lark-supervisor.log",
		2*time.Second,
		done,
		func() error {
			killCalled = true
			return nil
		},
	)
	if err != nil {
		t.Fatalf("waitForSupervisorPIDPublication returned error: %v", err)
	}
	if pid != os.Getpid() {
		t.Fatalf("pid = %d, want %d", pid, os.Getpid())
	}
	if killCalled {
		t.Fatal("kill function should not be called when pid is published")
	}
}

func TestWaitForSupervisorPIDPublicationTimesOutAndKills(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidFile := filepath.Join(dir, "lark-supervisor.pid")
	done := make(chan error)

	killCalled := false
	_, err := waitForSupervisorPIDPublication(
		pidFile,
		"/tmp/lark-supervisor.log",
		150*time.Millisecond,
		done,
		func() error {
			killCalled = true
			return nil
		},
	)
	if err == nil {
		t.Fatal("expected timeout error")
	}
	if !strings.Contains(err.Error(), "supervisor start timed out") {
		t.Fatalf("unexpected error: %v", err)
	}
	if !killCalled {
		t.Fatal("kill function should be called on timeout")
	}
}

