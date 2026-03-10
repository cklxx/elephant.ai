package lark

import (
	"fmt"
	"strings"

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

func signalFromEnvelope(e *domain.WorkflowEventEnvelope) (slowProgressSignal, bool) {
	if e == nil {
		return slowProgressSignal{}, false
	}
	toolName := strings.TrimSpace(envelopeToolName(e))
	switch strings.TrimSpace(e.Event) {
	case types.EventNodeStarted:
		step := resolveSlowProgressStepLabel(asString(e.Payload["step_description"]), e.NodeID)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始步骤：" + step}, true
	case types.EventNodeCompleted:
		step := resolveSlowProgressStepLabel(asString(e.Payload["step_description"]), e.NodeID)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成步骤：" + step}, true
	case types.EventToolStarted:
		if toolName == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始工具：" + toolName}, true
	case types.EventToolCompleted:
		if toolName == "" {
			toolName = "tool"
		}
		errText := strings.TrimSpace(asString(e.Payload["error"]))
		if errText != "" {
			return slowProgressSignal{
				at:   e.Timestamp(),
				text: "工具失败：" + toolName + "（" + truncateForLark(errText, 80) + ")",
			}, true
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成工具：" + toolName}, true
	case types.EventNodeOutputSummary:
		content := sanitizeSlowProgressContent(asString(e.Payload["content"]))
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "阶段输出：" + content}, true
	default:
		return slowProgressSignal{}, false
	}
}

func signalFromUnified(e *domain.Event) (slowProgressSignal, bool) {
	if e == nil {
		return slowProgressSignal{}, false
	}
	switch strings.TrimSpace(e.Kind) {
	case types.EventNodeStarted:
		step := strings.TrimSpace(e.Data.StepDescription)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始步骤：" + truncateForLark(step, 120)}, true
	case types.EventNodeCompleted:
		step := strings.TrimSpace(e.Data.StepDescription)
		if step == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成步骤：" + truncateForLark(step, 120)}, true
	case types.EventToolStarted:
		toolName := strings.TrimSpace(e.Data.ToolName)
		if toolName == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "开始工具：" + toolName}, true
	case types.EventToolCompleted:
		toolName := strings.TrimSpace(e.Data.ToolName)
		if toolName == "" {
			toolName = "tool"
		}
		if e.Data.Error != nil {
			return slowProgressSignal{
				at:   e.Timestamp(),
				text: "工具失败：" + toolName + "（" + truncateForLark(e.Data.Error.Error(), 80) + ")",
			}, true
		}
		return slowProgressSignal{at: e.Timestamp(), text: "完成工具：" + toolName}, true
	case types.EventNodeOutputSummary:
		content := sanitizeSlowProgressContent(e.Data.Content)
		if content == "" {
			return slowProgressSignal{}, false
		}
		return slowProgressSignal{at: e.Timestamp(), text: "阶段输出：" + content}, true
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
