package builtin

import (
	"bufio"
	"context"
	"fmt"
	"os/exec"
	"sync"
	"time"

	"alex/internal/utils"
)

// SimplifiedBackgroundCommand provides a cleaner, simpler background command execution
// This replaces the overly complex context management in background_command.go
type SimplifiedBackgroundCommand struct {
	ID         string
	Command    string
	Args       []string
	WorkingDir string

	// Execution state
	Status    CommandStatus
	StartTime time.Time
	EndTime   time.Time
	ExitCode  int
	Output    []string

	// Context management - simplified to standard patterns
	ctx    context.Context
	cancel context.CancelFunc
	cmd    *exec.Cmd

	// Thread safety
	mutex sync.RWMutex

	// Progress callback
	callback utils.StreamCallback
}

// NewSimplifiedBackgroundCommand creates a new simplified background command
func NewSimplifiedBackgroundCommand(command string, args []string, workingDir string, timeoutSeconds int) *SimplifiedBackgroundCommand {
	// Create context with timeout using standard pattern
	ctx, cancel := context.WithTimeout(context.Background(), time.Duration(timeoutSeconds)*time.Second)

	return &SimplifiedBackgroundCommand{
		ID:         generateCommandID(),
		Command:    command,
		Args:       args,
		WorkingDir: workingDir,
		Status:     StatusRunning,
		StartTime:  time.Now(),
		Output:     make([]string, 0),
		ctx:        ctx,
		cancel:     cancel,
	}
}

// Start begins command execution in the background
func (sbc *SimplifiedBackgroundCommand) Start(callback utils.StreamCallback) error {
	sbc.mutex.Lock()
	defer sbc.mutex.Unlock()

	sbc.callback = callback

	// Create command with the timeout context
	sbc.cmd = exec.CommandContext(sbc.ctx, sbc.Command, sbc.Args...)
	if sbc.WorkingDir != "" {
		sbc.cmd.Dir = sbc.WorkingDir
	}

	// Set up stdout pipe for real-time output capture
	stdout, err := sbc.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := sbc.cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := sbc.cmd.Start(); err != nil {
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Start output capture goroutines
	go sbc.captureOutput(stdout, "stdout")
	go sbc.captureOutput(stderr, "stderr")

	// Start monitoring goroutine
	go sbc.monitor()

	return nil
}

// captureOutput captures output from stdout/stderr in real-time
func (sbc *SimplifiedBackgroundCommand) captureOutput(pipe interface{}, source string) {
	scanner := bufio.NewScanner(pipe.(interface{ Read([]byte) (int, error) }))

	for scanner.Scan() {
		line := scanner.Text()

		sbc.mutex.Lock()
		sbc.Output = append(sbc.Output, fmt.Sprintf("[%s] %s", source, line))
		sbc.mutex.Unlock()

		// Send progress update
		if sbc.callback != nil {
			sbc.callback(utils.StreamChunk{
				Type:    "progress",
				Content: line,
				Metadata: map[string]interface{}{
					"command_id": sbc.ID,
					"source":     source,
					"timestamp":  time.Now(),
				},
			})
		}
	}
}

// monitor handles command completion and cleanup
func (sbc *SimplifiedBackgroundCommand) monitor() {
	defer sbc.cancel() // Ensure cleanup

	// Wait for command completion
	err := sbc.cmd.Wait()

	sbc.mutex.Lock()
	defer sbc.mutex.Unlock()

	sbc.EndTime = time.Now()

	// Determine final status
	if sbc.ctx.Err() == context.DeadlineExceeded {
		sbc.Status = StatusTimedOut
		sbc.ExitCode = -1
	} else if err != nil {
		sbc.Status = StatusFailed
		if exitError, ok := err.(*exec.ExitError); ok {
			sbc.ExitCode = exitError.ExitCode()
		} else {
			sbc.ExitCode = -1
		}
	} else {
		sbc.Status = StatusCompleted
		sbc.ExitCode = 0
	}

	// Send final status update
	if sbc.callback != nil {
		sbc.callback(utils.StreamChunk{
			Type:    "completion",
			Content: fmt.Sprintf("Command finished with status: %s", sbc.Status),
			Metadata: map[string]interface{}{
				"command_id":     sbc.ID,
				"status":         string(sbc.Status),
				"exit_code":      sbc.ExitCode,
				"execution_time": sbc.EndTime.Sub(sbc.StartTime).String(),
			},
		})
	}
}

// Stop forcefully terminates the command
func (sbc *SimplifiedBackgroundCommand) Stop() error {
	sbc.mutex.Lock()
	defer sbc.mutex.Unlock()

	if sbc.Status != StatusRunning {
		return fmt.Errorf("command is not running")
	}

	// Cancel context - this will trigger cleanup
	sbc.cancel()

	// Kill process if it's still running
	if sbc.cmd != nil && sbc.cmd.Process != nil {
		if err := sbc.cmd.Process.Kill(); err != nil {
			return fmt.Errorf("failed to kill process: %w", err)
		}
	}

	sbc.Status = StatusKilled
	sbc.EndTime = time.Now()

	return nil
}

// GetStatus returns the current command status
func (sbc *SimplifiedBackgroundCommand) GetStatus() CommandStatus {
	sbc.mutex.RLock()
	defer sbc.mutex.RUnlock()
	return sbc.Status
}

// GetOutput returns all captured output
func (sbc *SimplifiedBackgroundCommand) GetOutput() []string {
	sbc.mutex.RLock()
	defer sbc.mutex.RUnlock()

	// Return a copy to prevent race conditions
	output := make([]string, len(sbc.Output))
	copy(output, sbc.Output)
	return output
}

// GetStats returns execution statistics
func (sbc *SimplifiedBackgroundCommand) GetStats() map[string]interface{} {
	sbc.mutex.RLock()
	defer sbc.mutex.RUnlock()

	var executionTime time.Duration
	if sbc.EndTime.IsZero() {
		executionTime = time.Since(sbc.StartTime)
	} else {
		executionTime = sbc.EndTime.Sub(sbc.StartTime)
	}

	return map[string]interface{}{
		"command_id":     sbc.ID,
		"status":         string(sbc.Status),
		"exit_code":      sbc.ExitCode,
		"execution_time": executionTime.String(),
		"start_time":     sbc.StartTime,
		"end_time":       sbc.EndTime,
		"output_lines":   len(sbc.Output),
	}
}

// IsRunning returns true if the command is currently running
func (sbc *SimplifiedBackgroundCommand) IsRunning() bool {
	return sbc.GetStatus() == StatusRunning
}

// generateCommandID generates a unique ID for the command
func generateCommandID() string {
	return fmt.Sprintf("cmd_%d", time.Now().UnixNano())
}

// Cleanup performs any necessary cleanup
func (sbc *SimplifiedBackgroundCommand) Cleanup() {
	sbc.mutex.Lock()
	defer sbc.mutex.Unlock()

	if sbc.cancel != nil {
		sbc.cancel()
	}
}
