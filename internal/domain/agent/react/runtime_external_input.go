package react

import (
	"fmt"
	"strings"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
)

// injectExternalInputRequests drains external agent input requests and injects
// system messages so the core agent can respond via ext_reply.
func (r *reactRuntime) injectExternalInputRequests() {
	if r.externalInputCh == nil {
		return
	}

	for {
		select {
		case req, ok := <-r.externalInputCh:
			if !ok {
				return
			}
			key := fmt.Sprintf("%s:%s", req.TaskID, req.RequestID)
			if r.externalInputEmitted != nil {
				if r.externalInputEmitted[key] {
					continue
				}
				r.externalInputEmitted[key] = true
			}
			msg := formatExternalInputRequestMessage(req)
			r.state.Messages = append(r.state.Messages, ports.Message{
				Role:    "user",
				Content: msg,
				Source:  ports.MessageSourceSystemPrompt,
			})

			r.engine.emitEvent(&domain.ExternalInputRequestEvent{
				BaseEvent: r.engine.newBaseEvent(r.ctx, r.state.SessionID, r.state.RunID, r.state.ParentRunID),
				TaskID:    req.TaskID,
				AgentType: req.AgentType,
				RequestID: req.RequestID,
				Type:      string(req.Type),
				Summary:   req.Summary,
			})
		default:
			return
		}
	}
}

func formatExternalInputRequestMessage(req agent.InputRequest) string {
	var sb strings.Builder
	sb.WriteString("[External Agent Input Required]")
	if req.TaskID != "" {
		sb.WriteString(fmt.Sprintf(" task_id=%q", req.TaskID))
	}
	if req.AgentType != "" {
		sb.WriteString(fmt.Sprintf(" agent_type=%q", req.AgentType))
	}
	sb.WriteString("\n")
	if req.Summary != "" {
		sb.WriteString("Request: ")
		sb.WriteString(req.Summary)
		sb.WriteString("\n")
	}
	if req.ToolCall != nil {
		sb.WriteString("Tool: ")
		sb.WriteString(req.ToolCall.Name)
		if len(req.ToolCall.Arguments) > 0 {
			sb.WriteString(fmt.Sprintf(" %v", req.ToolCall.Arguments))
		}
		sb.WriteString("\n")
		if len(req.ToolCall.FilePaths) > 0 {
			sb.WriteString("Files: ")
			sb.WriteString(strings.Join(req.ToolCall.FilePaths, ", "))
			sb.WriteString("\n")
		}
	}
	if len(req.Options) > 0 {
		sb.WriteString("Options: ")
		for idx, opt := range req.Options {
			if idx > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString(fmt.Sprintf("[%s]", opt.ID))
		}
		sb.WriteString("\n")
	}
	sb.WriteString(fmt.Sprintf("Use ext_reply(task_id=%q, request_id=%q, approved=true|false) to respond.", req.TaskID, req.RequestID))
	return sb.String()
}
