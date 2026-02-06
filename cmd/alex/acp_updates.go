package main

import (
	"strings"
	"sync"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

type acpEventListener struct {
	server  *acpServer
	session *acpSession

	mu            sync.Mutex
	sawAgentChunk bool
	toolStates    map[string]*acpToolState
	planEntries   []map[string]any
}

type acpToolState struct {
	title     string
	kind      string
	rawInput  map[string]any
	locations []map[string]any
}

func newACPEventListener(server *acpServer, session *acpSession) *acpEventListener {
	return &acpEventListener{
		server:     server,
		session:    session,
		toolStates: make(map[string]*acpToolState),
	}
}

func (l *acpEventListener) OnEvent(event agent.AgentEvent) {
	if event == nil {
		return
	}
	if env, ok := event.(*domain.WorkflowEventEnvelope); ok && env != nil {
		l.handleEnvelope(env)
		return
	}
	if env := envelopeFromDomainEvent(event); env != nil {
		l.handleEnvelope(env)
	}
}

func (l *acpEventListener) handleEnvelope(env *domain.WorkflowEventEnvelope) {
	if env == nil {
		return
	}
	switch env.Event {
	case "workflow.tool.started":
		l.handleToolStarted(env)
	case "workflow.tool.progress":
		l.handleToolProgress(env)
	case "workflow.tool.completed":
		l.handleToolCompleted(env)
	case "workflow.node.output.delta":
		l.handleOutputDelta(env)
	case "workflow.result.final":
		l.handleResultFinal(env)
	}
}

func envelopeFromDomainEvent(event agent.AgentEvent) *domain.WorkflowEventEnvelope {
	switch e := event.(type) {
	case *domain.WorkflowNodeOutputDeltaEvent:
		payload := map[string]any{
			"iteration": e.Iteration,
			"delta":     e.Delta,
			"final":     e.Final,
		}
		if !e.CreatedAt.IsZero() {
			payload["created_at"] = e.CreatedAt
		}
		if e.SourceModel != "" {
			payload["source_model"] = e.SourceModel
		}
		if e.MessageCount > 0 {
			payload["message_count"] = e.MessageCount
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent: e.BaseEvent,
			Event:     e.EventType(),
			NodeKind:  "generation",
			Payload:   payload,
		}
	case *domain.WorkflowToolStartedEvent:
		payload := map[string]any{
			"tool_name": e.ToolName,
			"arguments": e.Arguments,
			"iteration": e.Iteration,
			"call_id":   e.CallID,
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent: e.BaseEvent,
			Event:     e.EventType(),
			NodeID:    e.CallID,
			NodeKind:  "tool",
			Payload:   payload,
		}
	case *domain.WorkflowToolProgressEvent:
		payload := map[string]any{
			"chunk":       e.Chunk,
			"is_complete": e.IsComplete,
			"call_id":     e.CallID,
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent: e.BaseEvent,
			Event:     e.EventType(),
			NodeID:    e.CallID,
			NodeKind:  "tool",
			Payload:   payload,
		}
	case *domain.WorkflowToolCompletedEvent:
		payload := map[string]any{
			"tool_name":   e.ToolName,
			"result":      e.Result,
			"duration":    e.Duration.Milliseconds(),
			"metadata":    e.Metadata,
			"attachments": e.Attachments,
			"call_id":     e.CallID,
		}
		if e.Error != nil {
			payload["error"] = e.Error.Error()
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent: e.BaseEvent,
			Event:     e.EventType(),
			NodeID:    e.CallID,
			NodeKind:  "tool",
			Payload:   payload,
		}
	case *domain.WorkflowResultFinalEvent:
		payload := map[string]any{
			"final_answer":     e.FinalAnswer,
			"total_iterations": e.TotalIterations,
			"total_tokens":     e.TotalTokens,
			"stop_reason":      e.StopReason,
			"duration":         e.Duration.Milliseconds(),
			"is_streaming":     e.IsStreaming,
			"stream_finished":  e.StreamFinished,
			"attachments":      e.Attachments,
		}
		return &domain.WorkflowEventEnvelope{
			BaseEvent: e.BaseEvent,
			Event:     e.EventType(),
			NodeKind:  "result",
			Payload:   payload,
		}
	default:
		return nil
	}
}

func (l *acpEventListener) handleOutputDelta(env *domain.WorkflowEventEnvelope) {
	delta := strings.TrimSpace(stringParam(env.Payload, "delta"))
	if delta == "" {
		return
	}
	l.mu.Lock()
	l.sawAgentChunk = true
	l.mu.Unlock()

	l.server.sendSessionUpdate(l.session.id, map[string]any{
		"sessionUpdate": "agent_message_chunk",
		"content": map[string]any{
			"type": "text",
			"text": delta,
		},
	})
}

func (l *acpEventListener) handleResultFinal(env *domain.WorkflowEventEnvelope) {
	finalAnswer := strings.TrimSpace(stringParam(env.Payload, "final_answer"))
	isStreaming := boolParam(env.Payload, "is_streaming")
	streamFinished := boolParam(env.Payload, "stream_finished")

	l.mu.Lock()
	shouldSend := !l.sawAgentChunk && finalAnswer != ""
	if isStreaming && finalAnswer != "" && !l.sawAgentChunk {
		shouldSend = true
	}
	l.mu.Unlock()

	if shouldSend {
		l.server.sendSessionUpdate(l.session.id, map[string]any{
			"sessionUpdate": "agent_message_chunk",
			"content": map[string]any{
				"type": "text",
				"text": finalAnswer,
			},
		})
	}

	if streamFinished {
		l.sendPlanCompletion()
	}

	attachments := attachmentsFromPayload(env.Payload["attachments"])
	if len(attachments) > 0 {
		for _, block := range contentBlocksForAttachments(attachments) {
			l.server.sendSessionUpdate(l.session.id, map[string]any{
				"sessionUpdate": "agent_message_chunk",
				"content":       block,
			})
		}
	}
}

func (l *acpEventListener) handleToolStarted(env *domain.WorkflowEventEnvelope) {
	callID := toolCallID(env)
	if callID == "" {
		return
	}
	toolName := strings.TrimSpace(stringParam(env.Payload, "tool_name"))
	args := mapParam(env.Payload, "arguments")
	kind := toolKindForName(toolName)
	locations := extractToolLocations(args, mapParam(env.Payload, "metadata"))

	state := &acpToolState{
		title:     toolName,
		kind:      kind,
		rawInput:  args,
		locations: locations,
	}
	l.mu.Lock()
	l.toolStates[callID] = state
	l.mu.Unlock()

	update := map[string]any{
		"sessionUpdate": "tool_call",
		"toolCallId":    callID,
		"title":         toolName,
		"status":        "in_progress",
	}
	if kind != "" {
		update["kind"] = kind
	}
	if len(locations) > 0 {
		update["locations"] = locations
	}
	if len(args) > 0 {
		update["rawInput"] = args
	}

	l.server.sendSessionUpdate(l.session.id, update)
}

func (l *acpEventListener) handleToolProgress(env *domain.WorkflowEventEnvelope) {
	callID := toolCallID(env)
	if callID == "" {
		return
	}
	chunk := strings.TrimSpace(stringParam(env.Payload, "chunk"))
	if chunk == "" {
		return
	}

	update := map[string]any{
		"sessionUpdate": "tool_call_update",
		"toolCallId":    callID,
		"status":        "in_progress",
		"rawOutput":     chunk,
		"content": []any{
			map[string]any{
				"type": "text",
				"text": chunk,
			},
		},
	}

	l.mu.Lock()
	if state := l.toolStates[callID]; state != nil && state.kind != "" {
		update["kind"] = state.kind
	}
	l.mu.Unlock()

	l.server.sendSessionUpdate(l.session.id, update)
}

func (l *acpEventListener) handleToolCompleted(env *domain.WorkflowEventEnvelope) {
	callID := toolCallID(env)
	if callID == "" {
		return
	}

	toolName := strings.TrimSpace(stringParam(env.Payload, "tool_name"))
	result := strings.TrimSpace(stringParam(env.Payload, "result"))
	errMsg := strings.TrimSpace(stringParam(env.Payload, "error"))
	metadata := mapParam(env.Payload, "metadata")
	attachments := attachmentsFromPayload(env.Payload["attachments"])

	status := "completed"
	if errMsg != "" {
		status = "failed"
	}

	content := toolContentBlocks(result, errMsg, metadata, attachments)
	update := map[string]any{
		"sessionUpdate": "tool_call_update",
		"toolCallId":    callID,
		"status":        status,
	}
	if len(content) > 0 {
		update["content"] = content
	}
	if result != "" {
		update["rawOutput"] = result
	} else if errMsg != "" {
		update["rawOutput"] = errMsg
	}

	state := l.getToolState(callID)
	if state != nil {
		if state.kind != "" {
			update["kind"] = state.kind
		}
		if len(state.locations) > 0 {
			update["locations"] = state.locations
		}
		locations := extractToolLocations(state.rawInput, metadata)
		if len(locations) > 0 {
			update["locations"] = locations
		}
	}
	if state == nil {
		locations := extractToolLocations(nil, metadata)
		if len(locations) > 0 {
			update["locations"] = locations
		}
	}

	l.server.sendSessionUpdate(l.session.id, update)

	if toolName == "plan" {
		l.sendPlanUpdate(metadata)
	}
}

func (l *acpEventListener) getToolState(callID string) *acpToolState {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.toolStates[callID]
}

func (l *acpEventListener) sendPlanUpdate(metadata map[string]any) {
	if metadata == nil {
		return
	}
	goal := strings.TrimSpace(stringParam(metadata, "overall_goal_ui"))
	if goal == "" {
		return
	}
	entry := map[string]any{
		"content":  goal,
		"priority": "medium",
		"status":   "in_progress",
	}

	l.mu.Lock()
	l.planEntries = []map[string]any{entry}
	l.mu.Unlock()

	l.server.sendSessionUpdate(l.session.id, map[string]any{
		"sessionUpdate": "plan",
		"entries":       l.planEntries,
	})
}

func (l *acpEventListener) sendPlanCompletion() {
	l.mu.Lock()
	if len(l.planEntries) == 0 {
		l.mu.Unlock()
		return
	}
	completed := make([]map[string]any, 0, len(l.planEntries))
	for _, entry := range l.planEntries {
		clone := map[string]any{}
		for k, v := range entry {
			clone[k] = v
		}
		clone["status"] = "completed"
		completed = append(completed, clone)
	}
	l.planEntries = completed
	l.mu.Unlock()

	l.server.sendSessionUpdate(l.session.id, map[string]any{
		"sessionUpdate": "plan",
		"entries":       completed,
	})
}

func sendUserPromptUpdates(server *acpServer, sessionID string, input acpPromptInput) {
	if server == nil || sessionID == "" {
		return
	}
	if strings.TrimSpace(input.Text) != "" {
		server.sendSessionUpdate(sessionID, map[string]any{
			"sessionUpdate": "user_message_chunk",
			"content": map[string]any{
				"type": "text",
				"text": input.Text,
			},
		})
	}
	for _, block := range contentBlocksForAttachments(input.Attachments) {
		server.sendSessionUpdate(sessionID, map[string]any{
			"sessionUpdate": "user_message_chunk",
			"content":       block,
		})
	}
}

func toolCallID(env *domain.WorkflowEventEnvelope) string {
	if env == nil {
		return ""
	}
	if env.NodeID != "" {
		return env.NodeID
	}
	return strings.TrimSpace(stringParam(env.Payload, "call_id"))
}

func attachmentsFromPayload(value any) []ports.Attachment {
	if value == nil {
		return nil
	}
	switch typed := value.(type) {
	case map[string]ports.Attachment:
		out := make([]ports.Attachment, 0, len(typed))
		for _, att := range typed {
			out = append(out, att)
		}
		return out
	case map[string]any:
		out := make([]ports.Attachment, 0, len(typed))
		for _, raw := range typed {
			if att, ok := raw.(ports.Attachment); ok {
				out = append(out, att)
				continue
			}
			if attMap, ok := raw.(map[string]any); ok {
				out = append(out, attachmentFromMap(attMap))
			}
		}
		return out
	default:
		return nil
	}
}

func attachmentFromMap(m map[string]any) ports.Attachment {
	if m == nil {
		return ports.Attachment{}
	}
	return ports.Attachment{
		Name:        strings.TrimSpace(stringParam(m, "name")),
		MediaType:   strings.TrimSpace(stringParam(m, "media_type")),
		Data:        strings.TrimSpace(stringParam(m, "data")),
		URI:         strings.TrimSpace(stringParam(m, "uri")),
		Source:      strings.TrimSpace(stringParam(m, "source")),
		Description: strings.TrimSpace(stringParam(m, "description")),
		Kind:        strings.TrimSpace(stringParam(m, "kind")),
		Format:      strings.TrimSpace(stringParam(m, "format")),
	}
}

func contentBlocksForAttachments(attachments []ports.Attachment) []map[string]any {
	if len(attachments) == 0 {
		return nil
	}
	out := make([]map[string]any, 0, len(attachments))
	for _, att := range attachments {
		if block := attachmentToContentBlock(att); block != nil {
			out = append(out, block)
		}
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

func toolContentBlocks(result, errMsg string, metadata map[string]any, attachments []ports.Attachment) []any {
	blocks := make([]any, 0, 2+len(attachments))
	if result != "" {
		blocks = append(blocks, map[string]any{
			"type": "text",
			"text": result,
		})
	}
	if errMsg != "" {
		blocks = append(blocks, map[string]any{
			"type": "text",
			"text": "Error: " + errMsg,
		})
	}
	if metadata != nil {
		if diff, ok := metadata["diff"].(string); ok && strings.TrimSpace(diff) != "" {
			blocks = append(blocks, map[string]any{
				"type": "text",
				"text": diff,
			})
		}
	}
	for _, block := range contentBlocksForAttachments(attachments) {
		blocks = append(blocks, block)
	}
	return blocks
}

func extractToolLocations(args map[string]any, metadata map[string]any) []map[string]any {
	paths := make(map[string]bool)
	addPath := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		paths[value] = true
	}

	for _, key := range []string{"file_path", "path", "resolved_path"} {
		if args != nil {
			if val, ok := args[key].(string); ok {
				addPath(val)
			}
		}
		if metadata != nil {
			if val, ok := metadata[key].(string); ok {
				addPath(val)
			}
		}
	}

	locations := make([]map[string]any, 0, len(paths))
	for path := range paths {
		locations = append(locations, map[string]any{
			"path": path,
		})
	}
	return locations
}

func toolKindForName(name string) string {
	name = strings.ToLower(strings.TrimSpace(name))
	switch {
	case name == "file_read" || name == "list_files":
		return "read"
	case name == "file_write" || name == "file_edit" || name == "html_edit":
		return "edit"
	case name == "file_delete":
		return "delete"
	case name == "find" || name == "grep" || name == "ripgrep" || strings.Contains(name, "search"):
		return "search"
	case name == "bash" || name == "code_execute" || strings.HasPrefix(name, "sandbox_shell"):
		return "execute"
	case name == "plan" || name == "clarify" || name == "attention":
		return "think"
	case name == "web_fetch" || name == "douyin_hot":
		return "fetch"
	default:
		return "other"
	}
}
