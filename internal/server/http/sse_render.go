package http

import (
	"bytes"
	"encoding/json"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/domain"
	agent "alex/internal/agent/ports/agent"
)

var sseJSONBufferPool = sync.Pool{
	New: func() any {
		return new(bytes.Buffer)
	},
}

// serializeEvent converts domain event to JSON
func (h *SSEHandler) serializeEvent(event agent.AgentEvent, sentAttachments *stringLRU, finalAnswerCache *stringLRU) (string, error) {
	data, err := h.buildEventData(event, sentAttachments, finalAnswerCache, true)
	if err != nil {
		return "", err
	}

	return marshalSSEPayload(data)
}

func marshalSSEPayload(data map[string]interface{}) (string, error) {
	buf := sseJSONBufferPool.Get().(*bytes.Buffer)
	buf.Reset()
	defer sseJSONBufferPool.Put(buf)

	encoder := json.NewEncoder(buf)
	encoder.SetEscapeHTML(false)
	if err := encoder.Encode(data); err != nil {
		return "", err
	}

	payload := strings.TrimSuffix(buf.String(), "\n")
	return payload, nil
}

// buildEventData is the single source of truth for the SSE event envelope the
// backend emits. It assumes all events have already been translated into
// workflow.* envelopes.
func (h *SSEHandler) buildEventData(event agent.AgentEvent, sentAttachments *stringLRU, finalAnswerCache *stringLRU, streamDeltas bool) (map[string]interface{}, error) {
	data := map[string]interface{}{
		"event_id":        event.GetEventID(),
		"event_type":      event.EventType(),
		"seq":             event.GetSeq(),
		"timestamp":       event.Timestamp().Format(time.RFC3339Nano),
		"agent_level":     event.GetAgentLevel(),
		"session_id":      event.GetSessionID(),
		"run_id":          event.GetRunID(),
		"parent_run_id":   event.GetParentRunID(),
		"correlation_id":  event.GetCorrelationID(),
		"causation_id":    event.GetCausationID(),
	}
	if withLogID, ok := event.(interface{ GetLogID() string }); ok {
		if logID := strings.TrimSpace(withLogID.GetLogID()); logID != "" {
			data["log_id"] = logID
		}
	}

	// Subtask envelopes are flattened into the base envelope while retaining
	// metadata.
	if subtaskEvent, ok := event.(agent.SubtaskWrapper); ok {
		base, err := h.buildEventData(subtaskEvent.WrappedEvent(), sentAttachments, finalAnswerCache, streamDeltas)
		if err != nil {
			return nil, err
		}
		for k, v := range base {
			data[k] = v
		}
		data["timestamp"] = subtaskEvent.Timestamp().Format(time.RFC3339Nano)
		data["agent_level"] = subtaskEvent.GetAgentLevel()
		data["session_id"] = subtaskEvent.GetSessionID()
		data["run_id"] = subtaskEvent.GetRunID()
		if parentRunID := subtaskEvent.GetParentRunID(); parentRunID != "" {
			data["parent_run_id"] = parentRunID
		}
		data["correlation_id"] = subtaskEvent.GetCorrelationID()
		data["causation_id"] = subtaskEvent.GetCausationID()
		data["is_subtask"] = true
		meta := subtaskEvent.SubtaskDetails()
		if meta.Index > 0 {
			data["subtask_index"] = meta.Index
		}
		if meta.Total > 0 {
			data["total_subtasks"] = meta.Total
		}
		if meta.Preview != "" {
			data["subtask_preview"] = meta.Preview
		}
		if meta.MaxParallel > 0 {
			data["max_parallel"] = meta.MaxParallel
		}
		return data, nil
	}

	// Allow direct user input events if they have not been wrapped yet.
	if input, ok := event.(*domain.WorkflowInputReceivedEvent); ok {
		if sanitized := sanitizeAttachmentsForStream(input.Attachments, sentAttachments, h.dataCache, h.attachmentStore, false); len(sanitized) > 0 {
			data["attachments"] = sanitized
		}
		data["task"] = input.Task
		return data, nil
	}

	envelope, ok := event.(*domain.WorkflowEventEnvelope)
	if !ok {
		return data, nil
	}

	data["version"] = envelope.Version
	if envelope.WorkflowID != "" {
		data["workflow_id"] = envelope.WorkflowID
	}
	if envelope.RunID != "" {
		data["run_id"] = envelope.RunID
	}
	if envelope.NodeID != "" {
		data["node_id"] = envelope.NodeID
	}
	if envelope.NodeKind != "" {
		data["node_kind"] = envelope.NodeKind
	}
	if envelope.IsSubtask {
		data["is_subtask"] = true
	}
	if envelope.SubtaskIndex > 0 {
		data["subtask_index"] = envelope.SubtaskIndex
	}
	if envelope.TotalSubtasks > 0 {
		data["total_subtasks"] = envelope.TotalSubtasks
	}
	if envelope.SubtaskPreview != "" {
		data["subtask_preview"] = envelope.SubtaskPreview
	}
	if envelope.MaxParallel > 0 {
		data["max_parallel"] = envelope.MaxParallel
	}

	payload := sanitizeWorkflowEnvelopePayload(envelope, sentAttachments, h.dataCache, h.attachmentStore)

	// Force-include all attachments on the terminal workflow.result.final event
	// so the frontend always receives the full attachment set in the completion
	// card, even when individual attachments were already sent via earlier
	// tool-completed events and deduplicated by the LRU.
	if envelope.Event == "workflow.result.final" {
		if finished, _ := envelope.Payload["stream_finished"].(bool); finished {
			if rawAtts, ok := envelope.Payload["attachments"]; ok {
				if typedAtts := coerceAttachmentMap(rawAtts); len(typedAtts) > 0 {
					forced := sanitizeAttachmentsForStream(typedAtts, sentAttachments, h.dataCache, h.attachmentStore, true)
					if len(forced) > 0 {
						if payload == nil {
							payload = make(map[string]any)
						}
						payload["attachments"] = forced
					}
				}
			}
		}
	}

	if streamDeltas && envelope.Event == "workflow.result.final" {
		if val, ok := payload["final_answer"].(string); ok {
			key := envelope.GetRunID()
			delta := val
			if prev, ok := finalAnswerCache.Get(key); ok && strings.HasPrefix(val, prev) {
				delta = strings.TrimPrefix(val, prev)
			}
			if key != "" {
				if isStreaming, ok := payload["is_streaming"].(bool); ok && isStreaming {
					finalAnswerCache.Set(key, val)
				}
				if finished, ok := payload["stream_finished"].(bool); ok && finished {
					finalAnswerCache.Delete(key)
				}
			}
			payload["final_answer"] = delta
		}
	}
	if len(payload) > 0 {
		data["payload"] = payload
	}

	return data, nil
}
