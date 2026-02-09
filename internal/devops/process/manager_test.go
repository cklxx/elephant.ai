package process

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"testing"
	"time"
)

func TestStartKeepsNewProcessWhenOldExitWaits(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := NewManager(filepath.Join(dir, "pids"), filepath.Join(dir, "logs"))

	first, err := mgr.Start(context.Background(), "svc", exec.Command("sleep", "1"))
	if err != nil {
		t.Fatalf("start first process: %v", err)
	}
	defer func() { _ = mgr.Stop(context.Background(), "svc") }()

	second, err := mgr.Start(context.Background(), "svc", exec.Command("sleep", "4"))
	if err != nil {
		t.Fatalf("start second process: %v", err)
	}

	if second.PID == first.PID {
		t.Fatalf("expected different PIDs, both were %d", first.PID)
	}

	time.Sleep(1500 * time.Millisecond)

	running, pid := mgr.IsRunning("svc")
	if !running {
		t.Fatal("expected latest process to remain tracked and running")
	}
	if pid != second.PID {
		t.Fatalf("running pid = %d, want %d", pid, second.PID)
	}
}

func TestStopSkipsKillWhenPIDIdentityMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := NewManager(filepath.Join(dir, "pids"), filepath.Join(dir, "logs"))

	child := exec.Command("sleep", "5")
	if err := child.Start(); err != nil {
		t.Fatalf("start child process: %v", err)
	}
	defer func() {
		_ = child.Process.Kill()
		_ = child.Wait()
	}()

	pidFile := filepath.Join(dir, "pids", "svc.pid")
	metaFile := pidMetaFile(pidFile)
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		t.Fatalf("mkdir pid dir: %v", err)
	}
	if err := atomicWriteFile(pidFile, []byte(strconv.Itoa(child.Process.Pid))); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
	if err := writePIDMetadata(metaFile, "definitely-not-this-process"); err != nil {
		t.Fatalf("write pid metadata: %v", err)
	}

	if err := mgr.Stop(context.Background(), "svc"); err != nil {
		t.Fatalf("stop with mismatched identity: %v", err)
	}

	if !isProcessAlive(child.Process.Pid) {
		t.Fatal("child process was killed despite PID identity mismatch")
	}
	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid file removed, stat err=%v", err)
	}
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid metadata removed, stat err=%v", err)
	}
}

func TestRecoverRejectsPIDIdentityMismatch(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	mgr := NewManager(filepath.Join(dir, "pids"), filepath.Join(dir, "logs"))

	pidFile := filepath.Join(dir, "pids", "svc.pid")
	metaFile := pidMetaFile(pidFile)
	if err := os.MkdirAll(filepath.Dir(pidFile), 0o755); err != nil {
		t.Fatalf("mkdir pid dir: %v", err)
	}
	if err := atomicWriteFile(pidFile, []byte(strconv.Itoa(os.Getpid()))); err != nil {
		t.Fatalf("write pid file: %v", err)
	}
	if err := writePIDMetadata(metaFile, "definitely-not-current-test-process"); err != nil {
		t.Fatalf("write pid metadata: %v", err)
	}

	if _, err := mgr.Recover("svc"); err == nil {
		t.Fatal("recover should fail on identity mismatch")
	}

	if _, err := os.Stat(pidFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid file removed, stat err=%v", err)
	}
	if _, err := os.Stat(metaFile); !os.IsNotExist(err) {
		t.Fatalf("expected stale pid metadata removed, stat err=%v", err)
	}
}

func TestRecoverAcceptsMatchingPIDIdentity(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	pidDir := filepath.Join(dir, "pids")
	logDir := filepath.Join(dir, "logs")

	owner := NewManager(pidDir, logDir)
	proc, err := owner.Start(context.Background(), "svc", exec.Command("sleep", "5"))
	if err != nil {
		t.Fatalf("start process: %v", err)
	}
	defer func() { _ = owner.Stop(context.Background(), "svc") }()

	recovered := NewManager(pidDir, logDir)
	mp, err := recovered.Recover("svc")
	if err != nil {
		t.Fatalf("recover process: %v", err)
	}
	if mp.PID != proc.PID {
		t.Fatalf("recovered pid = %d, want %d", mp.PID, proc.PID)
	}
}
