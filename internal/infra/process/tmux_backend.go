package process

import (
	"context"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

const (
	tmuxSocket      = "elephant"
	tmuxPollDefault = 500 * time.Millisecond
)

// TmuxBackend spawns processes inside tmux sessions using a dedicated socket.
// This gives human observability (`tmux -L elephant attach -t <session>`).
type TmuxBackend struct{}

// Available reports whether tmux is installed and usable.
func (b *TmuxBackend) Available() bool {
	_, err := exec.LookPath("tmux")
	return err == nil
}

// Start creates a tmux session and runs the command inside it.
// The returned ProcessHandle monitors the process via PID liveness polling.
func (b *TmuxBackend) Start(ctx context.Context, cfg ProcessConfig) (ProcessHandle, error) {
	sessionName := tmuxSessionName(cfg.Name)

	// Kill any pre-existing session with the same name (stale leftover).
	_ = exec.CommandContext(ctx, "tmux", "-L", tmuxSocket, "kill-session", "-t", sessionName).Run()

	// Build the command string for tmux.
	cmdStr := cfg.Command
	if len(cfg.Args) > 0 {
		cmdStr += " " + strings.Join(cfg.Args, " ")
	}

	// Build tmux new-session args.
	args := []string{"-L", tmuxSocket, "new-session", "-d", "-s", sessionName}
	if cfg.WorkingDir != "" {
		args = append(args, "-c", cfg.WorkingDir)
	}

	// Set environment variables inside the tmux session.
	// tmux new-session supports -e KEY=VAL (tmux 3.2+).
	if len(cfg.Env) > 0 {
		for k, v := range cfg.Env {
			if v == "" {
				continue // unset semantics — skip
			}
			args = append(args, "-e", fmt.Sprintf("%s=%s", k, v))
		}
	}

	args = append(args, cmdStr)

	tmuxCmd := exec.CommandContext(ctx, "tmux", args...)
	if out, err := tmuxCmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("tmux new-session: %s: %w", strings.TrimSpace(string(out)), err)
	}

	// Get the PID of the process running inside the tmux pane.
	// Retry briefly: tmux needs a moment to register the pane, and short-lived
	// commands may cause the session to vanish before we can query it.
	var pid int
	var pidErr error
	for attempt := 0; attempt < 3; attempt++ {
		pid, pidErr = tmuxPanePID(ctx, sessionName)
		if pidErr == nil {
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if pidErr != nil {
		// Session may have already exited (very short-lived command).
		// Clean up and return an already-done handle.
		_ = exec.CommandContext(ctx, "tmux", "-L", tmuxSocket, "kill-session", "-t", sessionName).Run()
		h := &tmuxHandle{
			name:        cfg.Name,
			sessionName: sessionName,
			done:        make(chan struct{}),
		}
		close(h.done)
		return h, nil
	}

	h := &tmuxHandle{
		name:        cfg.Name,
		sessionName: sessionName,
		pid:         pid,
		done:        make(chan struct{}),
	}

	// Monitor process liveness in background.
	go h.monitor(cfg.Timeout)

	return h, nil
}

// tmuxSessionName generates a tmux session name from the process name.
// Convention: elephant-<name> (sanitised for tmux).
func tmuxSessionName(name string) string {
	// tmux disallows dots and colons in session names.
	safe := strings.NewReplacer(".", "-", ":", "-", "/", "-").Replace(name)
	return "elephant-" + safe
}

// tmuxPanePID returns the PID of the command running in the first pane of a session.
func tmuxPanePID(ctx context.Context, sessionName string) (int, error) {
	out, err := exec.CommandContext(ctx, "tmux", "-L", tmuxSocket,
		"list-panes", "-t", sessionName, "-F", "#{pane_pid}").Output()
	if err != nil {
		return 0, fmt.Errorf("tmux list-panes: %w", err)
	}
	line := strings.TrimSpace(string(out))
	if line == "" {
		return 0, fmt.Errorf("no pane pid for session %s", sessionName)
	}
	// Take the first line (first pane).
	if idx := strings.IndexByte(line, '\n'); idx >= 0 {
		line = line[:idx]
	}
	return strconv.Atoi(strings.TrimSpace(line))
}

// tmuxHandle implements ProcessHandle for tmux-managed processes.
type tmuxHandle struct {
	name        string
	sessionName string
	pid         int
	done        chan struct{}
	err         error
	mu          sync.Mutex
}

func (h *tmuxHandle) Backend() string  { return "tmux" }
func (h *tmuxHandle) Name() string     { return h.name }
func (h *tmuxHandle) PID() int         { return h.pid }
func (h *tmuxHandle) Done() <-chan struct{} { return h.done }

func (h *tmuxHandle) Wait() error {
	<-h.done
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.err
}

func (h *tmuxHandle) Stop() error {
	GracefulStop(h.pid, h.done, ShutdownPolicy{})

	// Clean up the tmux session.
	_ = exec.Command("tmux", "-L", tmuxSocket, "kill-session", "-t", h.sessionName).Run()
	return nil
}

func (h *tmuxHandle) StderrTail() string {
	// Capture last 50 lines from the tmux pane.
	out, err := exec.Command("tmux", "-L", tmuxSocket,
		"capture-pane", "-t", h.sessionName, "-p", "-S", "-50").Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func (h *tmuxHandle) Alive() bool {
	return IsAlive(h.pid)
}

// monitor polls process liveness and closes Done when the process exits.
func (h *tmuxHandle) monitor(timeout time.Duration) {
	var deadline <-chan time.Time
	if timeout > 0 {
		timer := time.NewTimer(timeout)
		defer timer.Stop()
		deadline = timer.C
	}

	ticker := time.NewTicker(tmuxPollDefault)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			if !IsAlive(h.pid) {
				h.mu.Lock()
				close(h.done)
				h.mu.Unlock()
				// Clean up tmux session.
				_ = exec.Command("tmux", "-L", tmuxSocket, "kill-session", "-t", h.sessionName).Run()
				return
			}
		case <-deadline:
			h.mu.Lock()
			h.err = fmt.Errorf("process timed out after %s", timeout)
			h.mu.Unlock()
			_ = h.Stop()
			return
		}
	}
}
