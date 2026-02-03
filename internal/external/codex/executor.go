package codex

import (
	"context"
	"fmt"
	"strings"
	"sync"
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

type progressState struct {
	mu           sync.Mutex
	tokensUsed   int
	currentTool  string
	currentArgs  string
	filesTouched []string
	lastActivity time.Time
	lastEmit     time.Time
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

	state := &progressState{}
	client.SetNotificationHandler(func(method string, params map[string]any) {
		if method != "codex/event" {
			return
		}
		e.handleCodexEvent(req, state, params)
	})

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
		msg := strings.TrimSpace(extractContent(result.Content))
		if msg == "" {
			msg = "codex tool error"
		}
		return &agent.ExternalAgentResult{Error: msg}, nil
	}
	answer := extractContent(result.Content)
	out := &agent.ExternalAgentResult{
		Answer: answer,
	}

	state.mu.Lock()
	out.TokensUsed = state.tokensUsed
	state.mu.Unlock()

	if result.StructuredContent != nil {
		out.Metadata = map[string]any{
			"structured_content": result.StructuredContent,
		}
		if threadID, ok := result.StructuredContent["thread_id"].(string); ok && strings.TrimSpace(threadID) != "" {
			out.Metadata["thread_id"] = threadID
		}
	}

	return out, nil
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

func (e *Executor) handleCodexEvent(req agent.ExternalAgentRequest, state *progressState, params map[string]any) {
	if req.OnProgress == nil {
		return
	}

	rawMsg, _ := params["msg"].(map[string]any)
	if rawMsg == nil {
		return
	}
	msgType, _ := rawMsg["type"].(string)
	if msgType == "" {
		return
	}

	// Drop noisy raw template payloads.
	if msgType == "raw_response_item" || msgType == "mcp_startup_update" {
		return
	}

	now := time.Now()
	updated := false

	state.mu.Lock()
	switch msgType {
	case "token_count":
		if tokens := extractTotalTokens(rawMsg); tokens > 0 && tokens != state.tokensUsed {
			state.tokensUsed = tokens
			updated = true
		}
		state.lastActivity = now
	case "agent_message_delta":
		if delta := extractDelta(rawMsg); delta != "" {
			state.currentTool = "assistant_output"
			state.currentArgs = truncate(delta, 200)
			state.lastActivity = now
			updated = true
		}
	case "agent_message":
		if text := extractDelta(rawMsg); text != "" {
			state.currentTool = "assistant_output"
			state.currentArgs = truncate(text, 200)
			state.lastActivity = now
			updated = true
		}
	case "task_started":
		state.currentTool = "task_started"
		state.currentArgs = ""
		state.lastActivity = now
		updated = true
	case "task_complete":
		state.currentTool = "task_complete"
		state.currentArgs = ""
		state.lastActivity = now
		updated = true
	default:
		// Best-effort: surface unknown events without payloads.
		if strings.Contains(msgType, "tool") {
			state.currentTool = msgType
			state.currentArgs = ""
			state.lastActivity = now
			updated = true
		}
	}

	shouldEmit := updated && (state.lastEmit.IsZero() || now.Sub(state.lastEmit) >= 250*time.Millisecond)
	if shouldEmit {
		state.lastEmit = now
	}
	snapshot := agent.ExternalAgentProgress{
		TokensUsed:   state.tokensUsed,
		CurrentTool:  state.currentTool,
		CurrentArgs:  state.currentArgs,
		FilesTouched: append([]string(nil), state.filesTouched...),
		LastActivity: state.lastActivity,
	}
	state.mu.Unlock()

	if shouldEmit {
		req.OnProgress(snapshot)
	}
}

func extractTotalTokens(msg map[string]any) int {
	info, _ := msg["info"].(map[string]any)
	if info == nil {
		return 0
	}
	totalUsage, _ := info["total_token_usage"].(map[string]any)
	if totalUsage == nil {
		return 0
	}
	switch v := totalUsage["total_tokens"].(type) {
	case float64:
		return int(v)
	case int:
		return v
	case int64:
		return int(v)
	}
	return 0
}

func extractDelta(msg map[string]any) string {
	if v, ok := msg["delta"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	if v, ok := msg["text"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	if v, ok := msg["content"].(string); ok && strings.TrimSpace(v) != "" {
		return v
	}
	return ""
}

func truncate(s string, max int) string {
	if max <= 0 {
		return ""
	}
	if len(s) <= max {
		return s
	}
	return s[:max]
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
