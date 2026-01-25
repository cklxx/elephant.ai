package mcp

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os/exec"
	"strings"
	"sync"
	"time"

	"alex/internal/async"
	"alex/internal/logging"
)

// ProcessManager manages an MCP server process lifecycle
type ProcessManager struct {
	command     string
	args        []string
	env         []string
	process     *exec.Cmd
	stdin       io.WriteCloser
	stdout      io.ReadCloser
	stderr      io.ReadCloser
	logger      logging.Logger
	mu          sync.Mutex
	running     bool
	restartChan chan struct{}
	stopChan    chan struct{}
	waitDone    chan error
}

// ProcessConfig configures the MCP server process
type ProcessConfig struct {
	Command string            // Executable command
	Args    []string          // Command arguments
	Env     map[string]string // Environment variables
}

// NewProcessManager creates a new process manager
func NewProcessManager(config ProcessConfig) *ProcessManager {
	pm := &ProcessManager{
		command:     config.Command,
		args:        config.Args,
		logger:      logging.NewComponentLogger(fmt.Sprintf("ProcessManager[%s]", config.Command)),
		restartChan: make(chan struct{}, 1),
		stopChan:    make(chan struct{}),
	}

	// Convert env map to []string format
	if config.Env != nil {
		pm.env = make([]string, 0, len(config.Env))
		for k, v := range config.Env {
			pm.env = append(pm.env, fmt.Sprintf("%s=%s", k, v))
		}
	}

	return pm
}

// Start spawns the MCP server process
func (pm *ProcessManager) Start(ctx context.Context) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if pm.running {
		return fmt.Errorf("process already running")
	}

	pm.stopChan = make(chan struct{})
	pm.waitDone = make(chan error, 1)

	pm.logger.Info("Starting MCP server: %s %v", pm.command, pm.args)

	resolved, err := resolveExecutable(pm.command)
	if err != nil {
		return err
	}

	// Create command with context
	pm.process = exec.CommandContext(ctx, resolved, pm.args...)
	pm.process.Env = pm.env

	// Setup pipes
	pm.stdin, err = pm.process.StdinPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdin pipe: %w", err)
	}

	pm.stdout, err = pm.process.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	pm.stderr, err = pm.process.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the process
	if err := pm.process.Start(); err != nil {
		return fmt.Errorf("failed to start process: %w", err)
	}

	pm.running = true
	pm.logger.Info("MCP server started with PID: %d", pm.process.Process.Pid)

	// Monitor stderr in background
	async.Go(pm.logger, "mcp.monitorStderr", func() {
		pm.monitorStderr()
	})

	// Monitor process exit
	async.Go(pm.logger, "mcp.monitorExit", func() {
		pm.monitorExit()
	})

	return nil
}

func resolveExecutable(command string) (string, error) {
	trimmed := strings.TrimSpace(command)
	if trimmed == "" {
		return "", fmt.Errorf("command is required")
	}
	if strings.Contains(trimmed, "\x00") {
		return "", fmt.Errorf("command contains invalid characters")
	}

	resolved, err := exec.LookPath(trimmed)
	if err != nil {
		return "", fmt.Errorf("command not found: %w", err)
	}

	return resolved, nil
}

// Stop gracefully stops the MCP server process
func (pm *ProcessManager) Stop(timeout time.Duration) error {
	pm.mu.Lock()
	if !pm.running {
		pm.mu.Unlock()
		return nil
	}

	pm.logger.Info("Stopping MCP server (timeout: %v)", timeout)
	pm.running = false

	stopChan := pm.stopChan
	waitDone := pm.waitDone
	process := pm.process
	stdin := pm.stdin
	pm.mu.Unlock()

	// Close stop channel to signal monitoring goroutines
	if stopChan != nil {
		close(stopChan)
	}

	// Try graceful shutdown first by closing stdin
	if stdin != nil {
		_ = stdin.Close()
	}

	if waitDone == nil {
		waitDone = make(chan error, 1)
		if process != nil {
			async.Go(pm.logger, "mcp.waitProcess", func() {
				waitDone <- process.Wait()
			})
		}
	}

	// Wait for process to exit with timeout
	select {
	case err := <-waitDone:
		pm.logger.Info("Process exited gracefully: %v", err)
		return nil
	case <-time.After(timeout):
		// Timeout - force kill
		pm.logger.Warn("Graceful shutdown timeout, killing process")
		if process != nil && process.Process != nil {
			if err := process.Process.Kill(); err != nil {
				return fmt.Errorf("failed to kill process: %w", err)
			}
		}
		return nil
	}
}

// Restart restarts the MCP server process with exponential backoff
func (pm *ProcessManager) Restart(ctx context.Context, maxAttempts int) error {
	pm.logger.Info("Restarting MCP server (max attempts: %d)", maxAttempts)

	// Stop current process
	if err := pm.Stop(5 * time.Second); err != nil {
		pm.logger.Error("Failed to stop process before restart: %v", err)
	}

	// Exponential backoff: 1s, 2s, 4s, 8s, 16s
	backoff := time.Second
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		pm.logger.Info("Restart attempt %d/%d (backoff: %v)", attempt, maxAttempts, backoff)

		// Wait for backoff period
		select {
		case <-ctx.Done():
			return fmt.Errorf("restart cancelled: %w", ctx.Err())
		case <-time.After(backoff):
		}

		// Try to start
		if err := pm.Start(ctx); err != nil {
			pm.logger.Error("Restart attempt %d failed: %v", attempt, err)
			backoff *= 2
			if backoff > 16*time.Second {
				backoff = 16 * time.Second
			}
			continue
		}

		pm.logger.Info("MCP server restarted successfully on attempt %d", attempt)
		return nil
	}

	return fmt.Errorf("failed to restart after %d attempts", maxAttempts)
}

// IsRunning checks if the process is currently running
func (pm *ProcessManager) IsRunning() bool {
	pm.mu.Lock()
	defer pm.mu.Unlock()
	return pm.running
}

// Write sends data to the process stdin
func (pm *ProcessManager) Write(data []byte) error {
	pm.mu.Lock()
	defer pm.mu.Unlock()

	if !pm.running {
		return fmt.Errorf("process not running")
	}

	if pm.stdin == nil {
		return fmt.Errorf("stdin not available")
	}

	n, err := pm.stdin.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write to stdin: %w", err)
	}

	if n != len(data) {
		return fmt.Errorf("incomplete write: wrote %d/%d bytes", n, len(data))
	}

	return nil
}

// ReadLine reads a line from the process stdout
func (pm *ProcessManager) ReadLine() ([]byte, error) {
	if !pm.running {
		return nil, fmt.Errorf("process not running")
	}

	if pm.stdout == nil {
		return nil, fmt.Errorf("stdout not available")
	}

	reader := bufio.NewReader(pm.stdout)
	line, err := reader.ReadBytes('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read from stdout: %w", err)
	}

	return line, nil
}

// GetStdout returns the stdout reader
func (pm *ProcessManager) GetStdout() io.ReadCloser {
	return pm.stdout
}

// monitorStderr logs stderr output from the process
func (pm *ProcessManager) monitorStderr() {
	if pm.stderr == nil {
		return
	}

	scanner := bufio.NewScanner(pm.stderr)
	for scanner.Scan() {
		select {
		case <-pm.stopChan:
			return
		default:
			line := scanner.Text()
			pm.logger.Debug("[STDERR] %s", line)
		}
	}

	if err := scanner.Err(); err != nil && err != io.EOF {
		pm.logger.Error("Error reading stderr: %v", err)
	}
}

// monitorExit monitors when the process exits unexpectedly
func (pm *ProcessManager) monitorExit() {
	if pm.process == nil {
		return
	}

	err := pm.process.Wait()

	select {
	case pm.waitDone <- err:
	default:
	}

	pm.mu.Lock()
	wasRunning := pm.running
	pm.running = false
	pm.mu.Unlock()

	if wasRunning {
		if err != nil {
			pm.logger.Error("Process exited unexpectedly: %v", err)
		} else {
			pm.logger.Warn("Process exited unexpectedly (no error)")
		}

		// Signal that restart is needed
		select {
		case pm.restartChan <- struct{}{}:
		default:
		}
	}
}

// RestartChannel returns the restart notification channel
func (pm *ProcessManager) RestartChannel() <-chan struct{} {
	return pm.restartChan
}
