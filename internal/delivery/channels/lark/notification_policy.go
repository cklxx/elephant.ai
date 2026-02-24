package lark

import (
	"strings"

	domain "alex/internal/domain/agent"
	agentports "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

type notificationMode string

const (
	notificationModeSilentUpdate notificationMode = "silent_update"
	notificationModeMilestone    notificationMode = "milestone"
	notificationModeBlocking     notificationMode = "blocking"
)

func (m notificationMode) String() string {
	if strings.TrimSpace(string(m)) == "" {
		return string(notificationModeMilestone)
	}
	return string(m)
}

type notificationPolicy struct {
	enabled bool
}

func newNotificationPolicy(enabled bool) notificationPolicy {
	return notificationPolicy{enabled: enabled}
}

func (p notificationPolicy) modeForEvent(event agentports.AgentEvent) notificationMode {
	switch e := event.(type) {
	case *domain.Event:
		return p.modeForUnifiedEvent(e)
	case *domain.WorkflowEventEnvelope:
		return p.modeForEnvelopeEvent(e)
	default:
		return notificationModeMilestone
	}
}

func (p notificationPolicy) modeForUnifiedEvent(e *domain.Event) notificationMode {
	if e == nil {
		return notificationModeMilestone
	}
	switch strings.TrimSpace(e.Kind) {
	case types.EventExternalInputRequested:
		return notificationModeBlocking
	case types.EventNodeStarted, types.EventExternalAgentProgress:
		return notificationModeSilentUpdate
	case types.EventToolCompleted:
		if needsUserInputForTool(e.Data.ToolName, e.Data.Metadata) {
			return notificationModeBlocking
		}
		return notificationModeMilestone
	default:
		return notificationModeMilestone
	}
}

func (p notificationPolicy) modeForEnvelopeEvent(e *domain.WorkflowEventEnvelope) notificationMode {
	if e == nil {
		return notificationModeMilestone
	}
	switch strings.TrimSpace(e.Event) {
	case types.EventExternalInputRequested:
		return notificationModeBlocking
	case types.EventNodeStarted:
		return notificationModeSilentUpdate
	case types.EventExternalAgentProgress:
		if !hasMeaningfulBackgroundProgress(e) {
			return notificationModeSilentUpdate
		}
		return notificationModeMilestone
	case types.EventToolCompleted:
		metadata, _ := e.Payload["metadata"].(map[string]any)
		if needsUserInputForTool(envelopeToolName(e), metadata) {
			return notificationModeBlocking
		}
		return notificationModeMilestone
	default:
		return notificationModeMilestone
	}
}

func (p notificationPolicy) allowProgressEvent(eventType string) bool {
	if !p.enabled {
		return true
	}
	mode := notificationModeMilestone
	if strings.TrimSpace(eventType) == types.EventNodeStarted {
		mode = notificationModeSilentUpdate
	}
	return mode != notificationModeSilentUpdate
}

func (p notificationPolicy) allowBackgroundProgress(env *domain.WorkflowEventEnvelope) bool {
	if !p.enabled {
		return true
	}
	return hasMeaningfulBackgroundProgress(env)
}

func (p notificationPolicy) modeForPlanClarify(payload planClarifyPayload) notificationMode {
	if payload.needsInput {
		return notificationModeBlocking
	}
	return notificationModeMilestone
}

func (p notificationPolicy) allowPlanClarify(payload planClarifyPayload) bool {
	if !p.enabled {
		return true
	}
	return p.modeForPlanClarify(payload) == notificationModeBlocking
}

func needsUserInputForTool(toolName string, metadata map[string]any) bool {
	if !strings.EqualFold(strings.TrimSpace(toolName), "clarify") {
		return false
	}
	return boolMeta(metadata, "needs_user_input")
}

func hasMeaningfulBackgroundProgress(env *domain.WorkflowEventEnvelope) bool {
	if env == nil {
		return false
	}
	if strings.TrimSpace(env.Event) != types.EventExternalAgentProgress {
		return true
	}
	if env.Payload == nil {
		return false
	}
	currentTool := asString(env.Payload["current_tool"])
	if currentTool == "__heartbeat__" {
		return false
	}
	if currentTool != "" {
		return true
	}
	if args := asString(env.Payload["current_args"]); args != "" {
		return true
	}
	if files := asStringSlice(env.Payload["files_touched"]); len(files) > 0 {
		return true
	}
	if asInt(env.Payload["tokens_used"]) > 0 {
		return true
	}
	return false
}
