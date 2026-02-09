package lark

import (
	"context"
	"fmt"
	"strings"
	"sync"

	"alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/logging"
)

// inputRequestListener bridges external agent input requests to Lark text-based interaction.
// When a background task (CC/Codex) requests permission or input, it formats a numbered
// options list and waits for the user's reply.
type inputRequestListener struct {
	inner   agentports.EventListener
	ctx     context.Context
	g       *Gateway
	chatID  string
	replyTo string
	logger  logging.Logger

	mu             sync.Mutex
	pendingRelay   map[string]*pendingInputRelay // chatID → relay
	closed         bool
}

// pendingInputRelay tracks a pending input request awaiting user response.
type pendingInputRelay struct {
	taskID    string
	requestID string
	agentType string
	options   []agentports.InputOption
	reqType   agentports.InputRequestType
}

func newInputRequestListener(
	ctx context.Context,
	inner agentports.EventListener,
	g *Gateway,
	chatID string,
	replyTo string,
	logger logging.Logger,
) *inputRequestListener {
	return &inputRequestListener{
		inner:        inner,
		ctx:          ctx,
		g:            g,
		chatID:       chatID,
		replyTo:      replyTo,
		logger:       logging.OrNop(logger),
		pendingRelay: make(map[string]*pendingInputRelay),
	}
}

func (l *inputRequestListener) Close() {
	l.mu.Lock()
	l.closed = true
	l.mu.Unlock()
}

func (l *inputRequestListener) OnEvent(event agentports.AgentEvent) {
	if l.inner != nil {
		l.inner.OnEvent(event)
	}

	switch e := event.(type) {
	case *domain.WorkflowEventEnvelope:
		l.onEnvelope(e)
	}
}

func (l *inputRequestListener) onEnvelope(env *domain.WorkflowEventEnvelope) {
	if env == nil {
		return
	}

	if strings.TrimSpace(env.Event) != types.EventExternalInputRequested {
		return
	}

	l.onInputRequested(env)
}

func (l *inputRequestListener) onInputRequested(env *domain.WorkflowEventEnvelope) {
	taskID := asString(env.Payload["task_id"])
	requestID := asString(env.Payload["request_id"])
	agentType := asString(env.Payload["agent_type"])
	summary := asString(env.Payload["summary"])
	reqType := asString(env.Payload["type"])

	if taskID == "" || requestID == "" {
		return
	}

	l.mu.Lock()
	if l.closed {
		l.mu.Unlock()
		return
	}

	// Parse options from payload
	var options []agentports.InputOption
	if rawOpts, ok := env.Payload["options"].([]any); ok {
		for _, raw := range rawOpts {
			if m, ok := raw.(map[string]any); ok {
				opt := agentports.InputOption{
					ID:          asString(m["id"]),
					Label:       asString(m["label"]),
					Description: asString(m["description"]),
				}
				if opt.ID != "" && opt.Label != "" {
					options = append(options, opt)
				}
			}
		}
	}

	// Store pending relay for response routing
	relay := &pendingInputRelay{
		taskID:    taskID,
		requestID: requestID,
		agentType: agentType,
		options:   options,
		reqType:   agentports.InputRequestType(reqType),
	}
	l.pendingRelay[l.chatID] = relay
	l.mu.Unlock()

	// Format and send the request to Lark
	text := l.formatInputRequest(taskID, agentType, summary, reqType, options)
	if _, err := l.g.dispatchMessage(l.ctx, l.chatID, l.replyTo, "text", textContent(text)); err != nil {
		l.logger.Warn("Input request dispatch failed: %v", err)
	}
}

// formatInputRequest formats an input request as a numbered options text message.
func (l *inputRequestListener) formatInputRequest(taskID, agentType, summary, reqType string, options []agentports.InputOption) string {
	var sb strings.Builder

	typeLabel := "输入请求"
	if reqType == string(agentports.InputRequestPermission) {
		typeLabel = "权限请求"
	}

	sb.WriteString(fmt.Sprintf("[任务 %s] %s\n", shortID(taskID), typeLabel))
	if agentType != "" {
		sb.WriteString(fmt.Sprintf("%s 请求确认:\n", agentType))
	}
	if summary != "" {
		sb.WriteString(truncateForLark(summary, 400))
		sb.WriteString("\n")
	}

	if len(options) > 0 {
		optLabels := make([]string, 0, len(options))
		for _, opt := range options {
			label := opt.Label
			if opt.Description != "" {
				label += " - " + opt.Description
			}
			optLabels = append(optLabels, label)
		}
		sb.WriteString("\n")
		sb.WriteString(formatNumberedOptions("选择操作:", optLabels))
	} else {
		// Default permission options
		sb.WriteString("\n")
		defaultOpts := []string{"同意", "拒绝", "同意并记住"}
		sb.WriteString(formatNumberedOptions("选择操作:", defaultOpts))
	}

	return sb.String()
}

// TryResolveInputReply checks if a user message is a reply to a pending input request.
// Returns true if the reply was handled.
func (l *inputRequestListener) TryResolveInputReply(ctx context.Context, chatID, content string) bool {
	l.mu.Lock()
	relay, ok := l.pendingRelay[chatID]
	if !ok {
		l.mu.Unlock()
		return false
	}
	delete(l.pendingRelay, chatID)
	l.mu.Unlock()

	resp := l.buildResponse(relay, content)

	// Try to reply through the gateway's agent if it supports external input
	if responder, ok := l.g.agent.(agentports.ExternalInputResponder); ok {
		if err := responder.ReplyExternalInput(ctx, resp); err != nil {
			l.logger.Warn("External input reply failed: %v", err)
			return false
		}
		return true
	}

	l.logger.Warn("Agent does not support ExternalInputResponder interface")
	return false
}

// buildResponse converts user text input into an InputResponse.
func (l *inputRequestListener) buildResponse(relay *pendingInputRelay, content string) agentports.InputResponse {
	trimmed := strings.TrimSpace(content)
	resp := agentports.InputResponse{
		TaskID:    relay.taskID,
		RequestID: relay.requestID,
	}

	if isSkipReply(trimmed) {
		resp.Approved = false
		resp.Text = "skipped"
		return resp
	}

	if len(relay.options) > 0 {
		// Try numbered reply against options
		optLabels := make([]string, 0, len(relay.options))
		for _, opt := range relay.options {
			optLabels = append(optLabels, opt.Label)
		}
		selected := parseNumberedReply(trimmed, optLabels)
		for _, opt := range relay.options {
			if selected == opt.Label {
				resp.OptionID = opt.ID
				resp.Approved = true
				return resp
			}
		}
	}

	// Default permission handling
	switch {
	case trimmed == "1" || strings.EqualFold(trimmed, "同意"):
		resp.Approved = true
	case trimmed == "2" || strings.EqualFold(trimmed, "拒绝"):
		resp.Approved = false
	case trimmed == "3" || strings.EqualFold(trimmed, "同意并记住"):
		resp.Approved = true
		resp.Text = "remember"
	default:
		// Free text response
		resp.Text = trimmed
		resp.Approved = true
	}

	return resp
}

// parseMultiNumberedReply resolves comma-separated numeric replies against option list.
// E.g., "1,3" with ["a","b","c"] → ["a","c"]
func parseMultiNumberedReply(input string, options []string) []string {
	trimmed := strings.TrimSpace(input)
	if trimmed == "" || len(options) == 0 {
		return nil
	}

	parts := strings.Split(trimmed, ",")
	var result []string
	for _, part := range parts {
		resolved := parseNumberedReply(strings.TrimSpace(part), options)
		if resolved != strings.TrimSpace(part) { // was numeric
			result = append(result, resolved)
		}
	}
	return result
}

// isSkipReply checks if the input is a skip/pass command.
func isSkipReply(input string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(input))
	return trimmed == "skip" || trimmed == "跳过" || trimmed == "pass"
}
