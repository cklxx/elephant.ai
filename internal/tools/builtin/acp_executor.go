package builtin

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"mime"
	"path"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/acp"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/logging"
	jsonrpc "alex/internal/mcp"
	"gopkg.in/yaml.v3"
)

const (
	acpRetryBaseDelay  = 200 * time.Millisecond
	acpRetryMaxDelay   = 2 * time.Second
	acpRetryMaxElapsed = 10 * time.Second
)

// ACPExecutorConfig configures the ACP executor adapter.
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

	client, err := acp.Dial(execCtx, addr, 5*time.Second, t.logger)
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

	summary := strings.TrimSpace(handler.finalSummary())
	if summary == "" {
		summary = "ACP executor completed."
	}
	metadata := map[string]any{
		"executor_addr":     addr,
		"executor_session":  remoteID,
		"tool_call_count":   handler.toolCallCount(),
		"executor_updates":  handler.updateSummary(),
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
	resp, err := callWithRetry(ctx, client, "initialize", map[string]any{
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
	resp, err := callWithRetry(ctx, client, "session/set_mode", map[string]any{
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
		resp, err := callWithRetry(ctx, client, "session/load", map[string]any{
			"sessionId":  remoteID,
			"cwd":        cwd,
			"mcpServers": mcpServers,
		})
		if err == nil && resp != nil && resp.Error == nil {
			return remoteID, nil
		}
	}

	resp, err := callWithRetry(ctx, client, "session/new", map[string]any{
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

func callWithRetry(ctx context.Context, client *acp.Client, method string, params map[string]any) (*jsonrpc.Response, error) {
	if client == nil {
		return nil, fmt.Errorf("acp client not initialized")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	start := time.Now()
	delay := acpRetryBaseDelay

	for {
		resp, err := client.Call(ctx, method, params)
		if err == nil || !acp.IsRetryableError(err) {
			return resp, err
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		if time.Since(start) >= acpRetryMaxElapsed {
			return nil, err
		}

		timer := time.NewTimer(delay)
		select {
		case <-ctx.Done():
			timer.Stop()
			return nil, ctx.Err()
		case <-timer.C:
		}

		if delay < acpRetryMaxDelay {
			delay *= 2
			if delay > acpRetryMaxDelay {
				delay = acpRetryMaxDelay
			}
		}
	}
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
	updateCounts    map[string]int
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
		updateCounts:    make(map[string]int),
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

	h.recordUpdate(req.Params, updateType, updateRaw)

	switch updateType {
	case "agent_message_chunk":
		h.handleAgentMessage(updateRaw)
	case "user_message_chunk":
		h.handleUserMessage(updateRaw)
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

func (h *acpExecutorHandler) handleUserMessage(update map[string]any) {
	block, ok := update["content"].(map[string]any)
	if !ok {
		return
	}
	text, attachments := parseContentBlock(block)
	payload := map[string]any{
		"content": block,
	}
	if text != "" {
		payload["text"] = text
	}
	if len(attachments) > 0 {
		payload["attachments"] = attachments
	}
	nodeID := fmt.Sprintf("executor-user-%d", time.Now().UnixNano())
	h.emitEnvelope("workflow.executor.user_message", "executor", nodeID, payload)
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

func (h *acpExecutorHandler) recordUpdate(params map[string]any, updateType string, update map[string]any) {
	if updateType == "" {
		return
	}
	sessionID := strings.TrimSpace(stringArg(params, "sessionId"))
	h.mu.Lock()
	h.updateCounts[updateType]++
	h.mu.Unlock()

	payload := map[string]any{
		"update_type": updateType,
		"session_id":  sessionID,
		"update":      update,
	}
	nodeID := fmt.Sprintf("executor-update-%d", time.Now().UnixNano())
	h.emitEnvelope("workflow.executor.update", "diagnostic", nodeID, payload)
}

func (h *acpExecutorHandler) updateSummary() map[string]int {
	h.mu.Lock()
	defer h.mu.Unlock()
	if len(h.updateCounts) == 0 {
		return nil
	}
	out := make(map[string]int, len(h.updateCounts))
	for key, value := range h.updateCounts {
		out[key] = value
	}
	return out
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

type executorTaskPackage struct {
	SessionID    string          `yaml:"session_id"`
	TaskID       string          `yaml:"task_id,omitempty"`
	ParentTaskID string          `yaml:"parent_task_id,omitempty"`
	Instruction  string          `yaml:"instruction"`
	Context      executorContext `yaml:"context,omitempty"`
	Runtime      executorRuntime `yaml:"runtime"`
}

type executorContext struct {
	SystemPrompt string                     `yaml:"system_prompt,omitempty"`
	Messages     []executorMessage          `yaml:"messages,omitempty"`
	Attachments  []executorAttachment       `yaml:"attachments,omitempty"`
	Important    []ports.ImportantNote      `yaml:"important,omitempty"`
	Plans        []ports.PlanNode           `yaml:"plans,omitempty"`
	Beliefs      []ports.Belief             `yaml:"beliefs,omitempty"`
	Knowledge    []ports.KnowledgeReference `yaml:"knowledge_refs,omitempty"`
	WorldState   map[string]any             `yaml:"world_state,omitempty"`
	WorldDiff    map[string]any             `yaml:"world_diff,omitempty"`
	Feedback     []ports.FeedbackSignal     `yaml:"feedback,omitempty"`
	Meta         executorContextMeta        `yaml:"meta,omitempty"`
}

type executorContextMeta struct {
	Iterations   int `yaml:"iterations,omitempty"`
	TokenCount   int `yaml:"token_count,omitempty"`
	MessageCount int `yaml:"message_count,omitempty"`
}

type executorRuntime struct {
	CWD             string         `yaml:"cwd"`
	ToolMode        string         `yaml:"tool_mode,omitempty"`
	Limits          executorLimits `yaml:"limits,omitempty"`
	RequireManifest bool           `yaml:"require_manifest"`
}

type executorLimits struct {
	MaxCLICalls        int `yaml:"max_cli_calls,omitempty"`
	MaxDurationSeconds int `yaml:"max_duration_seconds,omitempty"`
}

type executorMessage struct {
	Role        string               `yaml:"role"`
	Content     string               `yaml:"content"`
	ToolCalls   []ports.ToolCall     `yaml:"tool_calls,omitempty"`
	ToolResults []executorToolResult `yaml:"tool_results,omitempty"`
	Metadata    map[string]any       `yaml:"metadata,omitempty"`
	Attachments []executorAttachment `yaml:"attachments,omitempty"`
	Source      ports.MessageSource  `yaml:"source,omitempty"`
}

type executorToolResult struct {
	CallID      string               `yaml:"call_id"`
	Content     string               `yaml:"content"`
	Error       string               `yaml:"error,omitempty"`
	Metadata    map[string]any       `yaml:"metadata,omitempty"`
	Attachments []executorAttachment `yaml:"attachments,omitempty"`
}

type executorAttachment struct {
	Name        string `yaml:"name"`
	MediaType   string `yaml:"media_type,omitempty"`
	URI         string `yaml:"uri,omitempty"`
	Source      string `yaml:"source,omitempty"`
	Description string `yaml:"description,omitempty"`
	Kind        string `yaml:"kind,omitempty"`
	Format      string `yaml:"format,omitempty"`
}

func buildExecutorPromptBlocks(ctx context.Context, instruction string, call ports.ToolCall, cfg ACPExecutorConfig, attachmentNames []string) ([]any, error) {
	pkg := executorTaskPackage{
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
		Instruction:  instruction,
		Runtime: executorRuntime{
			CWD:      cfg.CWD,
			ToolMode: strings.TrimSpace(cfg.Mode),
			Limits: executorLimits{
				MaxCLICalls:        cfg.MaxCLICalls,
				MaxDurationSeconds: cfg.MaxDurationSeconds,
			},
			RequireManifest: cfg.RequireArtifactManifest,
		},
	}

	if snapshot := ports.GetTaskStateSnapshot(ctx); snapshot != nil {
		pkg.Context = executorContext{
			SystemPrompt: snapshot.SystemPrompt,
			Messages:     buildExecutorMessages(snapshot.Messages),
			Attachments:  buildExecutorAttachments(snapshot.Attachments),
			Important:    cloneImportantNotes(snapshot.Important),
			Plans:        ports.ClonePlanNodes(snapshot.Plans),
			Beliefs:      ports.CloneBeliefs(snapshot.Beliefs),
			Knowledge:    ports.CloneKnowledgeReferences(snapshot.KnowledgeRefs),
			WorldState:   cloneMapAny(snapshot.WorldState),
			WorldDiff:    cloneMapAny(snapshot.WorldDiff),
			Feedback:     ports.CloneFeedbackSignals(snapshot.FeedbackSignals),
			Meta: executorContextMeta{
				Iterations:   snapshot.Iterations,
				TokenCount:   snapshot.TokenCount,
				MessageCount: len(snapshot.Messages),
			},
		}
	}

	payload, err := yaml.Marshal(pkg)
	if err != nil {
		return nil, err
	}
	prompt := "Task Package (YAML):\n" + string(payload)
	return buildPromptBlocks(prompt, attachmentNames, ctx), nil
}

func buildExecutorMessages(messages []ports.Message) []executorMessage {
	if len(messages) == 0 {
		return nil
	}
	out := make([]executorMessage, 0, len(messages))
	for _, msg := range messages {
		outMsg := executorMessage{
			Role:    msg.Role,
			Content: msg.Content,
			Source:  msg.Source,
		}
		if len(msg.ToolCalls) > 0 {
			outMsg.ToolCalls = append([]ports.ToolCall(nil), msg.ToolCalls...)
		}
		if len(msg.ToolResults) > 0 {
			outMsg.ToolResults = buildExecutorToolResults(msg.ToolResults)
		}
		if len(msg.Metadata) > 0 {
			meta := make(map[string]any, len(msg.Metadata))
			for k, v := range msg.Metadata {
				meta[k] = v
			}
			outMsg.Metadata = meta
		}
		if len(msg.Attachments) > 0 {
			outMsg.Attachments = buildExecutorAttachments(msg.Attachments)
		}
		out = append(out, outMsg)
	}
	return out
}

func buildExecutorToolResults(results []ports.ToolResult) []executorToolResult {
	if len(results) == 0 {
		return nil
	}
	out := make([]executorToolResult, 0, len(results))
	for _, result := range results {
		outRes := executorToolResult{
			CallID:  result.CallID,
			Content: result.Content,
		}
		if result.Error != nil {
			outRes.Error = result.Error.Error()
		}
		if len(result.Metadata) > 0 {
			meta := make(map[string]any, len(result.Metadata))
			for k, v := range result.Metadata {
				meta[k] = v
			}
			outRes.Metadata = meta
		}
		if len(result.Attachments) > 0 {
			outRes.Attachments = buildExecutorAttachments(result.Attachments)
		}
		out = append(out, outRes)
	}
	return out
}

func buildExecutorAttachments(attachments map[string]ports.Attachment) []executorAttachment {
	if len(attachments) == 0 {
		return nil
	}
	names := make([]string, 0, len(attachments))
	seen := make(map[string]bool, len(attachments))
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		names = append(names, name)
	}
	sort.Strings(names)

	out := make([]executorAttachment, 0, len(names))
	for _, name := range names {
		att := attachments[name]
		attName := strings.TrimSpace(att.Name)
		if attName == "" {
			attName = name
		}
		out = append(out, executorAttachment{
			Name:        attName,
			MediaType:   strings.TrimSpace(att.MediaType),
			URI:         strings.TrimSpace(att.URI),
			Source:      strings.TrimSpace(att.Source),
			Description: strings.TrimSpace(att.Description),
			Kind:        strings.TrimSpace(att.Kind),
			Format:      strings.TrimSpace(att.Format),
		})
	}
	return out
}

func cloneImportantNotes(notes map[string]ports.ImportantNote) []ports.ImportantNote {
	if len(notes) == 0 {
		return nil
	}
	keys := make([]string, 0, len(notes))
	for key := range notes {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	out := make([]ports.ImportantNote, 0, len(keys))
	for _, key := range keys {
		out = append(out, notes[key])
	}
	return out
}

func cloneMapAny(src map[string]any) map[string]any {
	if len(src) == 0 {
		return nil
	}
	out := make(map[string]any, len(src))
	for key, value := range src {
		out[key] = value
	}
	return out
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
