package lark

import (
	"fmt"
	"strings"
	"time"

	domain "alex/internal/domain/agent"
	"alex/internal/domain/agent/types"
	"alex/internal/shared/utils"
)

func isSlowSummaryTerminalEvent(eventType string) bool {
	switch strings.TrimSpace(eventType) {
	case types.EventResultFinal, types.EventResultCancelled:
		return true
	default:
		return false
	}
}

// normalizedEvent holds the fields needed to build a slowProgressSignal,
// extracted from either a WorkflowEventEnvelope or a unified Event.
type normalizedEvent struct {
	kind            string
	at              time.Time
	stepDescription string
	nodeID          string
	toolName        string
	errText         string
	content         string
}

func normalizeEnvelope(e *domain.WorkflowEventEnvelope) (normalizedEvent, bool) {
	if e == nil {
		return normalizedEvent{}, false
	}
	kind := strings.TrimSpace(e.Event)
	if kind == "" {
		return normalizedEvent{}, false
	}
	return normalizedEvent{
		kind:            kind,
		at:              e.Timestamp(),
		stepDescription: asString(e.Payload["step_description"]),
		nodeID:          e.NodeID,
		toolName:        strings.TrimSpace(envelopeToolName(e)),
		errText:         strings.TrimSpace(asString(e.Payload["error"])),
		content:         asString(e.Payload["content"]),
	}, true
}

func normalizeUnified(e *domain.Event) (normalizedEvent, bool) {
	if e == nil {
		return normalizedEvent{}, false
	}
	kind := strings.TrimSpace(e.Kind)
	if kind == "" {
		return normalizedEvent{}, false
	}
	var errText string
	if e.Data.Error != nil {
		errText = e.Data.Error.Error()
	}
	return normalizedEvent{
		kind:            kind,
		at:              e.Timestamp(),
		stepDescription: e.Data.StepDescription,
		nodeID:          "",
		toolName:        strings.TrimSpace(e.Data.ToolName),
		errText:         strings.TrimSpace(errText),
		content:         e.Data.Content,
	}, true
}

func signalFromEnvelope(e *domain.WorkflowEventEnvelope) (slowProgressSignal, bool) {
	n, ok := normalizeEnvelope(e)
	if !ok {
		return slowProgressSignal{}, false
	}
	return buildSignal(n)
}

func signalFromUnified(e *domain.Event) (slowProgressSignal, bool) {
	n, ok := normalizeUnified(e)
	if !ok {
		return slowProgressSignal{}, false
	}
	return buildSignal(n)
}

func buildSignal(n normalizedEvent) (slowProgressSignal, bool) {
	switch n.kind {
	case types.EventNodeStarted:
		step := resolveSlowProgressStepLabel(n.stepDescription, n.nodeID)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: n.at, text: "开始步骤：" + step}, true
	case types.EventNodeCompleted:
		step := resolveSlowProgressStepLabel(n.stepDescription, n.nodeID)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: n.at, text: "完成步骤：" + step}, true
	case types.EventToolStarted:
		if n.toolName == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: n.at, text: "开始工具：" + n.toolName}, true
	case types.EventToolCompleted:
		toolName := n.toolName
		if toolName == "" {
			toolName = "tool"
		}
		if n.errText != "" {
			return slowProgressSignal{
				at:   n.at,
				text: "工具失败：" + toolName + "（" + truncateForLark(n.errText, 80) + ")",
			}, true
		}
		return slowProgressSignal{at: n.at, text: "完成工具：" + toolName}, true
	case types.EventNodeOutputSummary:
		content := sanitizeSlowProgressContent(n.content)
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: n.at, text: "阶段输出：" + content}, true
	default:
		return slowProgressSignal{}, false
	}
}

func isValidSlowProgressLLMSummary(summary string) bool {
	trimmed := strings.TrimSpace(summary)
	if trimmed == "" {
		return false
	}
	lower := strings.ToLower(trimmed)
	if strings.HasPrefix(lower, "empty response:") || strings.HasPrefix(lower, "empty completion:") {
		return false
	}
	return !containsInternalProgressIdentifier(trimmed)
}

func resolveSlowProgressStepLabel(stepDescription string, nodeID string) string {
	step := strings.TrimSpace(stepDescription)
	if step != "" {
		return truncateForLark(step, 120)
	}
	humanized := humanizeSlowProgressNodeID(nodeID)
	if humanized == "" {
		return ""
	}
	return truncateForLark(humanized, 120)
}

func humanizeSlowProgressNodeID(nodeID string) string {
	nodeID = strings.TrimSpace(nodeID)
	if nodeID == "" {
		return ""
	}
	if strings.HasPrefix(nodeID, "react:iter:") {
		parts := strings.Split(nodeID, ":")
		if len(parts) >= 4 {
			iter := strings.TrimSpace(parts[2])
			if iter == "" {
				iter = "?"
			}
			switch strings.TrimSpace(parts[3]) {
			case "think":
				return fmt.Sprintf("第 %s 轮思考", iter)
			case "plan":
				return fmt.Sprintf("第 %s 轮规划", iter)
			case "tools":
				return fmt.Sprintf("第 %s 轮工具执行", iter)
			case "tool":
				if len(parts) >= 5 {
					toolNode := strings.TrimSpace(parts[4])
					if toolNode != "" && !isOpaqueToolCallID(toolNode) {
						return fmt.Sprintf("第 %s 轮工具调用（%s）", iter, toolNode)
					}
				}
				return fmt.Sprintf("第 %s 轮工具调用", iter)
			default:
				return fmt.Sprintf("第 %s 轮执行", iter)
			}
		}
	}
	if containsInternalProgressIdentifier(nodeID) {
		return ""
	}
	return nodeID
}

func sanitizeSlowProgressContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return ""
	}
	if containsInternalProgressIdentifier(content) {
		return ""
	}
	return truncateForLark(content, 120)
}

func containsInternalProgressIdentifier(text string) bool {
	lower := utils.TrimLower(text)
	if lower == "" {
		return false
	}
	if strings.Contains(lower, "react:iter:") ||
		strings.Contains(lower, "step_description") ||
		strings.Contains(lower, "payload") ||
		strings.Contains(lower, "nodeid") {
		return true
	}
	for _, token := range strings.FieldsFunc(lower, func(r rune) bool {
		return !(r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || r == '_' || r == '-' || r == ':')
	}) {
		if isOpaqueToolCallID(token) {
			return true
		}
	}
	return false
}

func isOpaqueToolCallID(token string) bool {
	token = utils.TrimLower(token)
	if token == "" {
		return false
	}
	if strings.HasPrefix(token, "call_") && len(token) >= len("call_")+6 {
		return true
	}
	if strings.HasPrefix(token, "call-") && len(token) >= len("call-")+6 {
		return true
	}
	return false
}
