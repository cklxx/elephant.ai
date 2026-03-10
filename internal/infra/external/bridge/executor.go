package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	core "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/process"
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
		Binary:        pickString(req.Config, "binary", e.cfg.Binary),
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

func requestKey(taskID, reqID string) string {
	return fmt.Sprintf("%s:%s", strings.TrimSpace(taskID), strings.TrimSpace(reqID))
}

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
