package coordinator

import (
	"strings"

	domain "alex/internal/domain/agent"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/types"
)

func (t *workflowEventTranslator) translateTool(evt agent.AgentEvent, eventType, callID string, payload map[string]any) []*domain.WorkflowEventEnvelope {
	return t.toolEnvelope(evt, eventType, callID, payload)
}

func (t *workflowEventTranslator) translateToolComplete(evt agent.AgentEvent, d *domain.EventData) []*domain.WorkflowEventEnvelope {
	payload := map[string]any{
		"tool_name":   d.ToolName,
		"result":      d.Result,
		"duration":    d.Duration.Milliseconds(),
		"metadata":    d.Metadata,
		"attachments": d.Attachments,
	}
	if d.Error != nil {
		payload["error"] = d.Error.Error()
	}

	envelopes := t.toolEnvelope(evt, types.EventToolCompleted, d.CallID, payload)
	if manifestPayload := buildArtifactManifestPayload(d); manifestPayload != nil {
		envelopes = append(envelopes, t.singleEnvelope(evt, types.EventArtifactManifest, "artifact", "artifact-manifest", manifestPayload)...)
	}
	return envelopes
}

func buildArtifactManifestPayload(d *domain.EventData) map[string]any {
	if d == nil {
		return nil
	}
	toolName := strings.ToLower(strings.TrimSpace(d.ToolName))
	if toolName == "acp_executor" {
		return nil
	}
	attachments := d.Attachments
	if d.Metadata != nil {
		if manifest, ok := d.Metadata["artifact_manifest"]; ok {
			payload := map[string]any{
				"manifest":    manifest,
				"source_tool": d.ToolName,
			}
			if len(attachments) > 0 {
				payload["attachments"] = attachments
			}
			return payload
		}
	}
	if toolName == "artifact_manifest" {
		payload := map[string]any{
			"result":      d.Result,
			"source_tool": d.ToolName,
		}
		if len(attachments) > 0 {
			payload["attachments"] = attachments
		}
		return payload
	}
	if len(attachments) == 0 {
		return nil
	}
	for _, att := range attachments {
		format := strings.ToLower(strings.TrimSpace(att.Format))
		if format == "manifest" || strings.Contains(strings.ToLower(att.Name), "manifest") {
			return map[string]any{
				"attachments": attachments,
				"source_tool": d.ToolName,
			}
		}
	}
	return nil
}
