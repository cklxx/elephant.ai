package bridge

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"os"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/process"
	"alex/internal/shared/utils"
)

const defaultAttachedTimeout = 4 * time.Hour

// handleProcessError records a role failure and returns a formatted error.
func (e *Executor) handleProcessError(sink *runtimeEventSink, result *agent.ExternalAgentResult, taskID string, err error, stderrTail string) (*agent.ExternalAgentResult, error) {
	errMsg := formatProcessError(e.cfg.AgentType, err, stderrTail)
	sink.record("role_failed", map[string]any{
		"task_id":     taskID,
		"error":       errMsg,
		"stderr_tail": compactTail(stderrTail, 400),
	})
	return result, errors.New(e.maybeAppendAuthHint(errMsg, stderrTail))
}

// executeAttached runs the bridge with stdout piped back to this process.
func (e *Executor) executeAttached(ctx context.Context, req agent.ExternalAgentRequest, bcfg bridgeConfig, pythonBin, bridgeScript string, env map[string]string) (*agent.ExternalAgentResult, error) {
	sink := newRuntimeEventSink(req)
	sink.record("role_started", map[string]any{
		"task_id":    req.TaskID,
		"agent_type": req.AgentType,
		"binary":     bcfg.Binary,
		"mode":       bcfg.ExecutionMode,
	})

	timeout := e.cfg.Timeout
	if timeout <= 0 {
		timeout = defaultAttachedTimeout
	}
	proc := e.subprocessFactory(process.ProcessConfig{
		Name:       fmt.Sprintf("bridge-%s-%s", e.cfg.AgentType, req.TaskID),
		Command:    pythonBin,
		Args:       []string{bridgeScript},
		Env:        env,
		WorkingDir: req.WorkingDir,
		Timeout:    timeout,
	})
	if err := proc.Start(ctx); err != nil {
		return nil, fmt.Errorf("start bridge: %w", err)
	}
	defer func() { _ = proc.Stop() }()

	// Kill subprocess when caller cancels context, unblocking the scanner loop.
	go func() {
		select {
		case <-ctx.Done():
			_ = proc.Stop()
		case <-proc.Done():
		}
	}()

	if err := writeBridgeConfig(proc, bcfg); err != nil {
		return nil, err
	}

	result := &agent.ExternalAgentResult{}
	scanner := bufio.NewScanner(proc.Stdout())
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		ev, err := ParseSDKEvent(scanner.Bytes())
		if err != nil {
			continue
		}
		e.applyEvent(ev, result, req.OnProgress)
		sink.recordFromSDK(ev)
		if ev.Type == SDKEventError {
			sink.record("role_failed", map[string]any{
				"task_id": req.TaskID,
				"error":   ev.Message,
			})
			return result, errors.New(ev.Message)
		}
	}
	// When context is cancelled the watchdog kills the subprocess, closing
	// the stdout pipe. scanner.Err() / proc.Wait() will report pipe or
	// signal errors that are just side effects of the cancellation — suppress
	// them so the caller sees a clean context.Canceled instead.
	if err := scanner.Err(); err != nil && ctx.Err() == nil {
		sink.record("role_failed", map[string]any{
			"task_id": req.TaskID,
			"error":   err.Error(),
		})
		return result, err
	}
	if err := proc.Wait(); err != nil && ctx.Err() == nil {
		return e.handleProcessError(sink, result, req.TaskID, err, proc.StderrTail())
	}
	enrichPlanMetadata(result, bcfg.ExecutionMode)
	sink.record("role_completed", map[string]any{
		"task_id": req.TaskID,
		"status":  "completed",
	})
	return result, nil
}

// executeDetached runs the bridge in detached mode: output goes to a file,
// subprocess becomes a session leader that survives parent death.
//
// Note: bridges require stdin for config delivery, which tmux cannot provide.
// Detached bridges always use the exec backend (Setsid session leader).
func (e *Executor) executeDetached(ctx context.Context, req agent.ExternalAgentRequest, bcfg bridgeConfig, pythonBin, bridgeScript string, env map[string]string) (*agent.ExternalAgentResult, error) {
	sink := newRuntimeEventSink(req)
	sink.record("role_started", map[string]any{
		"task_id":    req.TaskID,
		"agent_type": req.AgentType,
		"binary":     bcfg.Binary,
		"mode":       bcfg.ExecutionMode,
	})

	workDir := req.WorkingDir
	if workDir == "" {
		workDir = "."
	}
	taskID := req.TaskID

	outDir := bridgeOutputDir(workDir, taskID)
	if err := os.MkdirAll(outDir, 0o755); err != nil {
		return nil, fmt.Errorf("create bridge output dir: %w", err)
	}

	outputFile := bridgeOutputFile(workDir, taskID)
	statusFile := bridgeStatusFile(workDir, taskID)
	doneFile := bridgeDoneFile(workDir, taskID)

	proc := e.subprocessFactory(process.ProcessConfig{
		Name:       fmt.Sprintf("bridge-%s-%s", e.cfg.AgentType, taskID),
		Command:    pythonBin,
		Args:       []string{bridgeScript, "--output-file", outputFile},
		Env:        env,
		WorkingDir: workDir,
		Timeout:    e.cfg.Timeout,
		Detached:   true,
		OutputFile: outputFile,
		StatusFile: statusFile,
	})
	if err := proc.Start(ctx); err != nil {
		return nil, fmt.Errorf("start detached bridge: %w", err)
	}

	if err := writeBridgeConfig(proc, bcfg); err != nil {
		_ = proc.Stop()
		return nil, err
	}

	// Notify caller of bridge meta if available.
	if req.OnBridgeStarted != nil {
		req.OnBridgeStarted(BridgeStartedInfo{
			PID:        proc.PID(),
			OutputFile: outputFile,
			TaskID:     taskID,
		})
	}

	// Tail the output file for events.
	reader := NewOutputReader(outputFile, doneFile)
	events := reader.Read(ctx)

	result := &agent.ExternalAgentResult{}
	var lastErr error

	for ev := range events {
		e.applyEvent(ev, result, req.OnProgress)
		sink.recordFromSDK(ev)
		if ev.Type == SDKEventError {
			lastErr = errors.New(ev.Message)
		}
	}

	// Wait for process to finish (may already be done).
	if procDone := proc.Done(); procDone != nil {
		select {
		case <-procDone:
		case <-time.After(5 * time.Second):
			// Process hung after done sentinel — force kill.
			_ = proc.Stop()
		}
	}

	if lastErr != nil {
		sink.record("role_failed", map[string]any{
			"task_id": req.TaskID,
			"error":   lastErr.Error(),
		})
		return result, lastErr
	}
	if err := proc.Wait(); err != nil {
		return e.handleProcessError(sink, result, req.TaskID, err, proc.StderrTail())
	}
	enrichPlanMetadata(result, bcfg.ExecutionMode)
	sink.record("role_completed", map[string]any{
		"task_id": req.TaskID,
		"status":  "completed",
	})
	return result, nil
}

// applyEvent updates the result and fires progress callbacks for a single event.
func (e *Executor) applyEvent(ev SDKEvent, result *agent.ExternalAgentResult, onProgress func(agent.ExternalAgentProgress)) {
	switch ev.Type {
	case SDKEventTool:
		result.Iterations = ev.Iter
		if onProgress != nil {
			onProgress(agent.ExternalAgentProgress{
				Iteration:    ev.Iter,
				TokensUsed:   result.TokensUsed,
				CostUSD:      extractCostFromMeta(result.Metadata),
				CurrentTool:  ev.ToolName,
				CurrentArgs:  ev.Summary,
				FilesTouched: ev.Files,
				LastActivity: time.Now(),
			})
		}
	case SDKEventResult:
		result.Answer = ev.Answer
		result.TokensUsed = ev.Tokens
		result.Iterations = ev.Iters
		if ev.IsError {
			result.Error = ev.Answer
		}
		result.Metadata = map[string]any{
			"cost_usd": ev.Cost,
		}
	}
}

// BridgeStartedInfo is passed to OnBridgeStarted when a detached bridge launches.
type BridgeStartedInfo struct {
	PID        int
	OutputFile string
	TaskID     string
}

// BridgePID implements task.BridgeInfoProvider.
func (b BridgeStartedInfo) BridgePID() int { return b.PID }

// BridgeOutputFile implements task.BridgeInfoProvider.
func (b BridgeStartedInfo) BridgeOutputFile() string { return b.OutputFile }

// extractCostFromMeta extracts the cost_usd value from result metadata.
func extractCostFromMeta(meta map[string]any) float64 {
	if meta == nil {
		return 0
	}
	if v, ok := meta["cost_usd"].(float64); ok {
		return v
	}
	return 0
}

// enrichPlanMetadata adds plan-specific metadata to the result when in plan mode.
func enrichPlanMetadata(result *agent.ExternalAgentResult, executionMode string) {
	if result == nil || executionMode != "plan" {
		return
	}
	if result.Metadata == nil {
		result.Metadata = make(map[string]any)
	}
	if utils.HasContent(result.Answer) {
		result.Metadata["plan"] = result.Answer
	}
}
