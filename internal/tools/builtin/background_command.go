package builtin

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"alex/internal/utils"
)

// CommandStatus represents the current status of a background command
type CommandStatus string

const (
	StatusRunning   CommandStatus = "running"
	StatusCompleted CommandStatus = "completed"
	StatusFailed    CommandStatus = "failed"
	StatusTimedOut  CommandStatus = "timed_out"
	StatusKilled    CommandStatus = "killed"
)

// CommandStats holds statistics about command execution
type CommandStats struct {
	OutputLines   int           `json:"output_lines"`
	ExecutionTime time.Duration `json:"execution_time"`
	LastActivity  time.Time     `json:"last_activity"`
	OutputSize    int64         `json:"output_size"`
}

// CircularBuffer maintains a circular buffer of recent output lines
type CircularBuffer struct {
	lines    []string
	maxLines int
	current  int
	full     bool
	mutex    sync.RWMutex
}

func NewCircularBuffer(maxLines int) *CircularBuffer {
	return &CircularBuffer{
		lines:    make([]string, maxLines),
		maxLines: maxLines,
	}
}

func (cb *CircularBuffer) Add(line string) {
	cb.mutex.Lock()
	defer cb.mutex.Unlock()

	cb.lines[cb.current] = line
	cb.current = (cb.current + 1) % cb.maxLines
	if cb.current == 0 {
		cb.full = true
	}
}

func (cb *CircularBuffer) GetRecentLines(n int) []string {
	cb.mutex.RLock()
	defer cb.mutex.RUnlock()

	if !cb.full && cb.current <= n {
		return append([]string{}, cb.lines[:cb.current]...)
	}

	// Construct the most recent n lines
	var result []string
	start := cb.current - n
	if start < 0 {
		start = cb.maxLines + start
		result = append(result, cb.lines[start:]...)
		result = append(result, cb.lines[:cb.current]...)
	} else {
		result = append(result, cb.lines[start:cb.current]...)
	}
	return result
}

// ProgressDisplay manages progress display for background commands
type ProgressDisplay struct {
	outputBuffer   *CircularBuffer
	statsCollector *CommandStats
	lastUpdateTime time.Time
	updateInterval time.Duration
}

func NewProgressDisplay() *ProgressDisplay {
	return &ProgressDisplay{
		outputBuffer:   NewCircularBuffer(100), // Keep last 100 lines
		statsCollector: &CommandStats{},
		updateInterval: 2 * time.Second,
	}
}

// BackgroundCommand represents a command running in the background
type BackgroundCommand struct {
	ID         string
	Command    string
	StartTime  time.Time
	Status     CommandStatus
	WorkingDir string

	// Process control
	cmd    *exec.Cmd
	ctx    context.Context
	cancel context.CancelFunc

	// Output collection
	outputChan      chan string
	fullOutput      strings.Builder
	progressDisplay *ProgressDisplay

	// Progress callback
	callback       utils.StreamCallback
	timeoutSeconds int

	// Decision state for timeout handling
	timeoutDecisionPending bool
	timeoutDecisionMessage string

	mutex sync.RWMutex
}

// BackgroundCommandManager manages all background commands
type BackgroundCommandManager struct {
	commands map[string]*BackgroundCommand
	mutex    sync.RWMutex
	cleanup  *time.Ticker
}

var (
	bgManager *BackgroundCommandManager
	once      sync.Once
)

// GetBackgroundCommandManager returns the singleton background command manager
func GetBackgroundCommandManager() *BackgroundCommandManager {
	once.Do(func() {
		bgManager = &BackgroundCommandManager{
			commands: make(map[string]*BackgroundCommand),
			cleanup:  time.NewTicker(30 * time.Second), // Cleanup every 30 seconds
		}
		go bgManager.cleanupLoop()
	})
	return bgManager
}

func (bcm *BackgroundCommandManager) Register(id string, cmd *BackgroundCommand) {
	bcm.mutex.Lock()
	defer bcm.mutex.Unlock()
	bcm.commands[id] = cmd
}

func (bcm *BackgroundCommandManager) Get(id string) *BackgroundCommand {
	bcm.mutex.RLock()
	defer bcm.mutex.RUnlock()
	return bcm.commands[id]
}

func (bcm *BackgroundCommandManager) List() []*BackgroundCommand {
	bcm.mutex.RLock()
	defer bcm.mutex.RUnlock()

	var cmds []*BackgroundCommand
	for _, cmd := range bcm.commands {
		cmds = append(cmds, cmd)
	}
	return cmds
}

func (bcm *BackgroundCommandManager) Remove(id string) {
	bcm.mutex.Lock()
	defer bcm.mutex.Unlock()
	delete(bcm.commands, id)
}

func (bcm *BackgroundCommandManager) cleanupLoop() {
	for range bcm.cleanup.C {
		bcm.performCleanup()
	}
}

func (bcm *BackgroundCommandManager) performCleanup() {
	bcm.mutex.Lock()
	defer bcm.mutex.Unlock()

	for id, cmd := range bcm.commands {
		// Clean up completed commands older than 10 minutes
		if cmd.Status != StatusRunning && time.Since(cmd.StartTime) > 10*time.Minute {
			cmd.cleanup()
			delete(bcm.commands, id)
			continue
		}

		// Check for zombie processes
		if cmd.Status == StatusRunning && cmd.cmd != nil && cmd.cmd.ProcessState != nil && cmd.cmd.ProcessState.Exited() {
			cmd.setStatus(StatusCompleted)
		}
	}
}

// generateExecutionID creates a unique execution ID
func generateExecutionID() string {
	timestamp := time.Now().Unix()
	return fmt.Sprintf("bg_%d_%04d", timestamp, time.Now().Nanosecond()%10000)
}

// NewBackgroundCommand creates a new background command instance
func NewBackgroundCommand(command string, workingDir string, timeout int, callback utils.StreamCallback) *BackgroundCommand {
	ctx, cancel := context.WithCancel(context.Background())

	bc := &BackgroundCommand{
		ID:              generateExecutionID(),
		Command:         command,
		StartTime:       time.Now(),
		Status:          StatusRunning,
		WorkingDir:      workingDir,
		ctx:             ctx,
		cancel:          cancel,
		outputChan:      make(chan string, 100),
		progressDisplay: NewProgressDisplay(),
		callback:        callback,
		timeoutSeconds:  timeout,
	}

	return bc
}

// Start begins execution of the background command
func (bc *BackgroundCommand) Start() error {
	// Create command context with timeout
	cmdCtx, cmdCancel := context.WithTimeout(bc.ctx, time.Duration(bc.timeoutSeconds)*time.Second)

	// Determine shell command based on OS
	var cmd *exec.Cmd
	if runtime.GOOS == "windows" {
		cmd = exec.CommandContext(cmdCtx, "cmd", "/C", bc.Command)
	} else {
		cmd = exec.CommandContext(cmdCtx, "sh", "-c", bc.Command)
	}

	// Set working directory if specified
	if bc.WorkingDir != "" {
		cmd.Dir = bc.WorkingDir
	}

	bc.cmd = cmd

	// Setup stdout and stderr pipes
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		cmdCancel()
		return fmt.Errorf("failed to create stdout pipe: %w", err)
	}

	stderr, err := cmd.StderrPipe()
	if err != nil {
		cmdCancel()
		return fmt.Errorf("failed to create stderr pipe: %w", err)
	}

	// Start the command
	if err := cmd.Start(); err != nil {
		cmdCancel()
		return fmt.Errorf("failed to start command: %w", err)
	}

	// Start output collection goroutines
	go bc.collectOutput(stdout, "stdout")
	go bc.collectOutput(stderr, "stderr")
	go bc.keepAlive()
	go bc.timeoutChecker(cmdCtx, cmdCancel)

	// Wait for command completion in background
	go func() {
		defer cmdCancel()
		err := cmd.Wait()
		if err != nil {
			if cmdCtx.Err() == context.DeadlineExceeded {
				bc.setStatus(StatusTimedOut)
				bc.sendTimeoutDecision()
			} else {
				bc.setStatus(StatusFailed)
			}
		} else {
			bc.setStatus(StatusCompleted)
		}
		close(bc.outputChan)
		bc.sendFinalStatus()
	}()

	return nil
}

func (bc *BackgroundCommand) collectOutput(reader io.Reader, source string) {
	scanner := bufio.NewScanner(reader)
	for scanner.Scan() {
		line := scanner.Text()

		// Add timestamp prefix for stderr
		if source == "stderr" {
			line = "[STDERR] " + line
		}

		// Update statistics
		bc.mutex.Lock()
		bc.fullOutput.WriteString(line + "\n")
		bc.progressDisplay.outputBuffer.Add(line)
		bc.progressDisplay.statsCollector.OutputLines++
		bc.progressDisplay.statsCollector.LastActivity = time.Now()
		bc.progressDisplay.statsCollector.OutputSize += int64(len(line))
		bc.mutex.Unlock()

		// Send to channel for real-time processing
		select {
		case bc.outputChan <- line:
		case <-bc.ctx.Done():
			return
		}

		// Send important output immediately
		if bc.isImportantOutput(line) {
			bc.sendImportantOutput(line)
		}
	}
}

func (bc *BackgroundCommand) keepAlive() {
	ticker := time.NewTicker(bc.calculateUpdateInterval())
	defer ticker.Stop()

	for {
		select {
		case <-bc.ctx.Done():
			return
		case <-ticker.C:
			bc.sendProgressUpdate()
			// Adjust ticker interval based on execution time
			ticker.Reset(bc.calculateUpdateInterval())
		case line := <-bc.outputChan:
			// Process output line (already handled in collectOutput)
			_ = line
		}
	}
}

func (bc *BackgroundCommand) calculateUpdateInterval() time.Duration {
	elapsed := time.Since(bc.StartTime)

	switch {
	case elapsed < 10*time.Second:
		return 1 * time.Second // First 10 seconds: update every second
	case elapsed < 60*time.Second:
		return 3 * time.Second // First minute: update every 3 seconds
	case elapsed < 300*time.Second:
		return 10 * time.Second // First 5 minutes: update every 10 seconds
	default:
		return 30 * time.Second // After 5 minutes: update every 30 seconds
	}
}

func (bc *BackgroundCommand) timeoutChecker(cmdCtx context.Context, cmdCancel context.CancelFunc) {
	<-cmdCtx.Done()
	if cmdCtx.Err() == context.DeadlineExceeded {
		bc.setStatus(StatusTimedOut)
	}
}

func (bc *BackgroundCommand) sendProgressUpdate() {
	if bc.callback == nil {
		return
	}

	now := time.Now()

	// Check if we should send update based on interval
	if now.Sub(bc.progressDisplay.lastUpdateTime) < bc.progressDisplay.updateInterval {
		return
	}

	bc.progressDisplay.lastUpdateTime = now
	bc.progressDisplay.statsCollector.ExecutionTime = now.Sub(bc.StartTime)

	recentOutput := bc.progressDisplay.outputBuffer.GetRecentLines(5)
	progressContent := bc.formatProgressContent(recentOutput)

	bc.callback(utils.StreamChunk{
		Type:    "progress",
		Content: progressContent,
		Metadata: map[string]interface{}{
			"execution_id":   bc.ID,
			"status":         string(bc.Status),
			"execution_time": bc.progressDisplay.statsCollector.ExecutionTime.String(),
			"output_lines":   bc.progressDisplay.statsCollector.OutputLines,
			"last_activity":  bc.progressDisplay.statsCollector.LastActivity.Format("15:04:05"),
		},
	})
}

func (bc *BackgroundCommand) formatProgressContent(recentOutput []string) string {
	var content strings.Builder

	// Status line with execution time
	content.WriteString(fmt.Sprintf("🔄 [%s] 执行中... ⏱️ %v\n",
		bc.ID[:8], bc.progressDisplay.statsCollector.ExecutionTime.Truncate(time.Second)))

	// Statistics
	content.WriteString(fmt.Sprintf("📊 输出行数: %d | 最后活动: %s\n",
		bc.progressDisplay.statsCollector.OutputLines,
		bc.progressDisplay.statsCollector.LastActivity.Format("15:04:05")))

	// Recent output
	if len(recentOutput) > 0 {
		content.WriteString("📄 最新输出:\n")
		for _, line := range recentOutput {
			// Truncate long lines
			if len(line) > 100 {
				line = line[:97] + "..."
			}
			content.WriteString(fmt.Sprintf("  %s\n", line))
		}
	}

	return content.String()
}

func (bc *BackgroundCommand) isImportantOutput(line string) bool {
	importantKeywords := []string{
		"ERROR", "error", "Error",
		"WARNING", "warning", "Warning",
		"FAILED", "failed", "Failed",
		"SUCCESS", "success", "Success",
		"COMPLETED", "completed", "Completed",
		"progress", "Progress", "%",
	}

	lowerLine := strings.ToLower(line)
	for _, keyword := range importantKeywords {
		if strings.Contains(lowerLine, strings.ToLower(keyword)) {
			return true
		}
	}
	return false
}

func (bc *BackgroundCommand) sendImportantOutput(line string) {
	if bc.callback == nil {
		return
	}

	bc.callback(utils.StreamChunk{
		Type:    "command_output",
		Content: fmt.Sprintf("⚡ [%s] %s", bc.ID[:8], line),
		Metadata: map[string]interface{}{
			"execution_id": bc.ID,
			"important":    true,
			"timestamp":    time.Now().Format("15:04:05"),
		},
	})
}

func (bc *BackgroundCommand) sendTimeoutDecision() {
	currentOutput := bc.GetOutput()

	// Save timeout decision message to command state instead of sending via callback
	bc.mutex.Lock()
	bc.timeoutDecisionPending = true
	bc.timeoutDecisionMessage = fmt.Sprintf(`⏰ 命令执行超时！

📝 命令: %s
⏱️ 已运行: %v
📊 输出行数: %d
📄 当前输出预览:
%s

🤖 请使用bash_control工具进行决策:
• bash_control {"execution_id": "%s", "action": "extend_timeout", "seconds": 300} - 延长5分钟
• bash_control {"execution_id": "%s", "action": "terminate"} - 终止并获取结果
• bash_status {"execution_id": "%s"} - 查看详细状态`,
		bc.Command,
		time.Since(bc.StartTime).Truncate(time.Second),
		bc.progressDisplay.statsCollector.OutputLines,
		bc.truncateOutput(currentOutput, 10),
		bc.ID, bc.ID, bc.ID)
	bc.mutex.Unlock()

	// Also send via callback if available (for compatibility)
	if bc.callback != nil {
		bc.callback(utils.StreamChunk{
			Type:    "timeout_decision",
			Content: bc.timeoutDecisionMessage,
			Metadata: map[string]interface{}{
				"execution_id": bc.ID,
				"command":      bc.Command,
				"output_lines": bc.progressDisplay.statsCollector.OutputLines,
			},
		})
	}
}

func (bc *BackgroundCommand) sendFinalStatus() {
	if bc.callback == nil {
		return
	}

	var content string
	switch bc.Status {
	case StatusCompleted:
		content = fmt.Sprintf("✅ [%s] 命令执行完成", bc.ID[:8])
	case StatusFailed:
		content = fmt.Sprintf("❌ [%s] 命令执行失败", bc.ID[:8])
	case StatusTimedOut:
		content = fmt.Sprintf("⏰ [%s] 命令执行超时", bc.ID[:8])
	case StatusKilled:
		content = fmt.Sprintf("🛑 [%s] 命令被终止", bc.ID[:8])
	default:
		content = fmt.Sprintf("ℹ️ [%s] 命令状态: %s", bc.ID[:8], bc.Status)
	}

	bc.callback(utils.StreamChunk{
		Type:    "command_status",
		Content: content,
		Metadata: map[string]interface{}{
			"execution_id":   bc.ID,
			"status":         string(bc.Status),
			"execution_time": time.Since(bc.StartTime).String(),
			"final":          true,
		},
	})
}

// GetOutput returns the full command output
func (bc *BackgroundCommand) GetOutput() string {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.fullOutput.String()
}

// GetStats returns current command statistics
func (bc *BackgroundCommand) GetStats() *CommandStats {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()

	stats := *bc.progressDisplay.statsCollector
	stats.ExecutionTime = time.Since(bc.StartTime)
	return &stats
}

// Terminate forcefully stops the background command
func (bc *BackgroundCommand) Terminate() error {
	bc.setStatus(StatusKilled)

	if bc.cmd != nil && bc.cmd.Process != nil {
		// Try graceful termination first
		if err := bc.cmd.Process.Signal(os.Interrupt); err != nil {
			// Force kill if graceful termination fails
			return bc.cmd.Process.Kill()
		}
	}

	bc.cancel()
	return nil
}

// ExtendTimeout extends the command timeout
func (bc *BackgroundCommand) ExtendTimeout(duration time.Duration) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.timeoutSeconds += int(duration.Seconds())
}

func (bc *BackgroundCommand) setStatus(status CommandStatus) {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.Status = status
}

func (bc *BackgroundCommand) cleanup() {
	if bc.cancel != nil {
		bc.cancel()
	}
	if bc.outputChan != nil {
		close(bc.outputChan)
	}
}

// truncateOutput returns a truncated version of output for display
func (bc *BackgroundCommand) truncateOutput(output string, maxLines int) string {
	lines := strings.Split(output, "\n")
	if len(lines) <= maxLines {
		return output
	}

	// Show last maxLines
	result := strings.Join(lines[len(lines)-maxLines:], "\n")
	return fmt.Sprintf("... (显示最后 %d 行，共 %d 行)\n%s", maxLines, len(lines), result)
}

// GetTimeoutDecision returns the timeout decision message if pending
func (bc *BackgroundCommand) GetTimeoutDecision() (bool, string) {
	bc.mutex.RLock()
	defer bc.mutex.RUnlock()
	return bc.timeoutDecisionPending, bc.timeoutDecisionMessage
}

// ClearTimeoutDecision clears the timeout decision state
func (bc *BackgroundCommand) ClearTimeoutDecision() {
	bc.mutex.Lock()
	defer bc.mutex.Unlock()
	bc.timeoutDecisionPending = false
	bc.timeoutDecisionMessage = ""
}
