package codex

import (
	"context"
	"fmt"
	"strings"
	"time"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
	"alex/internal/mcp"
)

// Config configures the Codex executor.
type Config struct {
	BinaryPath     string
	APIKey         string
	DefaultModel   string
	ApprovalPolicy string
	Sandbox        string
	Timeout        time.Duration
	Env            map[string]string
}

// Executor implements agent.ExternalAgentExecutor for the Codex MCP server.
type Executor struct {
	cfg    Config
	logger logging.Logger
}

func New(cfg Config) *Executor {
	if strings.TrimSpace(cfg.BinaryPath) == "" {
		cfg.BinaryPath = "codex"
	}
	return &Executor{
		cfg:    cfg,
		logger: logging.NewComponentLogger("CodexExecutor"),
	}
}

func (e *Executor) SupportedTypes() []string {
	return []string{"codex"}
}

func (e *Executor) Execute(ctx context.Context, req agent.ExternalAgentRequest) (*agent.ExternalAgentResult, error) {
	if strings.TrimSpace(req.Prompt) == "" {
		return nil, fmt.Errorf("prompt is required")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	if e.cfg.Timeout > 0 {
		var cancel context.CancelFunc
		ctx, cancel = context.WithTimeout(ctx, e.cfg.Timeout)
		defer cancel()
	}

	model := pickString(req.Config, "model", e.cfg.DefaultModel)
	approvalPolicy := pickString(req.Config, "approval_policy", e.cfg.ApprovalPolicy)
	sandbox := pickString(req.Config, "sandbox", e.cfg.Sandbox)

	env := cloneStringMap(e.cfg.Env)
	if e.cfg.APIKey != "" {
		env["OPENAI_API_KEY"] = e.cfg.APIKey
	}

	process := mcp.NewProcessManager(mcp.ProcessConfig{
		Command: e.cfg.BinaryPath,
		Args:    []string{"mcp-server"},
		Env:     env,
	})
	client := mcp.NewClient("codex", process)

	if err := client.Start(ctx); err != nil {
		return nil, err
	}
	defer func() { _ = client.Stop() }()

	args := map[string]any{
		"prompt": req.Prompt,
	}
	if approvalPolicy != "" {
		args["approval-policy"] = approvalPolicy
	}
	if sandbox != "" {
		args["sandbox"] = sandbox
	}
	if model != "" {
		args["model"] = model
	}
	if req.WorkingDir != "" {
		args["cwd"] = req.WorkingDir
	}

	result, err := client.CallTool(ctx, "codex", args)
	if err != nil {
		return nil, err
	}
	if result.IsError {
		return &agent.ExternalAgentResult{Error: "codex tool error"}, nil
	}
	answer := extractContent(result.Content)
	return &agent.ExternalAgentResult{
		Answer: answer,
	}, nil
}

func extractContent(content []mcp.ContentBlock) string {
	var sb strings.Builder
	for _, block := range content {
		if block.Type == "text" {
			sb.WriteString(block.Text)
		}
	}
	return sb.String()
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
