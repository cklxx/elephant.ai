package claudecode

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
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
	id "alex/internal/shared/utils/id"
)

// Config configures the Claude Code executor.
type Config struct {
	BinaryPath             string
	APIKey                 string
	DefaultModel           string
	DefaultMode            string
	AutonomousAllowedTools []string
	MaxBudgetUSD           float64
	MaxTurns               int
	Timeout                time.Duration
	Env                    map[string]string
}

// Executor implements agent.InteractiveExternalExecutor for Claude Code CLI.
type Executor struct {
	cfg               Config
	inputCh           chan agent.InputRequest
	pending           sync.Map
	logger            logging.Logger
	subprocessFactory func(subprocess.Config) subprocessRunner
}

type subprocessRunner interface {
	Start(ctx context.Context) error
	Stdout() io.ReadCloser
	StderrTail() string
	Wait() error
	Stop() error
}

func New(cfg Config) *Executor {
	if strings.TrimSpace(cfg.BinaryPath) == "" {
		cfg.BinaryPath = "claude"
	}
	if strings.TrimSpace(cfg.DefaultMode) == "" {
		cfg.DefaultMode = "interactive"
	}
	return &Executor{
		cfg:               cfg,
		inputCh:           make(chan agent.InputRequest, 32),
		logger:            logging.NewComponentLogger("ClaudeCodeExecutor"),
		subprocessFactory: func(cfg subprocess.Config) subprocessRunner { return subprocess.New(cfg) },
	}
}

func (e *Executor) SupportedTypes() []string {
	return []string{"claude_code"}
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

func (e *Executor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}

	mode := pickString(req.Config, "mode", e.cfg.DefaultMode)
	model := pickString(req.Config, "model", e.cfg.DefaultModel)
	maxTurns := pickInt(req.Config, "max_turns", e.cfg.MaxTurns)
	maxBudget := pickFloat(req.Config, "max_budget_usd", e.cfg.MaxBudgetUSD)
	allowedTools := e.cfg.AutonomousAllowedTools
	if override := pickString(req.Config, "allowed_tools", ""); override != "" {
		allowedTools = splitList(override)
	}

	args := []string{"-p", "--output-format", "stream-json", "--verbose"}
	if model != "" {
		args = append(args, "--model", model)
	}
	if maxTurns > 0 {
		args = append(args, "--max-turns", fmt.Sprintf("%d", maxTurns))
	}
	if maxBudget > 0 {
		args = append(args, "--max-budget-usd", fmt.Sprintf("%.2f", maxBudget))
	}

	var cleanup func()
	if strings.EqualFold(mode, "autonomous") {
		args = append(args, "--dangerously-skip-permissions")
		if len(allowedTools) > 0 {
			args = append(args, "--allowedTools", strings.Join(allowedTools, ","))
		}
	} else {
		socketPath, clean, err := e.startPermissionServer(ctx, req, allowedTools)
		if err != nil {
			return nil, err
		}
		cleanup = clean
		mcpConfigPath, err := writePermissionMCPConfig(socketPath, req.TaskID)
		if err != nil {
			if cleanup != nil {
				cleanup()
			}
			return nil, err
		}
		defer os.Remove(mcpConfigPath)
		args = append(args, "--mcp-config", mcpConfigPath, "--permission-prompt-tool", "mcp__elephant__approve")
	}

	args = append(args, "--", req.Prompt)

	env := cloneStringMap(e.cfg.Env)
	if e.cfg.APIKey != "" {
		env["ANTHROPIC_API_KEY"] = e.cfg.APIKey
	}

	proc := e.subprocessFactory(subprocess.Config{
		Command:    e.cfg.BinaryPath,
		Args:       args,
		Env:        env,
		WorkingDir: req.WorkingDir,
		Timeout:    e.cfg.Timeout,
	})
	if err := proc.Start(ctx); err != nil {
		if cleanup != nil {
			cleanup()
		}
		return nil, err
	}
	defer func() { _ = proc.Stop() }()
	if cleanup != nil {
		defer cleanup()
	}

	result := &agent.ExternalAgentResult{}
	scanner := bufio.NewScanner(proc.Stdout())
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 2*1024*1024)

	for scanner.Scan() {
		line := scanner.Bytes()
		msg, err := ParseStreamMessage(line)
		if err != nil {
			continue
		}
		if text := msg.ExtractText(); text != "" && msg.Type == "result" {
			result.Answer = text
		}
		if tokens, cost := msg.ExtractUsage(); tokens > 0 || cost > 0 {
			result.TokensUsed = tokens
		}
		if toolName, toolArgs := msg.ExtractToolEvent(); toolName != "" {
			progress := agent.ExternalAgentProgress{
				Iteration:    result.Iterations + 1,
				TokensUsed:   result.TokensUsed,
				CostUSD:      costFromResult(msg),
				CurrentTool:  toolName,
				CurrentArgs:  truncate(toolArgs, 120),
				LastActivity: time.Now(),
			}
			result.Iterations++
			if req.OnProgress != nil {
				req.OnProgress(progress)
			}
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

func (e *Executor) startPermissionServer(ctx context.Context, req agent.ExternalAgentRequest, allowedTools []string) (string, func(), error) {
	relay, err := newPermissionRelay(ctx, req.TaskID, req.AgentType, allowedTools, e.inputCh, &e.pending, e.logger)
	if err != nil {
		return "", nil, err
	}
	socketPath, cleanup, err := relay.Start()
	if err != nil {
		return "", nil, err
	}
	return socketPath, cleanup, nil
}

func writePermissionMCPConfig(socketPath, taskID string) (string, error) {
	tmpDir := os.TempDir()
	path := filepath.Join(tmpDir, fmt.Sprintf("elephant-mcp-%s.json", id.NewKSUID()))
	payload := map[string]any{
		"mcpServers": map[string]any{
			"elephant": map[string]any{
				"command": os.Args[0],
				"args": []string{
					"mcp-permission-server",
					"--task-id",
					taskID,
					"--sock",
					socketPath,
				},
				"type": "stdio",
			},
		},
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, encoded, 0o644); err != nil {
		return "", err
	}
	return path, nil
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

func truncate(input string, limit int) string {
	if limit <= 0 || len(input) <= limit {
		return input
	}
	return input[:limit]
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

func maybeAppendClaudeAuthHint(msg string, stderrTail string) string {
	if !containsAny(stderrTail, []string{"not logged", "unauthorized"}) {
		return msg
	}
	return fmt.Sprintf("%s Hint: ensure the Claude CLI is logged in (e.g. run `claude login`).", msg)
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

func costFromResult(msg StreamMessage) float64 {
	_, cost := msg.ExtractUsage()
	return cost
}
