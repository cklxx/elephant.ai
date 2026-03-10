package react

import (
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/utils"
)

// normalizeAttachmentMutations extracts add/update/remove operations from tool
// metadata, tolerating partial inputs and legacy shapes.
func normalizeAttachmentMutations(metadata map[string]any) *attachmentMutations {
	if len(metadata) == 0 {
		return nil
	}

	raw, ok := metadata["attachment_mutations"]
	if !ok {
		raw = metadata["attachments_mutations"]
	}
	rawMap, ok := raw.(map[string]any)
	if !ok || len(rawMap) == 0 {
		return nil
	}

	replace := parseAttachmentMap(rawMap["replace"], rawMap["snapshot"], rawMap["catalog"])
	add := parseAttachmentMap(rawMap["add"], rawMap["create"])
	update := parseAttachmentMap(rawMap["update"], rawMap["upsert"])
	remove := parseAttachmentRemovals(rawMap["remove"], rawMap["delete"])

	if replace == nil && add == nil && update == nil && len(remove) == 0 {
		return nil
	}

	return &attachmentMutations{replace: replace, add: add, update: update, remove: remove}
}

// parseAttachmentRemovals coalesces removal requests that may be expressed as
// strings, arrays, or arbitrary values into a normalized name list.
func parseAttachmentRemovals(values ...any) []string {
	seen := make(map[string]struct{})
	var result []string

	for _, value := range values {
		switch typed := value.(type) {
		case []string:
			for _, item := range typed {
				if trimmed := strings.TrimSpace(item); trimmed != "" {
					if _, ok := seen[trimmed]; !ok {
						seen[trimmed] = struct{}{}
						result = append(result, trimmed)
					}
				}
			}
		case []any:
			for _, item := range typed {
				str := strings.TrimSpace(fmt.Sprint(item))
				if str == "" {
					continue
				}
				if _, ok := seen[str]; !ok {
					seen[str] = struct{}{}
					result = append(result, str)
				}
			}
		}
	}

	return result
}

// parseAttachmentMap converts heterogeneous mutation payloads into a typed
// attachment map, ignoring malformed entries.
func parseAttachmentMap(values ...any) map[string]ports.Attachment {
	for _, value := range values {
		switch typed := value.(type) {
		case map[string]ports.Attachment:
			return normalizeAttachmentMap(typed)
		case map[string]any:
			converted := make(map[string]ports.Attachment, len(typed))
			for key, item := range typed {
				if att, ok := parseAttachment(item); ok {
					converted[key] = att
				}
			}
			return normalizeAttachmentMap(converted)
		}
	}
	return nil
}

func parseAttachment(value any) (ports.Attachment, bool) {
	switch typed := value.(type) {
	case ports.Attachment:
		return typed, true
	case map[string]any:
		var att ports.Attachment
		if err := mapToAttachment(typed, &att); err == nil {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func mapToAttachment(input map[string]any, out *ports.Attachment) error {
	if input == nil {
		return fmt.Errorf("attachment map is nil")
	}
	encoded, err := json.Marshal(input)
	if err != nil {
		return err
	}
	return json.Unmarshal(encoded, out)
}

// normalizeAttachmentMap cleans attachment entries to avoid nil maps and keeps
// only non-empty names.
func normalizeAttachmentMap(input map[string]ports.Attachment) map[string]ports.Attachment {
	return ports.NormalizeAttachmentMapFillBlankName(input)
}

// buildAttachmentCatalogContent renders a human-readable index of current
// attachments so the model can reference placeholders reliably.
func buildAttachmentCatalogContent(state *TaskState) string {
	if state == nil || len(state.Attachments) == 0 {
		return ""
	}
	keys := sortedAttachmentKeys(state.Attachments)
	if len(keys) == 0 {
		return ""
	}

	var builder strings.Builder
	builder.WriteString("Attachment catalog (informational — the system displays these to the user automatically).\n")
	builder.WriteString("Do not reference these files in your response text. They will be attached to the final result automatically.\n\n")

	for i, key := range keys {
		att := state.Attachments[key]
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("%d. %s", i+1, name))
		var meta []string
		description := strings.TrimSpace(att.Description)
		if description != "" {
			meta = append(meta, description)
		}
		if source := strings.TrimSpace(att.Source); source != "" {
			meta = append(meta, "source: "+source)
		}
		if len(meta) > 0 {
			builder.WriteString(" — " + strings.Join(meta, " | "))
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\nThese attachments are available for follow-up tool calls if needed.")

	return strings.TrimSpace(builder.String())
}

func attachmentReferenceValue(att ports.Attachment) string {
	return ports.AttachmentReferenceValue(att)
}

func isA2UIAttachment(att ports.Attachment) bool {
	media := utils.TrimLower(att.MediaType)
	format := utils.TrimLower(att.Format)
	profile := utils.TrimLower(att.PreviewProfile)
	return strings.Contains(media, "a2ui") || format == "a2ui" || strings.Contains(profile, "a2ui")
}

func collectA2UIAttachments(state *TaskState) map[string]ports.Attachment {
	if state == nil || len(state.Attachments) == 0 {
		return nil
	}
	collected := make(map[string]ports.Attachment)
	for key, att := range state.Attachments {
		if !isA2UIAttachment(att) {
			continue
		}
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		cloned := att
		if cloned.Name == "" {
			cloned.Name = placeholder
		}
		collected[placeholder] = cloned
	}
	if len(collected) == 0 {
		return nil
	}
	return collected
}

// collectGeneratedAttachments returns attachments produced during the current
// iteration so alias resolution can prioritize fresh outputs.
func collectGeneratedAttachments(state *TaskState, iteration int) map[string]ports.Attachment {
	if state == nil || len(state.Attachments) == 0 {
		return nil
	}
	generated := make(map[string]ports.Attachment)
	for key, att := range state.Attachments {
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if state.AttachmentIterations != nil {
			if iter, ok := state.AttachmentIterations[placeholder]; ok && iter > iteration {
				continue
			}
		}
		if strings.EqualFold(strings.TrimSpace(att.Source), "user_upload") {
			continue
		}
		cloned := att
		if cloned.Name == "" {
			cloned.Name = placeholder
		}
		generated[placeholder] = cloned
	}
	if len(generated) == 0 {
		return nil
	}
	return generated
}

// collectAllToolGeneratedAttachments returns every non-user-upload attachment
// from the task state, regardless of whether the final answer references them.
// This is used by finalize() so the final result carries the complete set of
// tool-generated assets for downstream rendering.
func collectAllToolGeneratedAttachments(state *TaskState) map[string]ports.Attachment {
	if state == nil || len(state.Attachments) == 0 {
		return nil
	}
	collected := make(map[string]ports.Attachment)
	for key, att := range state.Attachments {
		if strings.EqualFold(strings.TrimSpace(att.Source), "user_upload") {
			continue
		}
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		cloned := att
		if cloned.Name == "" {
			cloned.Name = placeholder
		}
		collected[placeholder] = cloned
	}
	if len(collected) == 0 {
		return nil
	}
	return collected
}

// stripAttachmentPlaceholders removes all [placeholder] markers from the text.
// Attachments are delivered as separate messages by downstream channels.
func stripAttachmentPlaceholders(answer string) string {
	normalized := strings.TrimSpace(answer)
	replaced := contentPlaceholderPattern.ReplaceAllString(normalized, "")
	return strings.TrimSpace(replaced)
}
