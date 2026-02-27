package process

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// execHandle implements PipedHandle for os/exec-based processes.
type execHandle struct {
	name       string
	cfg        ProcessConfig
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	stderrTail *TailBuffer
	done       chan struct{}
	err        error
	pgid       int
	outFile    *os.File // non-nil in detached mode
	mu         sync.Mutex
}

// ExecBackend spawns processes via os/exec and returns PipedHandle.
type ExecBackend struct{}

// Start spawns a process and returns a PipedHandle.
func (b *ExecBackend) Start(ctx context.Context, cfg ProcessConfig) (PipedHandle, error) {
	if cfg.Detached {
		return b.startDetached(ctx, cfg)
	}
	return b.startAttached(ctx, cfg)
}

func (b *ExecBackend) startAttached(ctx context.Context, cfg ProcessConfig) (PipedHandle, error) {
	cmd := exec.CommandContext(ctx, cfg.Command, cfg.Args...)
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}
	if len(cfg.Env) > 0 {
		cmd.Env = MergeEnv(cfg.Env)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start process: %w", err)
	}

	h := &execHandle{
		name:       cfg.Name,
		cfg:        cfg,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     stdout,
		stderr:     stderr,
		stderrTail: NewTailBuffer(DefaultStderrTail),
		done:       make(chan struct{}),
	}

	go func() {
		err := cmd.Wait()
		h.mu.Lock()
		h.err = err
		close(h.done)
		h.mu.Unlock()
	}()

	go func() {
		if stderr == nil {
			return
		}
		_, _ = io.Copy(h.stderrTail, stderr)
	}()

	if cfg.Timeout > 0 {
		go func() {
			timer := time.NewTimer(cfg.Timeout)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = h.Stop()
			case <-h.done:
			}
		}()
	}

	if cmd.Process != nil {
		h.pgid, _ = syscall.Getpgid(cmd.Process.Pid)
	}

	return h, nil
}

func (b *ExecBackend) startDetached(ctx context.Context, cfg ProcessConfig) (PipedHandle, error) {
	if cfg.OutputFile == "" {
		return nil, fmt.Errorf("detached mode requires OutputFile")
	}

	cmd := exec.Command(cfg.Command, cfg.Args...)
	if cfg.WorkingDir != "" {
		cmd.Dir = cfg.WorkingDir
	}
	if len(cfg.Env) > 0 {
		cmd.Env = MergeEnv(cfg.Env)
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	outFile, err := os.OpenFile(cfg.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open output file: %w", err)
	}
	cmd.Stdout = outFile

	stdin, err := cmd.StdinPipe()
	if err != nil {
		outFile.Close()
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}

	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		outFile.Close()
		return nil, fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return nil, fmt.Errorf("start process: %w", err)
	}

	h := &execHandle{
		name:       cfg.Name,
		cfg:        cfg,
		cmd:        cmd,
		stdin:      stdin,
		stdout:     nil, // output goes to file
		stderr:     stderrPipe,
		stderrTail: NewTailBuffer(DefaultStderrTail),
		done:       make(chan struct{}),
		outFile:    outFile,
	}

	if cfg.StatusFile != "" {
		writeStatusFile(cfg.StatusFile, cmd.Process.Pid)
	}

	go func() {
		err := cmd.Wait()
		outFile.Close()
		h.mu.Lock()
		h.err = err
		close(h.done)
		h.mu.Unlock()
	}()

	go func() {
		if stderrPipe == nil {
			return
		}
		_, _ = io.Copy(h.stderrTail, stderrPipe)
	}()

	if cfg.Timeout > 0 {
		go func() {
			timer := time.NewTimer(cfg.Timeout)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = h.Stop()
			case <-h.done:
			}
		}()
	}

	if cmd.Process != nil {
		h.pgid = cmd.Process.Pid // session leader — use PID directly
	}

	return h, nil
}

func writeStatusFile(path string, pid int) {
	data := fmt.Sprintf(`{"pid":%d,"started_at":"%s"}`, pid, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(path, []byte(data), 0o644)
}

// --- execHandle implements PipedHandle ---

func (h *execHandle) Name() string { return h.name }

func (h *execHandle) PID() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cmd != nil && h.cmd.Process != nil {
		return h.cmd.Process.Pid
	}
	return 0
}

func (h *execHandle) Done() <-chan struct{} {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.done
}

func (h *execHandle) Wait() error {
	h.mu.Lock()
	done := h.done
	h.mu.Unlock()
	if done == nil {
		return nil
	}
	<-done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *execHandle) Stop() error {
	h.mu.Lock()
	cmd := h.cmd
	done := h.done
	pgid := h.pgid
	detached := h.cfg.Detached
	h.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if pgid == 0 {
		pgid = cmd.Process.Pid
	}

	GracefulStop(pgid, done, ShutdownPolicy{
		UseProcessGroup: !detached,
	})
	return nil
}

func (h *execHandle) StderrTail() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.stderrTail == nil {
		return ""
	}
	return h.stderrTail.String()
}

func (h *execHandle) Alive() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.cmd == nil || h.cmd.Process == nil {
		return false
	}
	return IsAlive(h.cmd.Process.Pid)
}

func (h *execHandle) Stdin() io.WriteCloser {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stdin
}

func (h *execHandle) Stdout() io.ReadCloser {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.stdout
}
