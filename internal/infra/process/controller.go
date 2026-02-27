package process

import (
	"context"
	"sync"
	"time"
)

// ProcessInfo is a snapshot returned by Controller.List().
type ProcessInfo struct {
	Name    string
	PID     int
	Alive   bool
	Backend string // "exec" or "tmux"
}

// Controller is the central registry for all managed processes.
type Controller struct {
	exec    ExecBackend
	tmux    TmuxBackend
	handles map[string]ProcessHandle
	mu      sync.Mutex
}

// NewController creates a new process controller.
func NewController() *Controller {
	return &Controller{
		handles: make(map[string]ProcessHandle),
	}
}

// StartTmux spawns a process inside a tmux session for human observability.
// If tmux is not available, it falls back to StartExec transparently.
// Note: tmux-managed processes do not provide stdio pipes — only ProcessHandle.
func (c *Controller) StartTmux(ctx context.Context, cfg ProcessConfig) (ProcessHandle, error) {
	if !c.tmux.Available() {
		// Fallback: run via exec (caller loses tmux observability but process still works).
		h, err := c.exec.Start(ctx, cfg)
		if err != nil {
			return nil, err
		}
		c.register(cfg.Name, h)
		return h, nil
	}
	h, err := c.tmux.Start(ctx, cfg)
	if err != nil {
		return nil, err
	}
	c.register(cfg.Name, h)
	return h, nil
}

// TmuxAvailable reports whether the tmux backend is usable.
func (c *Controller) TmuxAvailable() bool {
	return c.tmux.Available()
}

// StartExec spawns a process via os/exec and registers it.
func (c *Controller) StartExec(ctx context.Context, cfg ProcessConfig) (PipedHandle, error) {
	h, err := c.exec.Start(ctx, cfg)
	if err != nil {
		return nil, err
	}
	c.register(cfg.Name, h)
	return h, nil
}

// Get returns a registered handle by name.
func (c *Controller) Get(name string) (ProcessHandle, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	h, ok := c.handles[name]
	return h, ok
}

// List returns a snapshot of all registered processes.
func (c *Controller) List() []ProcessInfo {
	c.mu.Lock()
	defer c.mu.Unlock()

	out := make([]ProcessInfo, 0, len(c.handles))
	for _, h := range c.handles {
		backend := "exec"
		if b, ok := h.(interface{ Backend() string }); ok {
			backend = b.Backend()
		}
		out = append(out, ProcessInfo{
			Name:    h.Name(),
			PID:     h.PID(),
			Alive:   h.Alive(),
			Backend: backend,
		})
	}
	return out
}

// StopAll stops all registered processes.
func (c *Controller) StopAll() error {
	c.mu.Lock()
	handles := make([]ProcessHandle, 0, len(c.handles))
	for _, h := range c.handles {
		handles = append(handles, h)
	}
	c.mu.Unlock()

	var lastErr error
	for _, h := range handles {
		if err := h.Stop(); err != nil {
			lastErr = err
		}
	}
	return lastErr
}

// Shutdown stops all processes and waits for them to exit.
func (c *Controller) Shutdown(timeout time.Duration) error {
	_ = c.StopAll()

	c.mu.Lock()
	handles := make([]ProcessHandle, 0, len(c.handles))
	for _, h := range c.handles {
		handles = append(handles, h)
	}
	c.mu.Unlock()

	deadline := time.After(timeout)
	for _, h := range handles {
		select {
		case <-h.Done():
		case <-deadline:
			return nil
		}
	}
	return nil
}

func (c *Controller) register(name string, h ProcessHandle) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.handles[name] = h

	// Auto-deregister when process exits.
	go func() {
		<-h.Done()
		c.mu.Lock()
		if c.handles[name] == h {
			delete(c.handles, name)
		}
		c.mu.Unlock()
	}()
}

