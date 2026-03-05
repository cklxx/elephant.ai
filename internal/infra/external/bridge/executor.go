package bridge

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"

	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/process"
	"alex/internal/shared/executioncontrol"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// BridgeConfig configures a bridge executor for any supported agent type.
type BridgeConfig struct {
	AgentType    string // "claude_code", "codex", or "kimi"
	Binary       string // Agent CLI binary name/path (primarily for codex bridge).
	PythonBinary string
	BridgeScript string

	// Claude Code fields.
	APIKey                 string
	DefaultModel           string
	DefaultMode            string
	AutonomousAllowedTools []string
	PlanAllowedTools       []string
	MaxBudgetUSD           float64
	MaxTurns               int

	// Codex fields.
	ApprovalPolicy     string
	Sandbox            string
	PlanApprovalPolicy string
	PlanSandbox        string

	// Common fields.
	Timeout       time.Duration
	Env           map[string]string
	ResumeEnabled bool

	// Detached mode: subprocess survives parent death.
	// When true, bridge output goes to a file instead of stdout pipe.
	Detached bool
}

// bridgeRunner abstracts subprocess lifecycle for testability.
type bridgeRunner interface {
	Start(ctx context.Context) error
	Write(data []byte) error
	Stdout() interface{ Read([]byte) (int, error) }
	StderrTail() string
	Wait() error
	Stop() error
	PID() int
	Done() <-chan struct{}
}

// pipedHandleAdapter adapts process.PipedHandle to bridgeRunner.
type pipedHandleAdapter struct {
	h process.PipedHandle
}

func (a *pipedHandleAdapter) Start(_ context.Context) error { return nil /* already started */ }
func (a *pipedHandleAdapter) Write(data []byte) error {
	if a.h.Stdin() == nil {
		return fmt.Errorf("stdin not available")
	}
	_, err := a.h.Stdin().Write(data)
	return err
}
func (a *pipedHandleAdapter) Stdout() interface{ Read([]byte) (int, error) } {
	return a.h.Stdout()
}
func (a *pipedHandleAdapter) StderrTail() string    { return a.h.StderrTail() }
func (a *pipedHandleAdapter) Wait() error           { return a.h.Wait() }
func (a *pipedHandleAdapter) Stop() error           { return a.h.Stop() }
func (a *pipedHandleAdapter) PID() int              { return a.h.PID() }
func (a *pipedHandleAdapter) Done() <-chan struct{} { return a.h.Done() }

// failRunner is returned when the controller fails to start a process.
type failRunner struct{ err error }

func (f *failRunner) Start(_ context.Context) error                  { return f.err }
func (f *failRunner) Write(_ []byte) error                           { return f.err }
func (f *failRunner) Stdout() interface{ Read([]byte) (int, error) } { return nil }
func (f *failRunner) StderrTail() string                             { return "" }
func (f *failRunner) Wait() error                                    { return f.err }
func (f *failRunner) Stop() error                                    { return nil }
func (f *failRunner) PID() int                                       { return 0 }
func (f *failRunner) Done() <-chan struct{}                          { return nil }

// Executor implements agent.InteractiveExternalExecutor by spawning a Python
// bridge sidecar and reading pre-filtered JSONL events from its stdout.
// It supports multiple agent types (claude_code, codex, kimi) through a single
// parameterised implementation.
type Executor struct {
	cfg               BridgeConfig
	ctrl              *process.Controller
	inputCh           chan agent.InputRequest
	pending           sync.Map
	logger            logging.Logger
	subprocessFactory func(process.ProcessConfig) bridgeRunner
}

// New creates a new bridge executor for the configured agent type.
// ctrl may be nil in tests where subprocessFactory is overridden.
func New(cfg BridgeConfig, ctrl *process.Controller) *Executor {
	e := &Executor{
		cfg:     cfg,
		ctrl:    ctrl,
		inputCh: make(chan agent.InputRequest, 32),
		logger:  logging.NewComponentLogger("BridgeExecutor/" + cfg.AgentType),
	}
	e.subprocessFactory = func(c process.ProcessConfig) bridgeRunner {
		if ctrl == nil {
			return &failRunner{err: fmt.Errorf("no process controller")}
		}
		h, err := ctrl.StartExec(context.Background(), c)
		if err != nil {
			return &failRunner{err: err}
		}
		return &pipedHandleAdapter{h: h}
	}
	return e
}

func (e *Executor) SupportedTypes() []string {
	return []string{e.cfg.AgentType}
}

func (e *Executor) InputRequests() <-chan agent.InputRequest {
	return e.inputCh
}

func (e *Executor) Reply(ctx context.Context, resp agent.InputResponse) error {
	key := requestKey(resp.TaskID, resp.RequestID)
	if chVal, ok := e.pending.Load(key); ok {
		ch := chVal.(chan agent.InputResponse)
		select {
		case ch <- resp:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	return fmt.Errorf("unknown request_id: %s", resp.RequestID)
}

// bridgeConfig is the JSON payload sent to any bridge script via stdin.
type bridgeConfig struct {
	Prompt       string   `json:"prompt"`
	Model        string   `json:"model,omitempty"`
	Mode         string   `json:"mode,omitempty"`
	MaxTurns     int      `json:"max_turns,omitempty"`
	MaxBudgetUSD float64  `json:"max_budget_usd,omitempty"`
	WorkingDir   string   `json:"working_dir,omitempty"`
	Binary       string   `json:"binary,omitempty"`
	AllowedTools []string `json:"allowed_tools,omitempty"`
	// Codex-specific fields.
	ApprovalPolicy string `json:"approval_policy,omitempty"`
	Sandbox        string `json:"sandbox,omitempty"`
	// Cross-agent execution controls.
	ExecutionMode string `json:"execution_mode,omitempty"`
	AutonomyLevel string `json:"autonomy_level,omitempty"`
}

func (e *Executor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if utils.IsBlank(req.Prompt) {
		return nil, fmt.Errorf("prompt is required")
	}

	// Block write dispatches on main (non-worktree) unless plan/read-only mode.
	execMode := normalizeExecutionMode(req.ExecutionMode, req.Config)
	if execMode != "plan" {
		if err := validateWorktreePolicy(req.WorkingDir); err != nil {
			return nil, err
		}
	}

	mode := pickString(req.Config, "mode", e.cfg.DefaultMode)
	model := pickString(req.Config, "model", e.cfg.DefaultModel)
	maxTurns := pickInt(req.Config, "max_turns", e.cfg.MaxTurns)
	maxBudget := pickFloat(req.Config, "max_budget_usd", e.cfg.MaxBudgetUSD)

	pythonBin := e.resolvePython()
	bridgeScript := e.resolveBridgeScript()

	bcfg := bridgeConfig{
		Prompt:        req.Prompt,
		Model:         model,
		Mode:          mode,
		MaxTurns:      maxTurns,
		MaxBudgetUSD:  maxBudget,
		WorkingDir:    req.WorkingDir,
		Binary:        resolveBinary(req.Config, e.cfg.Binary),
		ExecutionMode: normalizeExecutionMode(req.ExecutionMode, req.Config),
		AutonomyLevel: normalizeAutonomyLevel(req.AutonomyLevel, req.Config),
	}

	// Agent-type specific config.
	switch e.cfg.AgentType {
	case "claude_code":
		if bcfg.ExecutionMode == "plan" {
			bcfg.Mode = "autonomous"
		}
		if strings.EqualFold(bcfg.Mode, "autonomous") {
			allowedTools := e.cfg.AutonomousAllowedTools
			if override := pickString(req.Config, "allowed_tools", ""); override != "" {
				allowedTools = splitList(override)
			} else if bcfg.ExecutionMode == "plan" {
				allowedTools = e.cfg.PlanAllowedTools
				if len(allowedTools) == 0 {
					allowedTools = []string{"Read", "Glob", "Grep", "WebSearch"}
				}
			}
			bcfg.AllowedTools = allowedTools
		}
	case "codex", "kimi":
		bcfg.ApprovalPolicy = pickString(req.Config, "approval_policy", e.cfg.ApprovalPolicy)
		bcfg.Sandbox = pickString(req.Config, "sandbox", e.cfg.Sandbox)
		if bcfg.ExecutionMode == "plan" {
			planApproval := pickString(req.Config, "plan_approval_policy", "")
			if utils.IsBlank(planApproval) {
				planApproval = bcfg.ApprovalPolicy
			}
			if utils.IsBlank(planApproval) {
				planApproval = e.cfg.PlanApprovalPolicy
			}
			bcfg.ApprovalPolicy = planApproval

			planSandbox := pickString(req.Config, "plan_sandbox", "")
			if utils.IsBlank(planSandbox) {
				planSandbox = bcfg.Sandbox
			}
			if utils.IsBlank(planSandbox) {
				planSandbox = e.cfg.PlanSandbox
			}
			bcfg.Sandbox = planSandbox

			if utils.IsBlank(bcfg.ApprovalPolicy) {
				bcfg.ApprovalPolicy = "never"
			}
			if utils.IsBlank(bcfg.Sandbox) {
				bcfg.Sandbox = "read-only"
			}
		}
	}

	env := core.CloneStringMap(e.cfg.Env)
	if env == nil {
		env = make(map[string]string)
	}
	if e.cfg.APIKey != "" {
		switch e.cfg.AgentType {
		case "claude_code":
			env["ANTHROPIC_API_KEY"] = e.cfg.APIKey
		case "codex":
			env["OPENAI_API_KEY"] = e.cfg.APIKey
		case "kimi":
			env["KIMI_API_KEY"] = e.cfg.APIKey
		}
	}

	// Prevent nested-session detection in Claude Code CLI.
	if e.cfg.AgentType == "claude_code" {
		env["CLAUDECODE"] = "" // empty = unset in process.MergeEnv
	}

	// Detached mode: output goes to a file, subprocess survives parent death.
	if e.cfg.Detached {
		return e.executeDetached(ctx, req, bcfg, pythonBin, bridgeScript, env)
	}
	return e.executeAttached(ctx, req, bcfg, pythonBin, bridgeScript, env)
}

const defaultAttachedTimeout = 4 * time.Hour

// writeBridgeConfig marshals and sends the bridge config to the subprocess stdin.
func writeBridgeConfig(proc bridgeRunner, bcfg bridgeConfig) error {
	data, err := json.Marshal(bcfg)
	if err != nil {
		return fmt.Errorf("marshal bridge config: %w", err)
	}
	data = append(data, '\n')
	if err := proc.Write(data); err != nil {
		return fmt.Errorf("write bridge config: %w", err)
	}
	return nil
}

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

type runtimeEventSink struct {
	roleLogPath  string
	eventLogPath string
	teamID       string
	roleID       string
	taskID       string
}

func newRuntimeEventSink(req agent.ExternalAgentRequest) *runtimeEventSink {
	var roleLogPath string
	var eventLogPath string
	var teamID string
	var roleID string
	if req.Config != nil {
		roleLogPath = strings.TrimSpace(req.Config["role_log_path"])
		eventLogPath = strings.TrimSpace(req.Config["team_event_log"])
		teamID = strings.TrimSpace(req.Config["team_id"])
		roleID = strings.TrimSpace(req.Config["role_id"])
	}
	return &runtimeEventSink{
		roleLogPath:  roleLogPath,
		eventLogPath: eventLogPath,
		teamID:       teamID,
		roleID:       roleID,
		taskID:       strings.TrimSpace(req.TaskID),
	}
}

func (s *runtimeEventSink) record(eventType string, fields map[string]any) {
	if s == nil {
		return
	}
	payload := map[string]any{
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"type":      eventType,
		"task_id":   s.taskID,
	}
	if s.teamID != "" {
		payload["team_id"] = s.teamID
	}
	if s.roleID != "" {
		payload["role_id"] = s.roleID
	}
	for k, v := range fields {
		payload[k] = v
	}
	data, err := json.Marshal(payload)
	if err != nil {
		return
	}
	if s.eventLogPath != "" {
		appendLogLine(s.eventLogPath, string(data))
	}
	if s.roleLogPath != "" {
		appendLogLine(s.roleLogPath, string(data))
	}
}

func (s *runtimeEventSink) recordFromSDK(ev SDKEvent) {
	switch ev.Type {
	case SDKEventTool:
		s.record("tool_call", map[string]any{
			"tool_name": ev.ToolName,
			"summary":   ev.Summary,
			"iter":      ev.Iter,
		})
	case SDKEventResult:
		s.record("result", map[string]any{
			"iters":    ev.Iters,
			"tokens":   ev.Tokens,
			"is_error": ev.IsError,
		})
	case SDKEventError:
		s.record("error", map[string]any{
			"message": ev.Message,
		})
	}
}

func appendLogLine(path string, line string) {
	trimmedPath := strings.TrimSpace(path)
	if trimmedPath == "" {
		return
	}
	if err := os.MkdirAll(filepath.Dir(trimmedPath), 0o755); err != nil {
		return
	}
	f, err := os.OpenFile(trimmedPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer f.Close()
	_, _ = f.WriteString(strings.TrimSpace(line) + "\n")
}

func (e *Executor) resolvePython() string {
	if e.cfg.PythonBinary != "" {
		return e.cfg.PythonBinary
	}
	if script := e.resolveBridgeScript(); script != "" {
		scriptDir := filepath.Dir(script)
		venvPython := filepath.Join(scriptDir, ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
		// Venv missing or broken — try auto-provisioning via setup.sh.
		if provisioned := e.ensureVenv(scriptDir); provisioned != "" {
			return provisioned
		}
	}
	return "python3"
}

// ensureVenv runs the setup.sh script in scriptDir to create the venv.
// Returns the venv python3 path on success, or empty string on failure.
func (e *Executor) ensureVenv(scriptDir string) string {
	setupScript := filepath.Join(scriptDir, "setup.sh")
	if _, err := os.Stat(setupScript); err != nil {
		return ""
	}
	e.logger.Info("Bridge venv missing, auto-provisioning via setup.sh", "dir", scriptDir)
	cmd := exec.Command("bash", setupScript)
	cmd.Dir = scriptDir
	if out, err := cmd.CombinedOutput(); err != nil {
		e.logger.Error("Bridge venv auto-setup failed", "err", err, "output", string(out))
		return ""
	}
	venvPython := filepath.Join(scriptDir, ".venv", "bin", "python3")
	if _, err := os.Stat(venvPython); err == nil {
		e.logger.Info("Bridge venv auto-provisioned successfully", "python", venvPython)
		return venvPython
	}
	return ""
}

func (e *Executor) resolveBridgeScript() string {
	if e.cfg.BridgeScript != "" {
		if abs, err := filepath.Abs(e.cfg.BridgeScript); err == nil {
			return abs
		}
		return e.cfg.BridgeScript
	}
	// Resolve relative to the running binary's directory.
	scriptDir := e.defaultScriptDir()
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "scripts", scriptDir, scriptDir+".py")
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	// Fallback: resolve relative path to absolute from CWD so it works
	// regardless of the subprocess WorkingDir.
	rel := filepath.Join("scripts", scriptDir, scriptDir+".py")
	if abs, err := filepath.Abs(rel); err == nil {
		return abs
	}
	return rel
}

// defaultScriptDir returns the bridge script directory name for the agent type.
func (e *Executor) defaultScriptDir() string {
	switch e.cfg.AgentType {
	case "kimi":
		return "kimi_bridge"
	case "codex":
		return "codex_bridge"
	default:
		return "cc_bridge"
	}
}

// maybeAppendAuthHint appends an agent-specific authentication hint when
// stderr output suggests authentication failure.
func (e *Executor) maybeAppendAuthHint(msg string, stderrTail string) string {
	switch e.cfg.AgentType {
	case "claude_code":
		if !containsAny(stderrTail, []string{"not logged", "unauthorized"}) {
			return msg
		}
		return fmt.Sprintf("%s Hint: ensure the Claude CLI is logged in (e.g. run `claude login`).", msg)
	case "codex":
		if !containsAny(stderrTail, []string{"api key", "unauthorized", "authentication"}) {
			return msg
		}
		return fmt.Sprintf("%s Hint: ensure Codex has a valid login or API key configured.", msg)
	case "kimi":
		if !containsAny(stderrTail, []string{"api key", "unauthorized", "authentication", "token"}) {
			return msg
		}
		return fmt.Sprintf("%s Hint: ensure Kimi CLI has valid authentication configured.", msg)
	default:
		return msg
	}
}

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

// validateWorktreePolicy returns an error if the working directory is on the
// main branch and not inside a git worktree. This enforces the worktree-based
// development workflow: agents must not write to files directly on main.
func validateWorktreePolicy(workDir string) error {
	if workDir == "" {
		workDir = "."
	}

	// Determine current branch.
	branchCmd := exec.Command("git", "-C", workDir, "branch", "--show-current")
	branchOut, err := branchCmd.Output()
	if err != nil {
		// Not a git repo or git not available — skip enforcement.
		return nil
	}
	branch := strings.TrimSpace(string(branchOut))
	if branch != "main" {
		return nil
	}

	// Check if we're in a worktree (git-dir differs from git-common-dir).
	gitDirCmd := exec.Command("git", "-C", workDir, "rev-parse", "--git-dir")
	gitDirOut, err := gitDirCmd.Output()
	if err != nil {
		return nil
	}
	gitCommonCmd := exec.Command("git", "-C", workDir, "rev-parse", "--git-common-dir")
	gitCommonOut, err := gitCommonCmd.Output()
	if err != nil {
		return nil
	}

	gitDir := strings.TrimSpace(string(gitDirOut))
	gitCommon := strings.TrimSpace(string(gitCommonOut))

	// Resolve to absolute for reliable comparison.
	if !filepath.IsAbs(gitDir) {
		gitDir = filepath.Join(workDir, gitDir)
	}
	if !filepath.IsAbs(gitCommon) {
		gitCommon = filepath.Join(workDir, gitCommon)
	}
	gitDir = filepath.Clean(gitDir)
	gitCommon = filepath.Clean(gitCommon)

	if gitDir != gitCommon {
		// Inside a worktree — allow.
		return nil
	}

	return fmt.Errorf("worktree policy: refusing to execute on main branch (not a worktree). " +
		"Create a worktree first: git worktree add -b <branch> ../<dir> main")
}

// --- Helpers ---

func requestKey(taskID, reqID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(taskID), strings.TrimSpace(reqID))
}

// resolveBinary returns the effective binary path/name for a bridge run.
//
// Priority order:
// 1. Request config "binary"
// 2. Executor fallback binary
//
// Special case: when the executor fallback is an absolute path and the request
// binary refers to the same basename (e.g. both are "kimi"), keep the absolute
// fallback path. This allows explicit pinned test/runtime binaries to remain
// stable even when runtime metadata injects a generic binary name/path.
func resolveBinary(config map[string]string, fallback string) string {
	requestBinary := pickString(config, "binary", "")
	fallback = strings.TrimSpace(fallback)
	if requestBinary == "" {
		return fallback
	}
	if fallback == "" {
		return requestBinary
	}
	if filepath.IsAbs(fallback) {
		requestBase := strings.TrimSpace(filepath.Base(requestBinary))
		fallbackBase := strings.TrimSpace(filepath.Base(fallback))
		if requestBase != "" && requestBase == fallbackBase {
			return fallback
		}
	}
	return requestBinary
}

func pickString(config map[string]string, key string, fallback string) string {
	if config == nil {
		return fallback
	}
	if val := strings.TrimSpace(config[key]); val != "" {
		return val
	}
	return fallback
}

func pickInt(config map[string]string, key string, fallback int) int {
	if config == nil {
		return fallback
	}
	if val := strings.TrimSpace(config[key]); val != "" {
		if parsed, err := strconv.Atoi(val); err == nil {
			return parsed
		}
	}
	return fallback
}

func pickFloat(config map[string]string, key string, fallback float64) float64 {
	if config == nil {
		return fallback
	}
	if val := strings.TrimSpace(config[key]); val != "" {
		if parsed, err := strconv.ParseFloat(val, 64); err == nil {
			return parsed
		}
	}
	return fallback
}

func splitList(raw string) []string {
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func normalizeExecutionMode(raw string, cfg map[string]string) string {
	mode := strings.TrimSpace(raw)
	if mode == "" && cfg != nil {
		mode = cfg["execution_mode"]
	}
	return executioncontrol.NormalizeExecutionMode(mode)
}

func normalizeAutonomyLevel(raw string, cfg map[string]string) string {
	level := strings.TrimSpace(raw)
	if level == "" && cfg != nil {
		level = cfg["autonomy_level"]
	}
	return executioncontrol.NormalizeAutonomyLevel(level)
}

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

func formatProcessError(agentName string, err error, stderrTail string) string {
	name := strings.TrimSpace(agentName)
	if name == "" {
		name = "external agent"
	}
	msg := fmt.Sprintf("%s exited: %v", name, err)
	if detail := exitDetail(err); detail != "" {
		msg = fmt.Sprintf("%s (%s)", msg, detail)
	}
	if tail := compactTail(stderrTail, 400); tail != "" {
		msg = fmt.Sprintf("%s | stderr tail: %s", msg, tail)
	}
	return msg
}

func containsAny(input string, needles []string) bool {
	lower := strings.ToLower(input)
	for _, needle := range needles {
		if needle == "" {
			continue
		}
		if strings.Contains(lower, strings.ToLower(needle)) {
			return true
		}
	}
	return false
}

func compactTail(tail string, limit int) string {
	trimmed := strings.TrimSpace(tail)
	if trimmed == "" {
		return ""
	}
	compact := strings.Join(strings.Fields(trimmed), " ")
	if limit > 0 && len(compact) > limit {
		return compact[:limit]
	}
	return compact
}

type exitCoder interface {
	ExitCode() int
}

func exitDetail(err error) string {
	if err == nil {
		return ""
	}
	detail := ""
	var exitErr exitCoder
	if errors.As(err, &exitErr) {
		if code := exitErr.ExitCode(); code >= 0 {
			detail = fmt.Sprintf("exit=%d", code)
		}
	}
	if execErr := new(exec.ExitError); errors.As(err, &execErr) {
		if status, ok := execErr.Sys().(syscall.WaitStatus); ok && status.Signaled() {
			if detail == "" {
				detail = fmt.Sprintf("signal=%s", status.Signal())
			} else {
				detail = fmt.Sprintf("%s signal=%s", detail, status.Signal())
			}
		}
	}
	return detail
}
