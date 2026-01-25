package builtin

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/acp"
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/logging"
)

type ACPExecutorConfig struct {
	Addr                    string
	CWD                     string
	Mode                    string
	AutoApprove             bool
	MaxCLICalls             int
	MaxDurationSeconds      int
	RequireArtifactManifest bool
}

type acpExecutorTool struct {
	cfg      ACPExecutorConfig
	logger   logging.Logger
	mu       sync.Mutex
	sessions map[string]string
}

// NewACPExecutor creates the ACP executor tool.
func NewACPExecutor(cfg ACPExecutorConfig) tools.ToolExecutor {
	return &acpExecutorTool{
		cfg:      cfg,
		logger:   logging.NewComponentLogger("ACPExecutor"),
		sessions: make(map[string]string),
	}
}

func (t *acpExecutorTool) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "acp_executor",
		Version:  "1.0.0",
		Category: "execution",
		Tags:     []string{"acp", "executor", "cli"},
	}
}

func (t *acpExecutorTool) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "acp_executor",
		Description: "Dispatch a task package to an ACP-ready executor (Codex/Claude/Gemini CLI) and stream back execution events.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"instruction": {
					Type:        "string",
					Description: "Task instruction for the executor (context-first task package is built automatically).",
				},
				"attachment_names": {
					Type:        "array",
					Description: "Attachment names from the current context to send to the executor.",
					Items:       &ports.Property{Type: "string"},
				},
			},
			Required: []string{},
		},
	}
}

func (t *acpExecutorTool) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	addr := strings.TrimSpace(t.cfg.Addr)
	if addr == "" {
		addr = "http://127.0.0.1:9000"
	}

	cwd := strings.TrimSpace(t.cfg.CWD)
	if cwd == "" {
		cwd = "/workspace"
	}
	if !strings.HasPrefix(cwd, "/") {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor cwd must be absolute")}, nil
	}

	instruction := strings.TrimSpace(stringArg(call.Arguments, "instruction"))
	if instruction == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor requires instruction")}, nil
	}

	maxCLICalls := t.cfg.MaxCLICalls
	maxDurationSeconds := t.cfg.MaxDurationSeconds
	requireManifest := t.cfg.RequireArtifactManifest
	mode := strings.TrimSpace(t.cfg.Mode)
	attachmentNames := stringSliceArg(call.Arguments, "attachment_names")
	promptBlocks, err := buildExecutorPromptBlocks(ctx, instruction, call, t.cfg, attachmentNames)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	execCtx := ctx
	var cancel context.CancelFunc
	if maxDurationSeconds > 0 {
		execCtx, cancel = context.WithTimeout(execCtx, time.Duration(maxDurationSeconds)*time.Second)
		defer cancel()
	}

	client, err := acp.Dial(addr, 5*time.Second, t.logger)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor dial failed: %w", err)}, nil
	}
	defer func() {
		_ = client.Close()
	}()

	autoApprove := t.cfg.AutoApprove
	handler := newACPExecutorHandler(ctx, call, maxCLICalls, requireManifest, autoApprove, client, t.logger)
	client.Start(execCtx, handler)

	if err := callInitialize(execCtx, client); err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}

	sessionID := call.SessionID
	if sessionID == "" {
		sessionID = call.TaskID
	}
	if sessionID == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor requires session id")}, nil
	}

	remoteID, availableModes, err := t.ensureSession(execCtx, client, sessionID, cwd)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	handler.setRemoteSession(remoteID)

	if mode != "" {
		if !modeSupported(availableModes, mode) {
			t.logger.Warn("ACP executor mode %q unsupported; skipping set_mode", mode)
		} else if err := callSetMode(execCtx, client, remoteID, mode); err != nil {
			if isUnsupportedModeError(err) {
				t.logger.Warn("ACP executor mode %q rejected; continuing without set_mode", mode)
			} else {
				return &ports.ToolResult{CallID: call.ID, Error: err}, nil
			}
		}
	}

	params := map[string]any{
		"sessionId": remoteID,
		"prompt":    promptBlocks,
	}

	resp, err := callWithRetry(execCtx, client, "session/prompt", params)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor prompt failed: %w", err)}, nil
	}
	if resp.Error != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor prompt error: %s", resp.Error.Message)}, nil
	}

	resultErr := handler.finish()
	if resultErr != nil {
		return &ports.ToolResult{CallID: call.ID, Error: resultErr}, nil
	}

	summary, summaryAttachments := handler.summaryPayload()
	if summary == "" {
		summary = "ACP executor completed."
	}
	handler.emitSummaryEvent(summary, summaryAttachments)
	handler.emitFallbackManifestEvent()
	metadata := map[string]any{
		"executor_addr":             addr,
		"executor_session":          remoteID,
		"tool_call_count":           handler.toolCallCount(),
		"executor_updates":          handler.updateSummary(),
		"artifact_manifest":         handler.manifestPayload(),
		"artifact_manifest_missing": handler.isManifestMissing(),
		"stop_reason":               extractStopReason(resp.Result),
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  summary,
		Metadata: metadata,
	}, nil
}

func extractStopReason(result any) string {
	if result == nil {
		return ""
	}
	if raw, ok := result.(map[string]any); ok {
		if val, ok := raw["stopReason"].(string); ok {
			return strings.TrimSpace(val)
		}
	}
	return ""
}
