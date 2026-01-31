package react

import (
	"context"
	"slices"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	id "alex/internal/utils/id"
)

func (e *ReactEngine) ensureSystemPromptMessage(state *TaskState) {
	if state == nil {
		return
	}

	prompt := strings.TrimSpace(state.SystemPrompt)
	if prompt == "" {
		return
	}

	for idx := range state.Messages {
		role := strings.ToLower(strings.TrimSpace(state.Messages[idx].Role))
		if role != "system" {
			continue
		}
		source := ports.MessageSource(strings.TrimSpace(string(state.Messages[idx].Source)))
		if source != "" && source != ports.MessageSourceSystemPrompt {

			continue
		}

		if strings.TrimSpace(state.Messages[idx].Content) != prompt {
			state.Messages[idx].Content = state.SystemPrompt
			state.Messages[idx].Source = ports.MessageSourceSystemPrompt
			e.logger.Debug("Updated existing system prompt in message history")
		} else if source == "" {
			state.Messages[idx].Source = ports.MessageSourceSystemPrompt
		}

		existing := state.Messages[idx]
		if idx > 0 {
			// Move system message to front using slices.Delete + slices.Insert for efficiency
			state.Messages = slices.Delete(state.Messages, idx, idx+1)
			state.Messages = slices.Insert(state.Messages, 0, existing)
		} else {
			state.Messages[0] = existing
		}
		return
	}

	systemMessage := Message{
		Role:    "system",
		Content: state.SystemPrompt,
		Source:  ports.MessageSourceSystemPrompt,
	}

	// Use slices.Insert for efficient prepend
	state.Messages = slices.Insert(state.Messages, 0, systemMessage)
	e.logger.Debug("Inserted system prompt into message history")
}

func (e *ReactEngine) applyToolAttachmentMutations(
	ctx context.Context,
	state *TaskState,
	call ToolCall,
	attachments map[string]ports.Attachment,
	metadata map[string]any,
	attachmentsMu *sync.RWMutex,
) map[string]ports.Attachment {
	normalized := normalizeAttachmentMap(attachments)
	mutations := normalizeAttachmentMutations(metadata)

	var existing map[string]ports.Attachment
	if state != nil {
		if attachmentsMu != nil {
			attachmentsMu.RLock()
			existing = normalizeAttachmentMap(state.Attachments)
			attachmentsMu.RUnlock()
		} else {
			existing = normalizeAttachmentMap(state.Attachments)
		}
	}

	merged := mergeAttachmentMutations(normalized, mutations, existing)
	if attachmentsMu != nil {
		attachmentsMu.Lock()
		defer attachmentsMu.Unlock()
	}
	applyAttachmentMutationsToState(ctx, state, merged, mutations, call.Name, e.attachmentPersister)

	// Return persisted versions from state so callers (result.Attachments,
	// buildToolMessages) carry URI references instead of inline base64.
	if state != nil && len(state.Attachments) > 0 && len(merged) > 0 {
		persisted := make(map[string]ports.Attachment, len(merged))
		for key := range merged {
			if att, ok := state.Attachments[key]; ok {
				persisted[key] = att
			}
		}
		return persisted
	}
	return merged
}

// extractImportantNotes normalizes and enriches important notes from tool
// metadata. This is a pure function that does not touch shared state, so it
// can safely run outside any lock.
func (e *ReactEngine) extractImportantNotes(call ToolCall, metadata map[string]any) []ports.ImportantNote {
	if len(metadata) == 0 {
		return nil
	}
	raw, ok := metadata["important_notes"]
	if !ok {
		return nil
	}
	notes := normalizeImportantNotes(raw, e.clock)
	if len(notes) == 0 {
		return nil
	}
	enriched := notes[:0]
	for _, note := range notes {
		if strings.TrimSpace(note.Content) == "" {
			continue
		}
		if note.ID == "" {
			note.ID = id.NewKSUID()
		}
		if note.CreatedAt.IsZero() && e.clock != nil {
			note.CreatedAt = e.clock.Now()
		}
		if note.Source == "" {
			note.Source = call.Name
		}
		enriched = append(enriched, note)
	}
	return enriched
}

// mergeImportantNotes writes pre-extracted notes into the shared task state.
// Caller must hold stateMu.
func (e *ReactEngine) mergeImportantNotes(state *TaskState, notes []ports.ImportantNote) {
	if state == nil || len(notes) == 0 {
		return
	}
	if state.Important == nil {
		state.Important = make(map[string]ports.ImportantNote)
	}
	for _, note := range notes {
		state.Important[note.ID] = note
	}
}

func normalizeImportantNotes(raw any, clock agent.Clock) []ports.ImportantNote {
	switch v := raw.(type) {
	case []ports.ImportantNote:
		notes := make([]ports.ImportantNote, len(v))
		copy(notes, v)
		return notes
	case []any:
		var notes []ports.ImportantNote
		for _, item := range v {
			switch note := item.(type) {
			case ports.ImportantNote:
				notes = append(notes, note)
			case map[string]any:
				if parsed := parseImportantNoteMap(note, clock); parsed.Content != "" {
					notes = append(notes, parsed)
				}
			}
		}
		return notes
	case map[string]any:
		if parsed := parseImportantNoteMap(v, clock); parsed.Content != "" {
			return []ports.ImportantNote{parsed}
		}
	}
	return nil
}

func parseImportantNoteMap(raw map[string]any, clock agent.Clock) ports.ImportantNote {
	note := ports.ImportantNote{}
	if idVal, ok := raw["id"].(string); ok {
		note.ID = strings.TrimSpace(idVal)
	}
	if content, ok := raw["content"].(string); ok {
		note.Content = strings.TrimSpace(content)
	}
	if source, ok := raw["source"].(string); ok {
		note.Source = strings.TrimSpace(source)
	}
	if tagsRaw, ok := raw["tags"].([]any); ok {
		for _, tag := range tagsRaw {
			if text, ok := tag.(string); ok {
				if trimmed := strings.TrimSpace(text); trimmed != "" {
					note.Tags = append(note.Tags, trimmed)
				}
			}
		}
	}
	switch created := raw["created_at"].(type) {
	case time.Time:
		note.CreatedAt = created
	case string:
		if parsed, err := time.Parse(time.RFC3339, created); err == nil {
			note.CreatedAt = parsed
		}
	}
	if note.CreatedAt.IsZero() && clock != nil {
		note.CreatedAt = clock.Now()
	}
	return note
}

func (e *ReactEngine) updateAttachmentCatalogMessage(state *TaskState) {
	if state == nil {
		return
	}
	content := buildAttachmentCatalogContent(state)
	if strings.TrimSpace(content) == "" {
		content = "Attachment catalog (for model reference only).\nNo attachments are currently available."
	}
	note := Message{
		Role:    "assistant",
		Content: content,
		Source:  ports.MessageSourceAssistantReply,
		Metadata: map[string]any{
			attachmentCatalogMetadataKey: true,
		},
	}
	state.Messages = append(state.Messages, note)
}

func (e *ReactEngine) expandPlaceholders(args map[string]any, state *TaskState) map[string]any {
	if len(args) == 0 {
		return args
	}
	expanded := make(map[string]any, len(args))
	for key, value := range args {
		expanded[key] = e.expandPlaceholderValue(value, state)
	}
	return expanded
}

func (e *ReactEngine) expandToolCallArguments(toolName string, args map[string]any, state *TaskState) map[string]any {
	if len(args) == 0 {
		return args
	}

	switch strings.TrimSpace(toolName) {
	case "vision_analyze":
		preserve := map[string]bool{"images": true, "prompt": true}
		return e.expandToolArgsPreservingKeys(args, state, preserve)
	case "artifacts_list":
		skipKeys := map[string]bool{"name": true}
		return e.expandToolArgsSkippingKeys(args, state, skipKeys)
	case "artifacts_write":
		skipKeys := map[string]bool{"name": true}
		return e.expandToolArgsSkippingKeys(args, state, skipKeys)
	case "artifacts_delete":
		skipKeys := map[string]bool{"name": true, "names": true}
		return e.expandToolArgsSkippingKeys(args, state, skipKeys)
	case "html_edit":
		skipKeys := map[string]bool{"name": true, "output_name": true}
		return e.expandToolArgsSkippingKeys(args, state, skipKeys)
	default:
		return e.expandPlaceholders(args, state)
	}
}

func (e *ReactEngine) expandToolArgsPreservingKeys(args map[string]any, state *TaskState, preserveKeys map[string]bool) map[string]any {
	expanded := make(map[string]any, len(args))
	for key, value := range args {
		if preserveKeys[key] {
			expanded[key] = value
			continue
		}
		expanded[key] = e.expandPlaceholderValue(value, state)
	}
	return expanded
}

func (e *ReactEngine) expandToolArgsSkippingKeys(args map[string]any, state *TaskState, skipKeys map[string]bool) map[string]any {
	expanded := make(map[string]any, len(args))
	for key, value := range args {
		if skipKeys[key] {
			expanded[key] = unwrapAttachmentPlaceholderValue(value)
			continue
		}
		expanded[key] = e.expandPlaceholderValue(value, state)
	}
	return expanded
}

func unwrapAttachmentPlaceholderValue(value any) any {
	switch v := value.(type) {
	case string:
		if name, ok := extractPlaceholderName(v); ok {
			return name
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i := range v {
			out[i] = unwrapAttachmentPlaceholderValue(v[i])
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i := range v {
			out[i] = v[i]
			if name, ok := extractPlaceholderName(v[i]); ok {
				out[i] = name
			}
		}
		return out
	default:
		return value
	}
}

func (e *ReactEngine) expandPlaceholderValue(value any, state *TaskState) any {
	switch v := value.(type) {
	case string:
		if replacement, ok := e.resolveStringAttachmentValue(v, state); ok {
			return replacement
		}
		return v
	case []any:
		out := make([]any, len(v))
		for i, item := range v {
			out[i] = e.expandPlaceholderValue(item, state)
		}
		return out
	case []string:
		out := make([]string, len(v))
		for i, item := range v {
			if replacement, ok := e.resolveStringAttachmentValue(item, state); ok {
				out[i] = replacement
				continue
			}
			out[i] = item
		}
		return out
	case map[string]any:
		nested := make(map[string]any, len(v))
		for key, item := range v {
			nested[key] = e.expandPlaceholderValue(item, state)
		}
		return nested
	case map[string]string:
		nested := make(map[string]string, len(v))
		for key, item := range v {
			if replacement, ok := e.resolveStringAttachmentValue(item, state); ok {
				nested[key] = replacement
				continue
			}
			nested[key] = item
		}
		return nested
	default:
		return value
	}
}

func (e *ReactEngine) lookupAttachmentByName(name string, state *TaskState) (ports.Attachment, string, bool) {
	att, canonical, kind, ok := lookupAttachmentByNameInternal(name, state)
	if !ok {
		return ports.Attachment{}, "", false
	}

	switch kind {
	case attachmentMatchSeedreamAlias:
		e.logger.Info("Resolved Seedream placeholder alias [%s] -> [%s]", name, canonical)
	case attachmentMatchGeneric:
		e.logger.Info("Mapping generic image placeholder [%s] to attachment [%s]", name, canonical)
	}

	return att, canonical, true
}

func (e *ReactEngine) resolveStringAttachmentValue(value string, state *TaskState) (string, bool) {
	alias, att, canonical, ok := matchAttachmentReference(value, state)
	if !ok {
		return "", false
	}
	replacement := attachmentReferenceValue(att)
	if replacement == "" {
		return "", false
	}
	if canonical != "" && canonical != alias {
		e.logger.Info("Resolved placeholder [%s] as alias for attachment [%s]", alias, canonical)
	}
	return replacement, true
}
