package execution

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
	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
	jsonrpc "alex/internal/mcp"
	"alex/internal/tools/builtin/shared"
)

type executorToolState struct {
	name    string
	started time.Time
}

type acpExecutorHandler struct {
	call        ports.ToolCall
	listener    agent.EventListener
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
	manifestMissing bool
	summaryEmitted  bool
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
	updateType := strings.TrimSpace(shared.StringArg(updateRaw, "sessionUpdate"))
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
		h.manifest = buildFallbackManifest(h.attachments)
		h.manifestMissing = true
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

func (h *acpExecutorHandler) isManifestMissing() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.manifestMissing
}

func (h *acpExecutorHandler) summaryPayload() (string, map[string]ports.Attachment) {
	h.mu.Lock()
	defer h.mu.Unlock()
	summary := strings.TrimSpace(h.textBuffer.String())
	attachments := make(map[string]ports.Attachment, len(h.attachments))
	for key, value := range h.attachments {
		attachments[key] = value
	}
	return summary, attachments
}

func (h *acpExecutorHandler) emitSummaryEvent(summary string, attachments map[string]ports.Attachment) {
	if summary == "" {
		return
	}
	h.mu.Lock()
	if h.summaryEmitted {
		h.mu.Unlock()
		return
	}
	h.summaryEmitted = true
	h.mu.Unlock()

	payload := map[string]any{
		"content": summary,
	}
	if len(attachments) > 0 {
		payload["attachments"] = attachments
	}
	nodeID := fmt.Sprintf("executor-summary-%d", time.Now().UnixNano())
	h.emitEnvelope("workflow.node.output.summary", "generation", nodeID, payload)
}

func (h *acpExecutorHandler) emitFallbackManifestEvent() {
	if !h.isManifestMissing() {
		return
	}
	payload := h.manifestPayload()
	if payload == nil {
		return
	}
	h.emitEnvelope("workflow.artifact.manifest", "artifact", "artifact-manifest-fallback", payload)
}

func buildFallbackManifest(attachments map[string]ports.Attachment) map[string]any {
	items := []map[string]any{
		{
			"type":    "notice",
			"message": "Executor did not emit artifact_manifest; generated by acp_executor.",
		},
	}
	if len(attachments) > 0 {
		for name, att := range attachments {
			item := map[string]any{
				"type": "attachment",
				"name": name,
			}
			if att.MediaType != "" {
				item["media_type"] = att.MediaType
			}
			if att.Kind != "" {
				item["kind"] = att.Kind
			}
			if att.Format != "" {
				item["format"] = att.Format
			}
			if att.URI != "" {
				item["uri"] = att.URI
			}
			items = append(items, item)
		}
	}
	return map[string]any{
		"items":                     items,
		"summary":                   "Auto-generated artifact manifest (executor did not emit artifact_manifest).",
		"generated_at":              time.Now().UTC().Format(time.RFC3339Nano),
		"generated_by":              "acp_executor",
		"missing_executor_manifest": true,
	}
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
	callID := strings.TrimSpace(shared.StringArg(update, "toolCallId"))
	if callID == "" {
		return
	}
	title := strings.TrimSpace(shared.StringArg(update, "title"))
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
	callID := strings.TrimSpace(shared.StringArg(update, "toolCallId"))
	if callID == "" {
		return
	}
	status := strings.ToLower(strings.TrimSpace(shared.StringArg(update, "status")))
	rawOutput := shared.StringArg(update, "rawOutput")

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
		content := strings.TrimSpace(shared.StringArg(item, "content"))
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
	parentTaskID := h.call.ID
	if parentTaskID == "" {
		parentTaskID = h.call.ParentTaskID
	}
	env := &domain.WorkflowEventEnvelope{
		BaseEvent: domain.NewBaseEvent(agent.LevelSubagent, h.call.SessionID, taskID, parentTaskID, ts),
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
	sessionID := strings.TrimSpace(shared.StringArg(params, "sessionId"))
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

func parseContentBlock(block map[string]any) (string, map[string]ports.Attachment) {
	if block == nil {
		return "", nil
	}
	blockType := strings.ToLower(shared.StringArg(block, "type"))
	switch blockType {
	case "text":
		return strings.TrimSpace(shared.StringArg(block, "text")), nil
	case "image":
		name := attachmentNameForMedia("image", shared.StringArg(block, "mimeType"))
		return "", map[string]ports.Attachment{
			name: {
				Name:      name,
				MediaType: strings.TrimSpace(shared.StringArg(block, "mimeType")),
				Data:      strings.TrimSpace(shared.StringArg(block, "data")),
			},
		}
	case "audio":
		name := attachmentNameForMedia("audio", shared.StringArg(block, "mimeType"))
		return "", map[string]ports.Attachment{
			name: {
				Name:      name,
				MediaType: strings.TrimSpace(shared.StringArg(block, "mimeType")),
				Data:      strings.TrimSpace(shared.StringArg(block, "data")),
			},
		}
	case "resource_link":
		name := strings.TrimSpace(shared.StringArg(block, "name"))
		uri := strings.TrimSpace(shared.StringArg(block, "uri"))
		if name == "" && uri != "" {
			name = path.Base(uri)
		}
		if name == "" {
			name = fmt.Sprintf("resource-%d", time.Now().UnixNano())
		}
		att := ports.Attachment{
			Name:        name,
			URI:         uri,
			MediaType:   strings.TrimSpace(shared.StringArg(block, "mimeType")),
			Description: strings.TrimSpace(shared.StringArg(block, "description")),
		}
		return "", map[string]ports.Attachment{name: att}
	case "resource":
		resource, ok := block["resource"].(map[string]any)
		if !ok {
			return "", nil
		}
		uri := strings.TrimSpace(shared.StringArg(resource, "uri"))
		name := ""
		if uri != "" {
			name = path.Base(uri)
		}
		if name == "" {
			name = fmt.Sprintf("resource-%d", time.Now().UnixNano())
		}
		mimeType := strings.TrimSpace(shared.StringArg(resource, "mimeType"))
		if text := strings.TrimSpace(shared.StringArg(resource, "text")); text != "" {
			return "", map[string]ports.Attachment{name: {
				Name:      name,
				URI:       uri,
				MediaType: mimeType,
				Data:      base64.StdEncoding.EncodeToString([]byte(text)),
			}}
		}
		if blob := strings.TrimSpace(shared.StringArg(resource, "blob")); blob != "" {
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

func parentListenerFromContext(ctx context.Context) agent.EventListener {
	return shared.GetParentListenerFromContext(ctx)
}
