package process

import (
	"os/exec"
	"syscall"
	"testing"
	"time"
)

func TestGracefulStop_AlreadyDead(t *testing.T) {
	// Calling GracefulStop on a non-existent PID should not panic.
	GracefulStop(1<<22-1, nil, ShutdownPolicy{Grace: 100 * time.Millisecond})
}

func TestGracefulStop_ZeroPID(t *testing.T) {
	GracefulStop(0, nil, ShutdownPolicy{})
}

func TestGracefulStop_NegativePID(t *testing.T) {
	GracefulStop(-5, nil, ShutdownPolicy{})
}

func TestGracefulStop_KillsSleepProcess(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}

	pid := cmd.Process.Pid
	if !IsAlive(pid) {
		t.Fatal("process should be alive after start")
	}

	GracefulStop(pid, nil, ShutdownPolicy{
		Grace:        500 * time.Millisecond,
		PollInterval: 50 * time.Millisecond,
	})

	// Wait for reaper.
	_ = cmd.Wait()

	if IsAlive(pid) {
		t.Fatal("process should be dead after GracefulStop")
	}
}

func TestGracefulStop_RespectsDoneChannel(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	defer func() {
		_ = cmd.Process.Kill()
		_ = cmd.Wait()
	}()

	done := make(chan struct{})
	close(done) // already closed — should return immediately

	start := time.Now()
	GracefulStop(pid, done, ShutdownPolicy{
		Grace:        5 * time.Second,
		PollInterval: 50 * time.Millisecond,
	})
	elapsed := time.Since(start)

	// Should return almost immediately because done is closed.
	if elapsed > time.Second {
		t.Fatalf("took %v, expected immediate return", elapsed)
	}
}

func TestGracefulStop_ProcessGroup(t *testing.T) {
	cmd := exec.Command("sleep", "60")
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	pid := cmd.Process.Pid
	pgid, _ := syscall.Getpgid(pid)

	GracefulStop(pgid, nil, ShutdownPolicy{
		Grace:           500 * time.Millisecond,
		PollInterval:    50 * time.Millisecond,
		UseProcessGroup: true,
	})

	_ = cmd.Wait()

	if IsAlive(pid) {
		t.Fatal("process should be dead after process group kill")
	}
}

func TestShutdownPolicy_Defaults(t *testing.T) {
	p := ShutdownPolicy{}
	if p.grace() != 5*time.Second {
		t.Fatalf("grace: got %v", p.grace())
	}
	if p.pollInterval() != 250*time.Millisecond {
		t.Fatalf("poll: got %v", p.pollInterval())
	}
	if p.signal() != syscall.SIGTERM {
		t.Fatalf("signal: got %v", p.signal())
	}
}
