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
}

// Subprocess manages the lifecycle of a single external agent process.
type Subprocess struct {
	cfg        Config
	cmd        *exec.Cmd
	stdin      io.WriteCloser
	stdout     io.ReadCloser
	stderr     io.ReadCloser
	stderrTail *tailBuffer
	done       chan struct{}
	err        error
	pgid       int
	mu         sync.Mutex
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
	s.mu.Unlock()

	if cmd == nil || cmd.Process == nil {
		return nil
	}
	if pgid == 0 {
		pgid = cmd.Process.Pid
	}
	_ = syscall.Kill(-pgid, syscall.SIGTERM)

	select {
	case <-done:
		return nil
	case <-time.After(5 * time.Second):
		_ = syscall.Kill(-pgid, syscall.SIGKILL)
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
