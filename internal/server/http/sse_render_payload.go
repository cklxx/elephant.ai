package http

import (
	"fmt"
	"time"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/workflow"
)

func sanitizeEnvelopePayload(payload map[string]any, sent *stringLRU, cache *DataCache, store *AttachmentStore) map[string]any {
	if len(payload) == 0 {
		return nil
	}
	sanitized := make(map[string]any, len(payload))
	for key, value := range payload {
		if key == "attachments" {
			sanitized[key] = sanitizeUntypedAttachments(value, sent, cache, store)
			continue
		}
		if key == "result" && cache != nil {
			clean := sanitizeStepResultValue(value)
			sanitized[key] = sanitizeEnvelopeValue(clean, sent, cache, store)
			continue
		}
		sanitized[key] = sanitizeEnvelopeValue(value, sent, cache, store)
	}
	return sanitized
}

func sanitizeEnvelopeValue(value any, sent *stringLRU, cache *DataCache, store *AttachmentStore) any {
	switch v := value.(type) {
	case nil:
		return nil
	case map[string]ports.Attachment:
		return sanitizeAttachmentsForStream(v, sent, cache, store, false)
	case ports.Attachment:
		if sanitized := sanitizeAttachmentsForStream(map[string]ports.Attachment{"attachment": v}, sent, cache, store, false); len(sanitized) > 0 {
			return sanitized["attachment"]
		}
		return nil
	case workflow.NodeSnapshot:
		return sanitizeWorkflowNode(v)
	case *workflow.NodeSnapshot:
		if v == nil {
			return nil
		}
		return sanitizeWorkflowNode(*v)
	case *workflow.WorkflowSnapshot:
		return sanitizeWorkflowSnapshot(v)
	case workflow.WorkflowSnapshot:
		snap := v
		return sanitizeWorkflowSnapshot(&snap)
	case time.Time:
		if v.IsZero() {
			return nil
		}
		return v.Format(time.RFC3339Nano)
	case map[string]any:
		sanitized := make(map[string]any, len(v))
		for key, val := range v {
			if key == "attachments" {
				sanitized[key] = sanitizeUntypedAttachments(val, sent, cache, store)
				continue
			}
			if key == "nodes" {
				continue
			}
			if key == "messages" || key == "attachment_iterations" {
				continue
			}
			sanitized[key] = sanitizeEnvelopeValue(val, sent, cache, store)
		}
		return sanitized
	case []any:
		out := make([]any, len(v))
		for i, entry := range v {
			out[i] = sanitizeEnvelopeValue(entry, sent, cache, store)
		}
		return out
	default:
		return sanitizeValue(cache, v)
	}
}

func sanitizeWorkflowEnvelopePayload(env *domain.WorkflowEventEnvelope, sent *stringLRU, cache *DataCache, store *AttachmentStore) map[string]any {
	if env == nil {
		return nil
	}

	payload := env.Payload
	if env.Event == "workflow.node.completed" && env.NodeKind == "step" {
		payload = scrubStepPayload(payload)
	}

	return sanitizeEnvelopePayload(payload, sent, cache, store)
}

func sanitizeStepResultValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		clean := make(map[string]any, len(v))
		for key, val := range v {
			if key == "messages" || key == "attachment_iterations" {
				continue
			}
			clean[key] = val
		}
		return clean
	case []any:
		return nil
	default:
		return value
	}
}

func scrubStepPayload(payload map[string]any) map[string]any {
	if len(payload) == 0 {
		return payload
	}

	scrubbed := make(map[string]any, len(payload)+1)
	for key, val := range payload {
		scrubbed[key] = val
	}

	if res, ok := scrubbed["result"]; ok {
		clean := sanitizeStepResultValue(res)
		scrubbed["result"] = clean
		if summary := summarizeStepResult(clean); summary != "" {
			scrubbed["step_result"] = summary
		}
	} else if sr, ok := scrubbed["step_result"]; ok {
		if summary := summarizeStepResult(sr); summary != "" {
			scrubbed["step_result"] = summary
		}
	}

	return scrubbed
}

func summarizeStepResult(value any) string {
	switch v := value.(type) {
	case nil:
		return ""
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case map[string]any:
		if errMsg, ok := v["error"].(string); ok && errMsg != "" {
			return errMsg
		}
		for _, key := range []string{"summary", "content", "output", "text"} {
			if s, ok := v[key].(string); ok && s != "" {
				return s
			}
		}

		clean := make(map[string]any, len(v))
		for key, val := range v {
			if key == "messages" || key == "attachments" || key == "attachment_iterations" {
				continue
			}
			clean[key] = val
		}
		if len(clean) == 0 {
			return ""
		}
		if desc, ok := clean["description"].(string); ok && desc != "" {
			return desc
		}
		return fmt.Sprint(clean)
	default:
		return fmt.Sprint(v)
	}
}
