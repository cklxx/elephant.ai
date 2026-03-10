package leaderlock

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestFileLock_AcquireRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock, err := NewFileLock(path, "test-lock")
	if err != nil {
		t.Fatalf("NewFileLock: %v", err)
	}

	acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}

	// Verify PID file was written.
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	if len(data) == 0 {
		t.Error("lock file should contain PID")
	}

	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release: %v", err)
	}
}

func TestFileLock_ReleaseWithoutAcquire(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock, err := NewFileLock(path, "test-lock")
	if err != nil {
		t.Fatalf("NewFileLock: %v", err)
	}

	// Release without Acquire should be a no-op.
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release without Acquire: %v", err)
	}
}

func TestFileLock_Name(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock, err := NewFileLock(path, "my-scheduler")
	if err != nil {
		t.Fatalf("NewFileLock: %v", err)
	}
	if lock.Name() != "my-scheduler" {
		t.Errorf("Name() = %q, want my-scheduler", lock.Name())
	}
	if lock.Path() != path {
		t.Errorf("Path() = %q, want %s", lock.Path(), path)
	}
}

func TestFileLock_CreatesParentDir(t *testing.T) {
	dir := t.TempDir()
	nested := filepath.Join(dir, "sub", "dir", "test.lock")

	lock, err := NewFileLock(nested, "test-lock")
	if err != nil {
		t.Fatalf("NewFileLock: %v", err)
	}

	acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected lock to be acquired")
	}
	defer lock.Release(context.Background())

	if _, err := os.Stat(nested); err != nil {
		t.Errorf("lock file should exist: %v", err)
	}
}

func TestFileLock_ReacquireAfterRelease(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.lock")

	lock, err := NewFileLock(path, "test-lock")
	if err != nil {
		t.Fatalf("NewFileLock: %v", err)
	}

	// Acquire → Release → Acquire should succeed.
	acquired, err := lock.Acquire(context.Background())
	if err != nil || !acquired {
		t.Fatalf("first Acquire: acquired=%v, err=%v", acquired, err)
	}
	if err := lock.Release(context.Background()); err != nil {
		t.Fatalf("Release: %v", err)
	}

	acquired, err = lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("second Acquire: %v", err)
	}
	if !acquired {
		t.Fatal("expected re-acquire to succeed after release")
	}
	lock.Release(context.Background())
}

func TestFileLock_ContendedViaSubprocess(t *testing.T) {
	if os.Getenv("FLOCK_CHILD") == "1" {
		// Child process: acquire lock and hold it.
		path := os.Getenv("FLOCK_PATH")
		signalFile := os.Getenv("FLOCK_SIGNAL")
		lock, err := NewFileLock(path, "child")
		if err != nil {
			os.Exit(2)
		}
		acquired, err := lock.Acquire(context.Background())
		if err != nil || !acquired {
			os.Exit(3)
		}
		// Signal parent via file, then block on stdin until parent closes it.
		_ = os.WriteFile(signalFile, []byte("LOCKED"), 0o644)
		buf := make([]byte, 1)
		os.Stdin.Read(buf) // blocks until parent kills us or closes stdin
		os.Exit(0)
	}

	// This test exercises real cross-process contention via exec.
	if testing.Short() {
		t.Skip("skipping subprocess test in short mode")
	}

	dir := t.TempDir()
	path := filepath.Join(dir, "contended.lock")
	signalFile := filepath.Join(dir, "signal")

	// Start child that holds the lock. Give it a stdin pipe to keep it alive.
	cmd := exec.Command(os.Args[0], "-test.run=^TestFileLock_ContendedViaSubprocess$")
	cmd.Env = append(os.Environ(), "FLOCK_CHILD=1", "FLOCK_PATH="+path, "FLOCK_SIGNAL="+signalFile)
	stdinPipe, err := cmd.StdinPipe()
	if err != nil {
		t.Fatalf("stdin pipe: %v", err)
	}

	if err := cmd.Start(); err != nil {
		t.Fatalf("start child: %v", err)
	}
	defer func() {
		stdinPipe.Close()
		cmd.Process.Kill()
		cmd.Wait()
	}()

	// Poll for signal file (child acquired lock).
	for i := 0; i < 100; i++ {
		if _, err := os.Stat(signalFile); err == nil {
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if _, err := os.Stat(signalFile); err != nil {
		t.Fatal("child did not signal lock acquisition")
	}

	// Parent tries to acquire — should fail (non-blocking).
	lock, err := NewFileLock(path, "parent")
	if err != nil {
		t.Fatalf("NewFileLock: %v", err)
	}
	acquired, err := lock.Acquire(context.Background())
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	if acquired {
		lock.Release(context.Background())
		t.Fatal("expected lock NOT to be acquired while child holds it")
	}
}
