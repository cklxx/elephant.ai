package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"path"
	"strings"
	"sync"
	"time"

	"alex/internal/acp"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/logging"
	jsonrpc "alex/internal/mcp"
)

// ACPExecutorConfig configures the ACP executor adapter.
type ACPExecutorConfig struct {
	Addr                    string
	CWD                     string
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
func NewACPExecutor(cfg ACPExecutorConfig) ports.ToolExecutor {
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
				"prompt": {
					Type:        "string",
					Description: "Full task package to send to the executor (includes context snapshot + instruction).",
				},
				"instruction": {
					Type:        "string",
					Description: "Task instruction for the executor (used when prompt is omitted).",
				},
				"context": {
					Type:        "string",
					Description: "Context snapshot to prepend when prompt is omitted.",
				},
				"cwd": {
					Type:        "string",
					Description: "Executor working directory (absolute path).",
				},
				"mode": {
					Type:        "string",
					Description: "Executor tool mode (full/read-only/safe).",
				},
				"addr": {
					Type:        "string",
					Description: "ACP executor HTTP base URL (http://host:port).",
				},
				"max_cli_calls": {
					Type:        "integer",
					Description: "Max CLI/tool calls allowed for this executor run.",
				},
				"max_duration_seconds": {
					Type:        "integer",
					Description: "Max duration in seconds for this executor run.",
				},
				"require_manifest": {
					Type:        "boolean",
					Description: "Require artifact manifest emission before completion.",
				},
				"auto_approve": {
					Type:        "boolean",
					Description: "Auto-approve executor permission requests (session/request_permission).",
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
	addr := strings.TrimSpace(stringArg(call.Arguments, "addr"))
	if addr == "" {
		addr = strings.TrimSpace(t.cfg.Addr)
	}
	if addr == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor addr is required")}, nil
	}

	cwd := strings.TrimSpace(stringArg(call.Arguments, "cwd"))
	if cwd == "" {
		cwd = strings.TrimSpace(t.cfg.CWD)
	}
	if cwd == "" {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor cwd is required")}, nil
	}
	if !strings.HasPrefix(cwd, "/") {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor cwd must be absolute")}, nil
	}

	prompt := strings.TrimSpace(stringArg(call.Arguments, "prompt"))
	if prompt == "" {
		instruction := strings.TrimSpace(stringArg(call.Arguments, "instruction"))
		contextSnapshot := strings.TrimSpace(stringArg(call.Arguments, "context"))
		if instruction == "" {
			return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor requires prompt or instruction")}, nil
		}
		if contextSnapshot != "" {
			prompt = fmt.Sprintf("Context Snapshot:\n%s\n\nTask:\n%s", contextSnapshot, instruction)
		} else {
			prompt = instruction
		}
	}

	maxCLICalls := intArg(call.Arguments, "max_cli_calls")
	if maxCLICalls <= 0 {
		maxCLICalls = t.cfg.MaxCLICalls
	}
	maxDurationSeconds := intArg(call.Arguments, "max_duration_seconds")
	if maxDurationSeconds <= 0 {
		maxDurationSeconds = t.cfg.MaxDurationSeconds
	}
	requireManifest := t.cfg.RequireArtifactManifest
	if raw, ok := call.Arguments["require_manifest"]; ok {
		if v, ok := raw.(bool); ok {
			requireManifest = v
		}
	}

	mode := strings.TrimSpace(stringArg(call.Arguments, "mode"))

	attachmentNames := stringSliceArg(call.Arguments, "attachment_names")
	promptBlocks := buildPromptBlocks(prompt, attachmentNames, ctx)

	execCtx := ctx
	var cancel context.CancelFunc
	if maxDurationSeconds > 0 {
		execCtx, cancel = context.WithTimeout(execCtx, time.Duration(maxDurationSeconds)*time.Second)
		defer cancel()
	}

	client, err := acp.Dial(execCtx, addr, 5*time.Second, t.logger)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: fmt.Errorf("acp_executor dial failed: %w", err)}, nil
	}
	defer func() {
		_ = client.Close()
	}()

	autoApprove := t.cfg.AutoApprove
	if raw, ok := call.Arguments["auto_approve"]; ok {
		if v, ok := raw.(bool); ok {
			autoApprove = v
		}
	}
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

	remoteID, err := t.ensureSession(execCtx, client, sessionID, cwd)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Error: err}, nil
	}
	handler.setRemoteSession(remoteID)

	if mode != "" {
		if err := callSetMode(execCtx, client, remoteID, mode); err != nil {
			return &ports.ToolResult{CallID: call.ID, Error: err}, nil
		}
	}

	params := map[string]any{
		"sessionId": remoteID,
		"prompt":    promptBlocks,
	}

	resp, err := client.Call(execCtx, "session/prompt", params)
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

	summary := strings.TrimSpace(handler.finalSummary())
	if summary == "" {
		summary = "ACP executor completed."
	}
	metadata := map[string]any{
		"executor_addr":     addr,
		"executor_session":  remoteID,
		"tool_call_count":   handler.toolCallCount(),
		"artifact_manifest": handler.manifestPayload(),
		"stop_reason":       extractStopReason(resp.Result),
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  summary,
		Metadata: metadata,
	}, nil
}

func callInitialize(ctx context.Context, client *acp.Client) error {
	resp, err := client.Call(ctx, "initialize", map[string]any{
		"protocolVersion": 1,
	})
	if err != nil {
		return fmt.Errorf("acp_executor initialize failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("acp_executor initialize error: %s", resp.Error.Message)
	}
	return nil
}

func callSetMode(ctx context.Context, client *acp.Client, sessionID, mode string) error {
	resp, err := client.Call(ctx, "session/set_mode", map[string]any{
		"sessionId": sessionID,
		"modeId":    mode,
	})
	if err != nil {
		return fmt.Errorf("acp_executor set_mode failed: %w", err)
	}
	if resp.Error != nil {
		return fmt.Errorf("acp_executor set_mode error: %s", resp.Error.Message)
	}
	return nil
}

func (t *acpExecutorTool) ensureSession(ctx context.Context, client *acp.Client, sessionID, cwd string) (string, error) {
	t.mu.Lock()
	remoteID := t.sessions[sessionID]
	t.mu.Unlock()

	mcpServers := []any{}

	if remoteID != "" {
		resp, err := client.Call(ctx, "session/load", map[string]any{
			"sessionId":  remoteID,
			"cwd":        cwd,
			"mcpServers": mcpServers,
		})
		if err == nil && resp != nil && resp.Error == nil {
			return remoteID, nil
		}
	}

	resp, err := client.Call(ctx, "session/new", map[string]any{
		"cwd":        cwd,
		"mcpServers": mcpServers,
	})
	if err != nil {
		return "", fmt.Errorf("acp_executor session/new failed: %w", err)
	}
	if resp.Error != nil {
		return "", fmt.Errorf("acp_executor session/new error: %s", resp.Error.Message)
	}

	newID := ""
	if resp.Result != nil {
		if raw, ok := resp.Result.(map[string]any); ok {
			if val, ok := raw["sessionId"].(string); ok {
				newID = strings.TrimSpace(val)
			}
		}
	}
	if newID == "" {
		return "", fmt.Errorf("acp_executor session/new missing sessionId")
	}
	t.mu.Lock()
	t.sessions[sessionID] = newID
	t.mu.Unlock()
	return newID, nil
}

type executorToolState struct {
	name    string
	started time.Time
}

type acpExecutorHandler struct {
	call        ports.ToolCall
	listener    ports.EventListener
	logger      logging.Logger
	client      *acp.Client
	autoApprove bool

	mu              sync.Mutex
	toolStates      map[string]*executorToolState
	textBuffer      strings.Builder
	manifest        map[string]any
	attachments     map[string]ports.Attachment
	toolCalls       int
	limitExceeded   bool
	requireManifest bool
	maxCLICalls     int
	remoteSessionID string
}

func newACPExecutorHandler(ctx context.Context, call ports.ToolCall, maxCLICalls int, requireManifest bool, autoApprove bool, client *acp.Client, logger logging.Logger) *acpExecutorHandler {
	parentListener := parentListenerFromContext(ctx)
	if logger == nil {
		logger = logging.NewComponentLogger("ACPExecutor")
	}
	return &acpExecutorHandler{
		call:            call,
		listener:        parentListener,
		logger:          logger,
		client:          client,
		autoApprove:     autoApprove,
		toolStates:      make(map[string]*executorToolState),
		attachments:     make(map[string]ports.Attachment),
		requireManifest: requireManifest,
		maxCLICalls:     maxCLICalls,
	}
}

func (h *acpExecutorHandler) setRemoteSession(sessionID string) {
	h.mu.Lock()
	h.remoteSessionID = sessionID
	h.mu.Unlock()
}

func (h *acpExecutorHandler) OnNotification(ctx context.Context, req *jsonrpc.Request) {
	if req == nil || req.Params == nil {
		return
	}
	if req.Method != "session/update" {
		return
	}
	updateRaw, ok := req.Params["update"].(map[string]any)
	if !ok {
		return
	}
	updateType := strings.TrimSpace(stringArg(updateRaw, "sessionUpdate"))
	if updateType == "" {
		return
	}

	switch updateType {
	case "agent_message_chunk":
		h.handleAgentMessage(updateRaw)
	case "tool_call":
		h.handleToolCall(updateRaw)
	case "tool_call_update":
		h.handleToolCallUpdate(updateRaw)
	case "plan":
		h.handlePlanUpdate(updateRaw)
	default:
	}
}

func (h *acpExecutorHandler) OnRequest(ctx context.Context, req *jsonrpc.Request) (*jsonrpc.Response, error) {
	if req == nil {
		return nil, nil
	}
	if req.Method != "session/request_permission" {
		return jsonrpc.NewResponse(req.ID, map[string]any{}), nil
	}
	outcome := "reject_once"
	if h.autoApprove {
		outcome = "allow_once"
	}
	response := map[string]any{
		"outcome": map[string]any{
			"outcome":  "selected",
			"optionId": outcome,
		},
	}
	return jsonrpc.NewResponse(req.ID, response), nil
}

func (h *acpExecutorHandler) finish() error {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.maxCLICalls > 0 && h.toolCalls > h.maxCLICalls {
		return fmt.Errorf("acp_executor exceeded max_cli_calls (%d)", h.maxCLICalls)
	}
	if h.requireManifest && h.manifest == nil {
		return fmt.Errorf("acp_executor missing artifact manifest")
	}
	return nil
}

func (h *acpExecutorHandler) finalSummary() string {
	h.mu.Lock()
	defer h.mu.Unlock()
	return strings.TrimSpace(h.textBuffer.String())
}

func (h *acpExecutorHandler) toolCallCount() int {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.toolCalls
}

func (h *acpExecutorHandler) manifestPayload() map[string]any {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.manifest == nil {
		return nil
	}
	payload := make(map[string]any, len(h.manifest))
	for k, v := range h.manifest {
		payload[k] = v
	}
	return payload
}

func (h *acpExecutorHandler) handleAgentMessage(update map[string]any) {
	block, ok := update["content"].(map[string]any)
	if !ok {
		return
	}
	text, attachments := parseContentBlock(block)
	if text != "" {
		h.mu.Lock()
		h.textBuffer.WriteString(text)
		h.mu.Unlock()
		h.emitEnvelope("workflow.node.output.delta", "generation", "executor-output", map[string]any{
			"delta": text,
			"final": false,
		})
	}
	if len(attachments) > 0 {
		h.mu.Lock()
		for name, att := range attachments {
			h.attachments[name] = att
		}
		h.mu.Unlock()
	}
}

func (h *acpExecutorHandler) handleToolCall(update map[string]any) {
	callID := strings.TrimSpace(stringArg(update, "toolCallId"))
	if callID == "" {
		return
	}
	title := strings.TrimSpace(stringArg(update, "title"))
	rawInput, _ := update["rawInput"].(map[string]any)

	h.mu.Lock()
	h.toolStates[callID] = &executorToolState{name: title, started: time.Now()}
	h.toolCalls++
	toolCalls := h.toolCalls
	maxCalls := h.maxCLICalls
	remoteSession := h.remoteSessionID
	h.mu.Unlock()

	if maxCalls > 0 && toolCalls > maxCalls {
		h.cancelRemote(remoteSession)
		h.mu.Lock()
		h.limitExceeded = true
		h.mu.Unlock()
		return
	}

	payload := map[string]any{
		"call_id":   callID,
		"tool_name": title,
		"arguments": rawInput,
		"iteration": 0,
	}
	h.emitEnvelope("workflow.tool.started", "tool", callID, payload)
}

func (h *acpExecutorHandler) handleToolCallUpdate(update map[string]any) {
	callID := strings.TrimSpace(stringArg(update, "toolCallId"))
	if callID == "" {
		return
	}
	status := strings.ToLower(strings.TrimSpace(stringArg(update, "status")))
	rawOutput := stringArg(update, "rawOutput")

	contentBlocks := []any{}
	if raw, ok := update["content"].([]any); ok {
		contentBlocks = raw
	}

	var (
		attachments = make(map[string]ports.Attachment)
		textChunks  []string
	)
	for _, block := range contentBlocks {
		blockMap, ok := block.(map[string]any)
		if !ok {
			continue
		}
		text, atts := parseContentBlock(blockMap)
		if text != "" {
			textChunks = append(textChunks, text)
		}
		for name, att := range atts {
			attachments[name] = att
		}
	}

	h.mu.Lock()
	state := h.toolStates[callID]
	h.mu.Unlock()
	toolName := ""
	startedAt := time.Time{}
	if state != nil {
		toolName = state.name
		startedAt = state.started
	}

	switch status {
	case "in_progress":
		chunk := rawOutput
		if chunk == "" && len(textChunks) > 0 {
			chunk = strings.Join(textChunks, "")
		}
		if chunk == "" {
			return
		}
		payload := map[string]any{
			"call_id": callID,
			"chunk":   chunk,
		}
		h.emitEnvelope("workflow.tool.progress", "tool", callID, payload)
	default:
		result := strings.TrimSpace(rawOutput)
		if result == "" && len(textChunks) > 0 {
			result = strings.Join(textChunks, "")
		}
		var errMsg string
		if status == "failed" {
			errMsg = result
		}
		payload := map[string]any{
			"call_id":   callID,
			"tool_name": toolName,
			"result":    result,
			"duration":  durationMs(startedAt),
		}
		if errMsg != "" {
			payload["error"] = errMsg
		}
		if len(attachments) > 0 {
			payload["attachments"] = attachments
		}
		h.emitEnvelope("workflow.tool.completed", "tool", callID, payload)
		h.handleArtifactManifest(toolName, result, attachments)
	}
}

func (h *acpExecutorHandler) handlePlanUpdate(update map[string]any) {
	entries, _ := update["entries"].([]any)
	if len(entries) == 0 {
		return
	}
	lines := make([]string, 0, len(entries))
	for _, entry := range entries {
		item, ok := entry.(map[string]any)
		if !ok {
			continue
		}
		content := strings.TrimSpace(stringArg(item, "content"))
		if content == "" {
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s", content))
	}
	if len(lines) == 0 {
		return
	}
	result := strings.Join(lines, "\n")
	callID := fmt.Sprintf("plan-%d", time.Now().UnixNano())
	payload := map[string]any{
		"call_id":   callID,
		"tool_name": "plan",
		"result":    result,
		"duration":  0,
		"metadata": map[string]any{
			"entries": entries,
		},
	}
	h.emitEnvelope("workflow.tool.completed", "tool", callID, payload)
}

func (h *acpExecutorHandler) handleArtifactManifest(toolName, result string, attachments map[string]ports.Attachment) {
	if !strings.EqualFold(strings.TrimSpace(toolName), "artifact_manifest") {
		return
	}
	manifest := parseManifestPayload(result)
	if manifest == nil {
		manifest = map[string]any{"raw": result}
	}
	if len(attachments) > 0 {
		manifest["attachments"] = attachments
	}

	h.mu.Lock()
	h.manifest = manifest
	h.mu.Unlock()

	h.emitEnvelope("workflow.artifact.manifest", "artifact", "artifact-manifest", manifest)
}

func (h *acpExecutorHandler) cancelRemote(sessionID string) {
	if sessionID == "" || h.client == nil {
		return
	}
	_ = h.client.Notify("session/cancel", map[string]any{
		"sessionId": sessionID,
	})
}

func (h *acpExecutorHandler) emitEnvelope(eventType, nodeKind, nodeID string, payload map[string]any) {
	if h.listener == nil {
		return
	}
	ts := time.Now()
	taskID := h.call.TaskID
	if taskID == "" {
		taskID = h.call.ID
	}
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(ports.LevelSubagent, h.call.SessionID, taskID, h.call.ParentTaskID, ts),
		Event:     eventType,
		Version:   1,
		NodeID:    nodeID,
		NodeKind:  nodeKind,
		Payload:   payload,
	}
	h.listener.OnEvent(env)
}

func buildPromptBlocks(prompt string, attachmentNames []string, ctx context.Context) []any {
	blocks := []any{map[string]any{"type": "text", "text": prompt}}
	if len(attachmentNames) == 0 {
		return blocks
	}
	attachments, _ := ports.GetAttachmentContext(ctx)
	for _, name := range attachmentNames {
		name = strings.TrimSpace(name)
		if name == "" {
			continue
		}
		att, ok := attachments[name]
		if !ok {
			continue
		}
		if block := attachmentToContentBlock(att); block != nil {
			blocks = append(blocks, block)
		}
	}
	return blocks
}

func attachmentToContentBlock(att ports.Attachment) map[string]any {
	if att.MediaType != "" {
		if strings.HasPrefix(att.MediaType, "image/") && att.Data != "" {
			return map[string]any{
				"type":     "image",
				"data":     att.Data,
				"mimeType": att.MediaType,
			}
		}
		if strings.HasPrefix(att.MediaType, "audio/") && att.Data != "" {
			return map[string]any{
				"type":     "audio",
				"data":     att.Data,
				"mimeType": att.MediaType,
			}
		}
	}

	if att.URI != "" && att.Data == "" {
		block := map[string]any{
			"type": "resource_link",
			"uri":  att.URI,
			"name": att.Name,
		}
		if att.MediaType != "" {
			block["mimeType"] = att.MediaType
		}
		if att.Description != "" {
			block["description"] = att.Description
		}
		return block
	}

	resource := map[string]any{
		"uri": att.URI,
	}
	if att.URI == "" {
		resource["uri"] = att.Name
	}
	if att.MediaType != "" {
		resource["mimeType"] = att.MediaType
	}
	if att.Data != "" {
		resource["blob"] = att.Data
	}

	return map[string]any{
		"type":     "resource",
		"resource": resource,
	}
}

func parseContentBlock(block map[string]any) (string, map[string]ports.Attachment) {
	if block == nil {
		return "", nil
	}
	blockType := strings.ToLower(stringArg(block, "type"))
	switch blockType {
	case "text":
		return strings.TrimSpace(stringArg(block, "text")), nil
	case "image":
		name := attachmentNameForMedia("image", stringArg(block, "mimeType"))
		return "", map[string]ports.Attachment{
			name: {
				Name:      name,
				MediaType: strings.TrimSpace(stringArg(block, "mimeType")),
				Data:      strings.TrimSpace(stringArg(block, "data")),
			},
		}
	case "audio":
		name := attachmentNameForMedia("audio", stringArg(block, "mimeType"))
		return "", map[string]ports.Attachment{
			name: {
				Name:      name,
				MediaType: strings.TrimSpace(stringArg(block, "mimeType")),
				Data:      strings.TrimSpace(stringArg(block, "data")),
			},
		}
	case "resource_link":
		name := strings.TrimSpace(stringArg(block, "name"))
		uri := strings.TrimSpace(stringArg(block, "uri"))
		if name == "" && uri != "" {
			name = path.Base(uri)
		}
		if name == "" {
			name = fmt.Sprintf("resource-%d", time.Now().UnixNano())
		}
		att := ports.Attachment{
			Name:        name,
			URI:         uri,
			MediaType:   strings.TrimSpace(stringArg(block, "mimeType")),
			Description: strings.TrimSpace(stringArg(block, "description")),
		}
		return "", map[string]ports.Attachment{name: att}
	case "resource":
		resource, ok := block["resource"].(map[string]any)
		if !ok {
			return "", nil
		}
		uri := strings.TrimSpace(stringArg(resource, "uri"))
		name := ""
		if uri != "" {
			name = path.Base(uri)
		}
		if name == "" {
			name = fmt.Sprintf("resource-%d", time.Now().UnixNano())
		}
		mimeType := strings.TrimSpace(stringArg(resource, "mimeType"))
		if text := strings.TrimSpace(stringArg(resource, "text")); text != "" {
			return "", map[string]ports.Attachment{name: {
				Name:      name,
				URI:       uri,
				MediaType: mimeType,
				Data:      base64.StdEncoding.EncodeToString([]byte(text)),
			}}
		}
		if blob := strings.TrimSpace(stringArg(resource, "blob")); blob != "" {
			return "", map[string]ports.Attachment{name: {
				Name:      name,
				URI:       uri,
				MediaType: mimeType,
				Data:      blob,
			}}
		}
	}
	return "", nil
}

func parseManifestPayload(raw string) map[string]any {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	var parsed any
	if err := json.Unmarshal([]byte(raw), &parsed); err != nil {
		return nil
	}
	switch v := parsed.(type) {
	case map[string]any:
		return v
	case []any:
		return map[string]any{"items": v}
	default:
		return nil
	}
}

func durationMs(start time.Time) int64 {
	if start.IsZero() {
		return 0
	}
	return time.Since(start).Milliseconds()
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

func intArg(args map[string]any, key string) int {
	if args == nil {
		return 0
	}
	switch value := args[key].(type) {
	case int:
		return value
	case int64:
		return int(value)
	case float64:
		return int(value)
	case json.Number:
		parsed, err := value.Int64()
		if err == nil {
			return int(parsed)
		}
	}
	return 0
}

func attachmentNameForMedia(prefix, mimeType string) string {
	ext := ""
	if mimeType != "" {
		if exts, err := mime.ExtensionsByType(mimeType); err == nil && len(exts) > 0 {
			ext = exts[0]
		}
	}
	if ext == "" {
		if prefix == "image" {
			ext = ".png"
		} else if prefix == "audio" {
			ext = ".wav"
		}
	}
	if ext == "" {
		ext = ".bin"
	}
	return fmt.Sprintf("%s-%d%s", prefix, time.Now().UnixNano(), ext)
}

func parentListenerFromContext(ctx context.Context) ports.EventListener {
	if ctx == nil {
		return nil
	}
	if listener := ctx.Value(parentListenerKey{}); listener != nil {
		if pl, ok := listener.(ports.EventListener); ok {
			return pl
		}
	}
	return nil
}
