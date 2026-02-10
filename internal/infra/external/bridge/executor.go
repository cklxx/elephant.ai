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

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/external/subprocess"
	"alex/internal/shared/logging"
)

// BridgeConfig configures a bridge executor for any supported agent type.
type BridgeConfig struct {
	AgentType    string // "claude_code" or "codex"
	PythonBinary string
	BridgeScript string
	Interactive  bool // whether to start permission relay (only claude_code)

	// Claude Code fields.
	APIKey                 string
	DefaultModel           string
	DefaultMode            string
	AutonomousAllowedTools []string
	MaxBudgetUSD           float64
	MaxTurns               int

	// Codex fields.
	ApprovalPolicy string
	Sandbox        string

	// Common fields.
	Timeout time.Duration
	Env     map[string]string

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

// subprocessAdapter adapts subprocess.Subprocess to bridgeRunner.
type subprocessAdapter struct {
	proc *subprocess.Subprocess
}

func (a *subprocessAdapter) Start(ctx context.Context) error { return a.proc.Start(ctx) }
func (a *subprocessAdapter) Write(data []byte) error         { return a.proc.Write(data) }
func (a *subprocessAdapter) Stdout() interface{ Read([]byte) (int, error) } {
	return a.proc.Stdout()
}
func (a *subprocessAdapter) StderrTail() string    { return a.proc.StderrTail() }
func (a *subprocessAdapter) Wait() error           { return a.proc.Wait() }
func (a *subprocessAdapter) Stop() error           { return a.proc.Stop() }
func (a *subprocessAdapter) PID() int              { return a.proc.PID() }
func (a *subprocessAdapter) Done() <-chan struct{}  { return a.proc.Done() }

// Executor implements agent.InteractiveExternalExecutor by spawning a Python
// bridge sidecar and reading pre-filtered JSONL events from its stdout.
// It supports multiple agent types (claude_code, codex) through a single
// parameterised implementation.
type Executor struct {
	cfg               BridgeConfig
	inputCh           chan agent.InputRequest
	pending           sync.Map
	logger            logging.Logger
	subprocessFactory func(subprocess.Config) bridgeRunner
}

// New creates a new bridge executor for the configured agent type.
func New(cfg BridgeConfig) *Executor {
	return &Executor{
		cfg:     cfg,
		inputCh: make(chan agent.InputRequest, 32),
		logger:  logging.NewComponentLogger("BridgeExecutor/" + cfg.AgentType),
		subprocessFactory: func(c subprocess.Config) bridgeRunner {
			return &subprocessAdapter{proc: subprocess.New(c)}
		},
	}
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
	Prompt            string         `json:"prompt"`
	Model             string         `json:"model,omitempty"`
	Mode              string         `json:"mode,omitempty"`
	MaxTurns          int            `json:"max_turns,omitempty"`
	MaxBudgetUSD      float64        `json:"max_budget_usd,omitempty"`
	WorkingDir        string         `json:"working_dir,omitempty"`
	AllowedTools      []string       `json:"allowed_tools,omitempty"`
	PermissionMCPConf map[string]any `json:"permission_mcp_config,omitempty"`
	// Codex-specific fields.
	ApprovalPolicy string `json:"approval_policy,omitempty"`
	Sandbox        string `json:"sandbox,omitempty"`
}

func (e *Executor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	mode := pickString(req.Config, "mode", e.cfg.DefaultMode)
	model := pickString(req.Config, "model", e.cfg.DefaultModel)
	maxTurns := pickInt(req.Config, "max_turns", e.cfg.MaxTurns)
	maxBudget := pickFloat(req.Config, "max_budget_usd", e.cfg.MaxBudgetUSD)

	pythonBin := e.resolvePython()
	bridgeScript := e.resolveBridgeScript()

	bcfg := bridgeConfig{
		Prompt:       req.Prompt,
		Model:        model,
		Mode:         mode,
		MaxTurns:     maxTurns,
		MaxBudgetUSD: maxBudget,
		WorkingDir:   req.WorkingDir,
	}

	// Agent-type specific config.
	switch e.cfg.AgentType {
	case "claude_code":
		if strings.EqualFold(mode, "autonomous") {
			allowedTools := e.cfg.AutonomousAllowedTools
			if override := pickString(req.Config, "allowed_tools", ""); override != "" {
				allowedTools = splitList(override)
			}
			bcfg.AllowedTools = allowedTools
		} else if e.cfg.Interactive {
			socketPath, cleanup, err := e.startPermissionServer(ctx, req)
			if err != nil {
				return nil, err
			}
			defer cleanup()
			bcfg.PermissionMCPConf = buildPermissionMCPPayload(socketPath, req.TaskID)
		}
	case "codex":
		bcfg.ApprovalPolicy = pickString(req.Config, "approval_policy", e.cfg.ApprovalPolicy)
		bcfg.Sandbox = pickString(req.Config, "sandbox", e.cfg.Sandbox)
	}

	env := cloneStringMap(e.cfg.Env)
	if e.cfg.APIKey != "" {
		switch e.cfg.AgentType {
		case "claude_code":
			env["ANTHROPIC_API_KEY"] = e.cfg.APIKey
		case "codex":
			env["OPENAI_API_KEY"] = e.cfg.APIKey
		}
	}

	// Detached mode: output goes to a file, subprocess survives parent death.
	if e.cfg.Detached {
		return e.executeDetached(ctx, req, bcfg, pythonBin, bridgeScript, env)
	}
	return e.executeAttached(ctx, req, bcfg, pythonBin, bridgeScript, env)
}

// executeAttached runs the bridge with stdout piped back to this process.
func (e *Executor) executeAttached(ctx context.Context, req agent.ExternalAgentRequest, bcfg bridgeConfig, pythonBin, bridgeScript string, env map[string]string) (*agent.ExternalAgentResult, error) {
	proc := e.subprocessFactory(subprocess.Config{
		Command:    pythonBin,
		Args:       []string{bridgeScript},
		Env:        env,
		WorkingDir: req.WorkingDir,
		Timeout:    e.cfg.Timeout,
	})
	if err := proc.Start(ctx); err != nil {
		return nil, fmt.Errorf("start bridge: %w", err)
	}
	defer func() { _ = proc.Stop() }()

	configJSON, err := json.Marshal(bcfg)
	if err != nil {
		return nil, fmt.Errorf("marshal bridge config: %w", err)
	}
	configJSON = append(configJSON, '\n')
	if err := proc.Write(configJSON); err != nil {
		return nil, fmt.Errorf("write bridge config: %w", err)
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
		if ev.Type == SDKEventError {
			return result, errors.New(ev.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if err := proc.Wait(); err != nil {
		errMsg := formatProcessError(req.AgentType, err, proc.StderrTail())
		return result, errors.New(e.maybeAppendAuthHint(errMsg, proc.StderrTail()))
	}
	return result, nil
}

// executeDetached runs the bridge in detached mode: output goes to a file,
// subprocess becomes a session leader that survives parent death.
func (e *Executor) executeDetached(ctx context.Context, req agent.ExternalAgentRequest, bcfg bridgeConfig, pythonBin, bridgeScript string, env map[string]string) (*agent.ExternalAgentResult, error) {
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

	proc := e.subprocessFactory(subprocess.Config{
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

	// Write config to stdin.
	configJSON, err := json.Marshal(bcfg)
	if err != nil {
		_ = proc.Stop()
		return nil, fmt.Errorf("marshal bridge config: %w", err)
	}
	configJSON = append(configJSON, '\n')
	if err := proc.Write(configJSON); err != nil {
		_ = proc.Stop()
		return nil, fmt.Errorf("write bridge config: %w", err)
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
		return result, lastErr
	}
	if err := proc.Wait(); err != nil {
		errMsg := formatProcessError(req.AgentType, err, proc.StderrTail())
		return result, errors.New(e.maybeAppendAuthHint(errMsg, proc.StderrTail()))
	}
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

func (e *Executor) startPermissionServer(ctx context.Context, req agent.ExternalAgentRequest) (string, func(), error) {
	relay, err := newPermissionRelay(ctx, req.TaskID, req.AgentType, e.cfg.AutonomousAllowedTools, e.inputCh, &e.pending, e.logger)
	if err != nil {
		return "", nil, err
	}
	socketPath, cleanup, err := relay.Start()
	if err != nil {
		return "", nil, err
	}
	return socketPath, cleanup, nil
}

// buildPermissionMCPPayload generates the MCP server config dict that the
// Python bridge will pass to ClaudeAgentOptions.mcp_servers.
func buildPermissionMCPPayload(socketPath, taskID string) map[string]any {
	return map[string]any{
		"elephant": map[string]any{
			"command": os.Args[0],
			"args": []string{
				"mcp-permission-server",
				"--task-id", taskID,
				"--sock", socketPath,
			},
			"type": "stdio",
		},
	}
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

// --- Helpers ---

func requestKey(taskID, reqID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(taskID), strings.TrimSpace(reqID))
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

func cloneStringMap(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
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
