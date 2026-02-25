package subprocess

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

// Config defines how to spawn and manage an external agent subprocess.
type Config struct {
	Command    string
	Args       []string
	Env        map[string]string
	WorkingDir string
	Timeout    time.Duration

	// Detached makes the subprocess survive parent process death.
	// When true, the subprocess becomes a session leader (Setsid) and
	// stdout is redirected to OutputFile instead of a pipe.
	// The PID is written to StatusFile on start.
	Detached   bool
	OutputFile string // Required when Detached is true.
	StatusFile string // Optional; PID and start time written here.
}

// Subprocess manages the lifecycle of a single external agent process.
type Subprocess struct {
	cfg             Config
	cmd             *exec.Cmd
	stdin           io.WriteCloser
	stdout          io.ReadCloser
	stderr          io.ReadCloser
	stderrTail      *tailBuffer
	done            chan struct{}
	err             error
	pgid            int
	detachedOutFile *os.File
	mu              sync.Mutex
}

// New creates a new Subprocess from the given config.
func New(cfg Config) *Subprocess {
	return &Subprocess{cfg: cfg}
}

func (s *Subprocess) Start(ctx context.Context) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if s.cmd != nil {
		return fmt.Errorf("subprocess already started")
	}

	if ctx == nil {
		ctx = context.Background()
	}

	if s.cfg.Detached {
		return s.startDetached(ctx)
	}
	return s.startAttached(ctx)
}

// startAttached starts the subprocess as a child of the current process.
// Stdout/stderr are piped back. exec.CommandContext sends SIGKILL on cancel.
func (s *Subprocess) startAttached(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, s.cfg.Command, s.cfg.Args...)
	if s.cfg.WorkingDir != "" {
		cmd.Dir = s.cfg.WorkingDir
	}
	if len(s.cfg.Env) > 0 {
		env := append([]string{}, os.Environ()...)
		for k, v := range s.cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("stdout pipe: %w", err)
	}
	stderr, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("stderr pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("start subprocess: %w", err)
	}
	s.cmd = cmd
	s.stdin = stdin
	s.stdout = stdout
	s.stderr = stderr
	s.stderrTail = newTailBuffer(defaultStderrTail)
	s.done = make(chan struct{})

	go func() {
		err := cmd.Wait()
		s.mu.Lock()
		s.err = err
		close(s.done)
		s.mu.Unlock()
	}()

	go func() {
		if stderr == nil {
			return
		}
		_, _ = io.Copy(s.stderrTail, stderr)
	}()

	if s.cfg.Timeout > 0 {
		go func() {
			timer := time.NewTimer(s.cfg.Timeout)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = s.Stop()
			case <-s.done:
			}
		}()
	}

	if cmd.Process != nil {
		s.pgid, _ = syscall.Getpgid(cmd.Process.Pid)
	}

	return nil
}

// startDetached starts the subprocess as a session leader that survives
// parent process death. Stdout is redirected to a file (OutputFile).
// Does NOT use exec.CommandContext so the subprocess is not killed on
// context cancellation — the caller must use Stop() explicitly.
func (s *Subprocess) startDetached(ctx context.Context) error {
	if s.cfg.OutputFile == "" {
		return fmt.Errorf("detached mode requires OutputFile")
	}

	// Use plain exec.Command — no context-based kill.
	cmd := exec.Command(s.cfg.Command, s.cfg.Args...)
	if s.cfg.WorkingDir != "" {
		cmd.Dir = s.cfg.WorkingDir
	}
	if len(s.cfg.Env) > 0 {
		env := append([]string{}, os.Environ()...)
		for k, v := range s.cfg.Env {
			env = append(env, fmt.Sprintf("%s=%s", k, v))
		}
		cmd.Env = env
	}
	// Setsid makes the process a session leader — survives parent death.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	// Redirect stdout to output file.
	outFile, err := os.OpenFile(s.cfg.OutputFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
	if err != nil {
		return fmt.Errorf("open output file: %w", err)
	}
	cmd.Stdout = outFile

	stdin, err := cmd.StdinPipe()
	if err != nil {
		outFile.Close()
		return fmt.Errorf("stdin pipe: %w", err)
	}

	// Capture stderr in a tail buffer even in detached mode.
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		outFile.Close()
		return fmt.Errorf("stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		outFile.Close()
		return fmt.Errorf("start subprocess: %w", err)
	}

	s.cmd = cmd
	s.stdin = stdin
	s.stdout = nil // No stdout pipe — output goes to file.
	s.stderr = stderrPipe
	s.stderrTail = newTailBuffer(defaultStderrTail)
	s.done = make(chan struct{})
	s.detachedOutFile = outFile

	// Write status file with PID and start time.
	if s.cfg.StatusFile != "" {
		s.writeStatusFile()
	}

	go func() {
		err := cmd.Wait()
		outFile.Close()
		s.mu.Lock()
		s.err = err
		close(s.done)
		s.mu.Unlock()
	}()

	go func() {
		if stderrPipe == nil {
			return
		}
		_, _ = io.Copy(s.stderrTail, stderrPipe)
	}()

	if s.cfg.Timeout > 0 {
		go func() {
			timer := time.NewTimer(s.cfg.Timeout)
			defer timer.Stop()
			select {
			case <-timer.C:
				_ = s.Stop()
			case <-s.done:
			}
		}()
	}

	// For detached mode, use the PID directly (no process group — session leader).
	if cmd.Process != nil {
		s.pgid = cmd.Process.Pid
	}

	return nil
}

// writeStatusFile writes PID and start time to the status file.
func (s *Subprocess) writeStatusFile() {
	if s.cmd == nil || s.cmd.Process == nil {
		return
	}
	data := fmt.Sprintf(`{"pid":%d,"started_at":"%s"}`, s.cmd.Process.Pid, time.Now().UTC().Format(time.RFC3339))
	_ = os.WriteFile(s.cfg.StatusFile, []byte(data), 0o644)
}

func (s *Subprocess) Write(data []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stdin == nil {
		return fmt.Errorf("stdin not available")
	}
	_, err := s.stdin.Write(data)
	return err
}

func (s *Subprocess) Stdout() io.ReadCloser {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stdout
}

func (s *Subprocess) Stderr() io.ReadCloser {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.stderr
}

func (s *Subprocess) StderrTail() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.stderrTail == nil {
		return ""
	}
	return s.stderrTail.String()
}

func (s *Subprocess) Wait() error {
	s.mu.Lock()
	done := s.done
	s.mu.Unlock()
	if done == nil {
		return nil
	}
	<-done
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.err
}

func (s *Subprocess) Stop() error {
	s.mu.Lock()
	cmd := s.cmd
	done := s.done
	pgid := s.pgid
	detached := s.cfg.Detached
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if pgid == 0 {
		pgid = cmd.Process.Pid
	}

	// For detached (session leader) processes, send to the process directly.
	// For attached processes, send to the process group.
	if detached {
		_ = syscall.Kill(pgid, syscall.SIGTERM)
	} else {
		_ = syscall.Kill(-pgid, syscall.SIGTERM)
	}

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		if detached {
			_ = syscall.Kill(pgid, syscall.SIGKILL)
		} else {
			_ = syscall.Kill(-pgid, syscall.SIGKILL)
		}
		return nil
	}
}

func (s *Subprocess) PID() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.cmd != nil && s.cmd.Process != nil {
		return s.cmd.Process.Pid
	}
	return 0
}

// Done returns a channel that is closed when the subprocess exits.
// Returns nil if the subprocess has not been started.
func (s *Subprocess) Done() <-chan struct{} {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.done
}

const defaultStderrTail = 8 * 1024

type tailBuffer struct {
	mu  sync.Mutex
	max int
	buf []byte
}

func newTailBuffer(max int) *tailBuffer {
	if max <= 0 {
		max = defaultStderrTail
	}
	return &tailBuffer{max: max}
}

func (t *tailBuffer) Write(p []byte) (int, error) {
	if len(p) == 0 {
		return 0, nil
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(p) >= t.max {
		t.buf = append(t.buf[:0], p[len(p)-t.max:]...)
		return len(p), nil
	}

	if len(t.buf)+len(p) > t.max {
		excess := len(t.buf) + len(p) - t.max
		t.buf = t.buf[excess:]
	}
	t.buf = append(t.buf, p...)
	return len(p), nil
}

func (t *tailBuffer) String() string {
	t.mu.Lock()
	defer t.mu.Unlock()
	if len(t.buf) == 0 {
		return ""
	}
	copyBuf := make([]byte, len(t.buf))
	copy(copyBuf, t.buf)
	return string(copyBuf)
}
