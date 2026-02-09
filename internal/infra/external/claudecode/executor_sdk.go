package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/external/subprocess"
	"alex/internal/shared/logging"
)

// SDKBridgeConfig configures the Python Agent SDK bridge executor.
type SDKBridgeConfig struct {
	PythonBinary string
	BridgeScript string
	APIKey       string
	DefaultModel string
	DefaultMode  string
	MaxBudgetUSD float64
	MaxTurns     int
	Timeout      time.Duration
	Env          map[string]string

	// AutonomousAllowedTools is forwarded to the bridge when mode=autonomous.
	AutonomousAllowedTools []string
}

// sdkBridgeRunner abstracts subprocess lifecycle for testability.
type sdkBridgeRunner interface {
	Start(ctx context.Context) error
	Write(data []byte) error
	Stdout() interface{ Read([]byte) (int, error) }
	StderrTail() string
	Wait() error
	Stop() error
}

// sdkSubprocessAdapter adapts subprocess.Subprocess to sdkBridgeRunner.
type sdkSubprocessAdapter struct {
	proc *subprocess.Subprocess
}

func (a *sdkSubprocessAdapter) Start(ctx context.Context) error { return a.proc.Start(ctx) }
func (a *sdkSubprocessAdapter) Write(data []byte) error         { return a.proc.Write(data) }
func (a *sdkSubprocessAdapter) Stdout() interface{ Read([]byte) (int, error) } {
	return a.proc.Stdout()
}
func (a *sdkSubprocessAdapter) StderrTail() string { return a.proc.StderrTail() }
func (a *sdkSubprocessAdapter) Wait() error        { return a.proc.Wait() }
func (a *sdkSubprocessAdapter) Stop() error         { return a.proc.Stop() }

// SDKBridgeExecutor implements agent.InteractiveExternalExecutor by spawning
// the Python bridge sidecar (scripts/cc_bridge/cc_bridge.py) and reading
// pre-filtered JSONL events from its stdout.
type SDKBridgeExecutor struct {
	cfg               SDKBridgeConfig
	inputCh           chan agent.InputRequest
	pending           sync.Map
	logger            logging.Logger
	subprocessFactory func(subprocess.Config) sdkBridgeRunner
}

// NewSDKBridge creates a new SDK bridge executor.
func NewSDKBridge(cfg SDKBridgeConfig) *SDKBridgeExecutor {
	return &SDKBridgeExecutor{
		cfg:     cfg,
		inputCh: make(chan agent.InputRequest, 32),
		logger:  logging.NewComponentLogger("ClaudeCodeSDKBridge"),
		subprocessFactory: func(c subprocess.Config) sdkBridgeRunner {
			return &sdkSubprocessAdapter{proc: subprocess.New(c)}
		},
	}
}

func (e *SDKBridgeExecutor) SupportedTypes() []string {
	return []string{"claude_code"}
}

func (e *SDKBridgeExecutor) InputRequests() <-chan agent.InputRequest {
	return e.inputCh
}

func (e *SDKBridgeExecutor) Reply(ctx context.Context, resp agent.InputResponse) error {
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

// bridgeConfig is the JSON payload sent to the Python bridge via stdin.
type bridgeConfig struct {
	Prompt            string            `json:"prompt"`
	Model             string            `json:"model,omitempty"`
	Mode              string            `json:"mode"`
	MaxTurns          int               `json:"max_turns,omitempty"`
	MaxBudgetUSD      float64           `json:"max_budget_usd,omitempty"`
	WorkingDir        string            `json:"working_dir,omitempty"`
	AllowedTools      []string          `json:"allowed_tools,omitempty"`
	PermissionMCPConf map[string]any    `json:"permission_mcp_config,omitempty"`
}

func (e *SDKBridgeExecutor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	mode := pickString(req.Config, "mode", e.cfg.DefaultMode)
	model := pickString(req.Config, "model", e.cfg.DefaultModel)
	maxTurns := pickInt(req.Config, "max_turns", e.cfg.MaxTurns)
	maxBudget := pickFloat(req.Config, "max_budget_usd", e.cfg.MaxBudgetUSD)

	pythonBin := e.resolvePython()
	bridgeScript := e.resolveBridgeScript()

	// Build bridge config.
	bcfg := bridgeConfig{
		Prompt:       req.Prompt,
		Model:        model,
		Mode:         mode,
		MaxTurns:     maxTurns,
		MaxBudgetUSD: maxBudget,
		WorkingDir:   req.WorkingDir,
	}

	if strings.EqualFold(mode, "autonomous") {
		allowedTools := e.cfg.AutonomousAllowedTools
		if override := pickString(req.Config, "allowed_tools", ""); override != "" {
			allowedTools = splitList(override)
		}
		bcfg.AllowedTools = allowedTools
	} else {
		// Interactive mode: start permission server, generate MCP config for bridge.
		socketPath, cleanup, err := e.startPermissionServer(ctx, req)
		if err != nil {
			return nil, err
		}
		defer cleanup()
		bcfg.PermissionMCPConf = buildPermissionMCPPayload(socketPath, req.TaskID)
	}

	env := cloneStringMap(e.cfg.Env)
	if e.cfg.APIKey != "" {
		env["ANTHROPIC_API_KEY"] = e.cfg.APIKey
	}

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

	// Send config JSON via stdin.
	configJSON, err := json.Marshal(bcfg)
	if err != nil {
		return nil, fmt.Errorf("marshal bridge config: %w", err)
	}
	configJSON = append(configJSON, '\n')
	if err := proc.Write(configJSON); err != nil {
		return nil, fmt.Errorf("write bridge config: %w", err)
	}

	// Read JSONL events from stdout.
	result := &agent.ExternalAgentResult{}
	scanner := bufio.NewScanner(proc.Stdout())
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		ev, err := ParseSDKEvent(scanner.Bytes())
		if err != nil {
			continue
		}
		switch ev.Type {
		case SDKEventTool:
			result.Iterations = ev.Iter
			if req.OnProgress != nil {
				req.OnProgress(agent.ExternalAgentProgress{
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
		case SDKEventError:
			return result, errors.New(ev.Message)
		}
	}
	if err := scanner.Err(); err != nil {
		return result, err
	}
	if err := proc.Wait(); err != nil {
		errMsg := formatProcessError(req.AgentType, err, proc.StderrTail())
		return result, errors.New(maybeAppendClaudeAuthHint(errMsg, proc.StderrTail()))
	}
	return result, nil
}

func (e *SDKBridgeExecutor) startPermissionServer(ctx context.Context, req agent.ExternalAgentRequest) (string, func(), error) {
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

func (e *SDKBridgeExecutor) resolvePython() string {
	if e.cfg.PythonBinary != "" {
		return e.cfg.PythonBinary
	}
	// Try the venv inside the bridge script directory.
	if script := e.resolveBridgeScript(); script != "" {
		venvPython := filepath.Join(filepath.Dir(script), ".venv", "bin", "python3")
		if _, err := os.Stat(venvPython); err == nil {
			return venvPython
		}
	}
	return "python3"
}

func (e *SDKBridgeExecutor) resolveBridgeScript() string {
	if e.cfg.BridgeScript != "" {
		return e.cfg.BridgeScript
	}
	// Resolve relative to the running binary's directory.
	if exe, err := os.Executable(); err == nil {
		candidate := filepath.Join(filepath.Dir(exe), "..", "scripts", "cc_bridge", "cc_bridge.py")
		if abs, err := filepath.Abs(candidate); err == nil {
			if _, err := os.Stat(abs); err == nil {
				return abs
			}
		}
	}
	// Fallback: assume scripts/cc_bridge/cc_bridge.py relative to cwd.
	return "scripts/cc_bridge/cc_bridge.py"
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
