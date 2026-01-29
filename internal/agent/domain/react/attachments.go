package react

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"

	"alex/internal/agent/ports"
)

// ensureToolAttachmentReferences injects attachment placeholders into the
// content when a tool emitted attachments but forgot to reference them.
func ensureToolAttachmentReferences(content string, attachments map[string]ports.Attachment) string {
	if len(attachments) == 0 {
		return strings.TrimSpace(content)
	}

	normalized := strings.TrimSpace(content)
	mentioned := make(map[string]bool, len(attachments))

	keys := sortedAttachmentKeys(attachments)
	for _, name := range keys {
		placeholder := fmt.Sprintf("[%s]", name)
		if strings.Contains(normalized, placeholder) {
			mentioned[name] = true
		}
	}

	var builder strings.Builder
	if normalized != "" {
		builder.WriteString(normalized)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Attachments available for follow-up steps:\n")
	for _, name := range keys {
		fmt.Fprintf(&builder, "- [%s]%s\n", name, boolToStar(mentioned[name]))
	}

	return strings.TrimSpace(builder.String())
}

// snapshotAttachments clones the attachment store and returns per-attachment
// iteration indices for later reconciliation.
func snapshotAttachments(state *TaskState) (map[string]ports.Attachment, map[string]int) {
	if state == nil {
		return nil, nil
	}
	var attachments map[string]ports.Attachment
	if len(state.Attachments) > 0 {
		attachments = make(map[string]ports.Attachment, len(state.Attachments))
		for key, att := range state.Attachments {
			attachments[key] = att
		}
	}
	var iterations map[string]int
	if len(state.AttachmentIterations) > 0 {
		iterations = make(map[string]int, len(state.AttachmentIterations))
		for key, iter := range state.AttachmentIterations {
			iterations[key] = iter
		}
	}
	return attachments, iterations
}

func boolToStar(b bool) string {
	if b {
		return " (referenced)"
	}
	return ""
}

// normalizeToolAttachments standardizes tool attachments by filling defaults
// and trimming empty entries before storage.
func normalizeToolAttachments(attachments map[string]ports.Attachment) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	normalized := make(map[string]ports.Attachment, len(attachments))
	for _, key := range sortedAttachmentKeys(attachments) {
		att := attachments[key]
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if att.Name == "" {
			att.Name = placeholder
		}
		normalized[placeholder] = att
	}
	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

func sortedAttachmentKeys(attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	keys := make([]string, 0, len(attachments))
	seen := make(map[string]bool, len(attachments))
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || seen[name] {
			continue
		}
		seen[name] = true
		keys = append(keys, name)
	}
	sort.Strings(keys)
	return keys
}

// applyAttachmentMutationsToState merges incoming attachment mutations with
// the task state, ensuring delete/update/replace semantics stay consistent.
func applyAttachmentMutationsToState(
	state *TaskState,
	merged map[string]ports.Attachment,
	mutations *attachmentMutations,
	defaultSource string,
) {
	if state == nil {
		return
	}
	if defaultSource = strings.TrimSpace(defaultSource); defaultSource == "" {
		defaultSource = "tool"
	}

	ensureAttachmentStore(state)

	if mutations != nil && mutations.replace != nil {
		state.Attachments = make(map[string]ports.Attachment, len(mutations.replace))
		state.AttachmentIterations = make(map[string]int, len(mutations.replace))
		for key, att := range mutations.replace {
			att.Source = coalesceAttachmentSource(att.Source, defaultSource)
			state.Attachments[key] = att
			state.AttachmentIterations[key] = state.Iterations
		}
	}

	if mutations != nil && len(mutations.remove) > 0 {
		for _, key := range mutations.remove {
			trimmed := strings.TrimSpace(key)
			if trimmed == "" {
				continue
			}
			delete(state.Attachments, trimmed)
			if state.AttachmentIterations != nil {
				delete(state.AttachmentIterations, trimmed)
			}
		}
	}

	if len(merged) == 0 {
		return
	}

	for key, att := range merged {
		att.Source = coalesceAttachmentSource(att.Source, defaultSource)
		state.Attachments[key] = att
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		state.AttachmentIterations[key] = state.Iterations
	}
}

func coalesceAttachmentSource(source, fallback string) string {
	if strings.TrimSpace(source) != "" {
		return source
	}
	return fallback
}

func mergeAttachmentMutations(
	base map[string]ports.Attachment,
	mutations *attachmentMutations,
	existing map[string]ports.Attachment,
) map[string]ports.Attachment {
	merged := make(map[string]ports.Attachment)

	switch {
	case mutations != nil && mutations.replace != nil:
		for key, att := range mutations.replace {
			merged[key] = att
		}
	case len(base) > 0:
		for key, att := range base {
			merged[key] = att
		}
	case len(existing) > 0:
		for key, att := range existing {
			merged[key] = att
		}
	}

	if mutations != nil {
		if mutations.add != nil {
			for key, att := range mutations.add {
				merged[key] = att
			}
		}
		if mutations.update != nil {
			for key, att := range mutations.update {
				merged[key] = att
			}
		}
		for _, key := range mutations.remove {
			trimmed := strings.TrimSpace(key)
			if trimmed != "" {
				delete(merged, trimmed)
			}
		}
	}

	if len(merged) == 0 {
		return nil
	}
	return merged
}

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
	if len(input) == 0 {
		return nil
	}

	normalized := make(map[string]ports.Attachment, len(input))
	for key, att := range input {
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if strings.TrimSpace(att.Name) == "" {
			att.Name = placeholder
		}
		normalized[placeholder] = att
	}

	if len(normalized) == 0 {
		return nil
	}
	return normalized
}

// ensureAttachmentStore initializes the attachment map on the task state.
func ensureAttachmentStore(state *TaskState) {
	if state.Attachments == nil {
		state.Attachments = make(map[string]ports.Attachment)
	}
	if state.AttachmentIterations == nil {
		state.AttachmentIterations = make(map[string]int)
	}
}

// registerMessageAttachments pulls attachments from a message into the shared
// task store, returning true when the catalog changed.
func registerMessageAttachments(state *TaskState, msg Message) bool {
	if len(msg.Attachments) == 0 {
		return false
	}
	ensureAttachmentStore(state)
	changed := false
	for key, att := range msg.Attachments {
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		if att.Name == "" {
			att.Name = placeholder
		}
		if existing, ok := state.Attachments[placeholder]; !ok || !attachmentsEqual(existing, att) {
			state.Attachments[placeholder] = att
			changed = true
		}
		if state.AttachmentIterations == nil {
			state.AttachmentIterations = make(map[string]int)
		}
		state.AttachmentIterations[placeholder] = state.Iterations
	}
	return changed
}

func attachmentsEqual(a, b ports.Attachment) bool {
	if a.Name != b.Name ||
		a.MediaType != b.MediaType ||
		a.Data != b.Data ||
		a.URI != b.URI ||
		a.Source != b.Source ||
		a.Description != b.Description ||
		a.Kind != b.Kind ||
		a.Format != b.Format ||
		a.PreviewProfile != b.PreviewProfile {
		return false
	}
	return previewAssetsEqual(a.PreviewAssets, b.PreviewAssets)
}

func previewAssetsEqual(a, b []ports.AttachmentPreviewAsset) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
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
	builder.WriteString("Attachment catalog (for model reference only).\n")
	builder.WriteString("Reference assets by typing their placeholders exactly as shown (e.g., [diagram.png]).\n\n")

	for i, key := range keys {
		att := state.Attachments[key]
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		if placeholder == "" {
			continue
		}
		builder.WriteString(fmt.Sprintf("%d. [%s]", i+1, placeholder))
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

	builder.WriteString("\nUse the placeholders verbatim to work with these attachments in follow-up steps.")

	return strings.TrimSpace(builder.String())
}

func attachmentReferenceValue(att ports.Attachment) string {
	return ports.AttachmentReferenceValue(att)
}

func lookupAttachmentByNameInternal(name string, state *TaskState) (ports.Attachment, string, string, bool) {
	if state == nil {
		return ports.Attachment{}, "", "", false
	}

	if att, ok := state.Attachments[name]; ok {
		return att, name, attachmentMatchExact, true
	}

	for key, att := range state.Attachments {
		if strings.EqualFold(key, name) {
			return att, key, attachmentMatchCaseInsensitive, true
		}
	}

	if canonical, att, ok := matchSeedreamPlaceholderAlias(name, state); ok {
		return att, canonical, attachmentMatchSeedreamAlias, true
	}

	if canonical, att, ok := matchGenericImageAlias(name, state); ok {
		return att, canonical, attachmentMatchGeneric, true
	}

	return ports.Attachment{}, "", "", false
}

// matchSeedreamPlaceholderAlias resolves legacy seedream placeholders such as
// "placeholder_N" to the generated attachment they reference.
func matchSeedreamPlaceholderAlias(name string, state *TaskState) (string, ports.Attachment, bool) {
	if state == nil || len(state.Attachments) == 0 {
		return "", ports.Attachment{}, false
	}

	trimmed := strings.TrimSpace(name)
	dot := strings.LastIndex(trimmed, ".")
	if dot <= 0 {
		return "", ports.Attachment{}, false
	}

	ext := strings.ToLower(trimmed[dot:])
	base := trimmed[:dot]
	underscore := strings.LastIndex(base, "_")
	if underscore <= 0 {
		return "", ports.Attachment{}, false
	}

	indexPart := base[underscore+1:]
	if _, err := strconv.Atoi(indexPart); err != nil {
		return "", ports.Attachment{}, false
	}

	prefix := strings.ToLower(strings.TrimSpace(base[:underscore]))
	if prefix == "" {
		return "", ports.Attachment{}, false
	}

	prefixWithSeparator := prefix + "_"
	suffix := fmt.Sprintf("_%s%s", indexPart, ext)

	var (
		chosenKey  string
		chosenAtt  ports.Attachment
		chosenIter int
		found      bool
	)

	for key, att := range state.Attachments {
		if !strings.EqualFold(strings.TrimSpace(att.Source), "seedream") {
			continue
		}
		lowerKey := strings.ToLower(key)
		if !strings.HasSuffix(lowerKey, suffix) {
			continue
		}
		if !strings.HasPrefix(lowerKey, prefixWithSeparator) {
			continue
		}
		middle := strings.TrimSuffix(strings.TrimPrefix(lowerKey, prefixWithSeparator), suffix)
		if middle == "" {
			continue
		}

		iter := 0
		if state.AttachmentIterations != nil {
			iter = state.AttachmentIterations[key]
		}

		if !found || iter > chosenIter {
			found = true
			chosenKey = key
			chosenAtt = att
			chosenIter = iter
		}
	}

	if !found {
		return "", ports.Attachment{}, false
	}

	return chosenKey, chosenAtt, true
}

// matchGenericImageAlias expands generic image placeholders (image_N) to the
// newest generated image attachments when the model omits a filename.
func matchGenericImageAlias(name string, state *TaskState) (string, ports.Attachment, bool) {
	trimmed := strings.TrimSpace(name)
	match := genericImageAliasPattern.FindStringSubmatch(trimmed)
	if match == nil {
		return "", ports.Attachment{}, false
	}

	candidates := collectImageAttachmentCandidates(state)
	if len(candidates) == 0 {
		return "", ports.Attachment{}, false
	}

	index := len(candidates) - 1
	if len(match) > 1 && match[1] != "" {
		if parsed, err := strconv.Atoi(match[1]); err == nil && parsed > 0 {
			idx := parsed - 1
			if idx < len(candidates) {
				index = idx
			}
		}
	}

	chosen := candidates[index]
	return chosen.key, chosen.attachment, true
}

func collectImageAttachmentCandidates(state *TaskState) []attachmentCandidate {
	if state == nil || len(state.Attachments) == 0 {
		return nil
	}
	candidates := make([]attachmentCandidate, 0)
	for key, att := range state.Attachments {
		mediaType := strings.ToLower(strings.TrimSpace(att.MediaType))
		if !strings.HasPrefix(mediaType, "image/") {
			continue
		}
		iter := 0
		if state.AttachmentIterations != nil {
			iter = state.AttachmentIterations[key]
		}
		generated := !strings.EqualFold(strings.TrimSpace(att.Source), "user_upload")
		candidates = append(candidates, attachmentCandidate{
			key:        key,
			attachment: att,
			iteration:  iter,
			generated:  generated,
		})
	}

	sort.Slice(candidates, func(i, j int) bool {
		if candidates[i].generated != candidates[j].generated {
			return candidates[i].generated && !candidates[j].generated
		}
		if candidates[i].iteration == candidates[j].iteration {
			return candidates[i].key < candidates[j].key
		}
		return candidates[i].iteration < candidates[j].iteration
	})

	return candidates
}

func extractPlaceholderName(value string) (string, bool) {
	trimmed := strings.TrimSpace(value)
	if len(trimmed) < 3 {
		return "", false
	}
	if !strings.HasPrefix(trimmed, "[") || !strings.HasSuffix(trimmed, "]") {
		return "", false
	}
	name := strings.TrimSpace(trimmed[1 : len(trimmed)-1])
	if name == "" {
		return "", false
	}
	return name, true
}

func matchAttachmentReference(raw string, state *TaskState) (string, ports.Attachment, string, bool) {
	if name, ok := extractPlaceholderName(raw); ok {
		att, canonical, _, resolved := lookupAttachmentByNameInternal(name, state)
		if !resolved {
			return "", ports.Attachment{}, "", false
		}
		return name, att, canonical, true
	}

	trimmed := strings.TrimSpace(raw)
	if !looksLikeDirectAttachmentReference(trimmed) {
		return "", ports.Attachment{}, "", false
	}
	att, canonical, _, ok := lookupAttachmentByNameInternal(trimmed, state)
	if !ok {
		return "", ports.Attachment{}, "", false
	}
	return trimmed, att, canonical, true
}

func looksLikeDirectAttachmentReference(value string) bool {
	if value == "" {
		return false
	}
	if strings.ContainsAny(value, "\n\r\t") {
		return false
	}
	if strings.Contains(value, " ") {
		return false
	}
	lower := strings.ToLower(value)
	if strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://") || strings.HasPrefix(lower, "data:") {
		return false
	}
	if strings.HasPrefix(lower, "[") && strings.HasSuffix(lower, "]") {
		return false
	}
	return strings.Contains(value, ".")
}

func resolveContentAttachments(content string, state *TaskState) map[string]ports.Attachment {
	if state == nil || len(state.Attachments) == 0 || strings.TrimSpace(content) == "" {
		return nil
	}
	matches := contentPlaceholderPattern.FindAllStringSubmatch(content, -1)
	if len(matches) == 0 {
		return nil
	}
	resolved := make(map[string]ports.Attachment)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		name := strings.TrimSpace(match[1])
		if name == "" {
			continue
		}
		att, _, _, ok := lookupAttachmentByNameInternal(name, state)
		if !ok {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		resolved[name] = att
	}
	if len(resolved) == 0 {
		return nil
	}
	return resolved
}

func isA2UIAttachment(att ports.Attachment) bool {
	media := strings.ToLower(strings.TrimSpace(att.MediaType))
	format := strings.ToLower(strings.TrimSpace(att.Format))
	profile := strings.ToLower(strings.TrimSpace(att.PreviewProfile))
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

// ensureAttachmentPlaceholders strips all attachment placeholder markers from
// the final answer. Attachments are delivered as separate messages by downstream
// channels, so inline placeholders should not leak into the reply text.
// stripAttachmentPlaceholders removes all [placeholder] markers from the text.
// Attachments are delivered as separate messages by downstream channels.
func stripAttachmentPlaceholders(answer string) string {
	normalized := strings.TrimSpace(answer)
	replaced := contentPlaceholderPattern.ReplaceAllString(normalized, "")
	return strings.TrimSpace(replaced)
}

