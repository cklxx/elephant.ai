package domain

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"alex/internal/agent/ports"
	materialapi "alex/internal/materials/api"
)

// NewReactEngine creates a new ReAct engine with injected infrastructure dependencies.
func NewReactEngine(cfg ReactEngineConfig) *ReactEngine {
	logger := cfg.Logger
	if logger == nil {
		logger = ports.NoopLogger{}
	}

	clock := cfg.Clock
	if clock == nil {
		clock = ports.SystemClock{}
	}

	stopReasons := cfg.StopReasons
	if len(stopReasons) == 0 {
		stopReasons = []string{"final_answer", "done", "complete"}
	}

	maxIterations := cfg.MaxIterations
	if maxIterations <= 0 {
		maxIterations = 1
	}

	completion := buildCompletionDefaults(cfg.CompletionDefaults)

	return &ReactEngine{
		maxIterations:      maxIterations,
		stopReasons:        stopReasons,
		logger:             logger,
		clock:              clock,
		eventListener:      cfg.EventListener,
		completion:         completion,
		attachmentMigrator: cfg.AttachmentMigrator,
		workflow:           cfg.Workflow,
	}
}

func buildCompletionDefaults(cfg CompletionDefaults) completionConfig {
	temperature := 0.7
	if cfg.Temperature != nil {
		temperature = *cfg.Temperature
	}

	maxTokens := 12000
	if cfg.MaxTokens != nil && *cfg.MaxTokens > 0 {
		maxTokens = *cfg.MaxTokens
	}

	topP := 1.0
	if cfg.TopP != nil {
		topP = *cfg.TopP
	}

	stopSequences := make([]string, len(cfg.StopSequences))
	copy(stopSequences, cfg.StopSequences)

	return completionConfig{
		temperature:   temperature,
		maxTokens:     maxTokens,
		topP:          topP,
		stopSequences: stopSequences,
	}
}

// formatToolArgumentsForLog renders tool arguments into a log-friendly JSON
// string while stripping bulky or sensitive values.
func formatToolArgumentsForLog(args map[string]any) string {
	if len(args) == 0 {
		return "{}"
	}
	sanitized := sanitizeToolArgumentsForLog(args)
	if len(sanitized) == 0 {
		return "{}"
	}
	if encoded, err := json.Marshal(sanitized); err == nil {
		return string(encoded)
	}
	return fmt.Sprintf("%v", sanitized)
}

func sanitizeToolArgumentsForLog(args map[string]any) map[string]any {
	if args == nil {
		return nil
	}
	sanitized := make(map[string]any, len(args))
	for key, value := range args {
		sanitized[key] = summarizeToolArgumentValue(key, value)
	}
	return sanitized
}

func summarizeToolArgumentValue(key string, value any) any {
	switch v := value.(type) {
	case string:
		return summarizeToolArgumentString(key, v)
	case map[string]any:
		return sanitizeToolArgumentsForLog(v)
	case []any:
		summarized := make([]any, 0, len(v))
		for idx, item := range v {
			summarized = append(summarized, summarizeToolArgumentValue(fmt.Sprintf("%s[%d]", key, idx), item))
		}
		return summarized
	case []string:
		summarized := make([]string, 0, len(v))
		for _, item := range v {
			summarized = append(summarized, summarizeToolArgumentString(key, item))
		}
		return summarized
	default:
		return value
	}
}

func summarizeToolArgumentString(key, raw string) string {
	trimmed := strings.TrimSpace(raw)
	if trimmed == "" {
		return trimmed
	}

	lowerKey := strings.ToLower(key)
	if strings.HasPrefix(trimmed, "data:") {
		return summarizeDataURIForLog(trimmed)
	}

	if strings.Contains(lowerKey, "image") {
		if strings.HasPrefix(trimmed, "http://") || strings.HasPrefix(trimmed, "https://") {
			return trimmed
		}
		if len(trimmed) > toolArgInlineLengthLimit || looksLikeBinaryString(trimmed) {
			return summarizeBinaryLikeString(trimmed)
		}
		return trimmed
	}

	if looksLikeBinaryString(trimmed) {
		return summarizeBinaryLikeString(trimmed)
	}

	if len(trimmed) > toolArgInlineLengthLimit {
		return summarizeLongPlainString(trimmed)
	}

	return trimmed
}

func summarizeDataURIForLog(value string) string {
	comma := strings.Index(value, ",")
	if comma == -1 {
		return fmt.Sprintf("data_uri(len=%d)", len(value))
	}
	header := value[:comma]
	payload := value[comma+1:]
	preview := truncateStringForLog(payload, toolArgPreviewLength)
	if len(payload) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("data_uri(header=%q,len=%d,payload_prefix=%q)", header, len(value), preview)
}

func summarizeBinaryLikeString(value string) string {
	preview := truncateStringForLog(value, toolArgPreviewLength)
	if len(value) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("base64(len=%d,prefix=%q)", len(value), preview)
}

func summarizeLongPlainString(value string) string {
	preview := truncateStringForLog(value, toolArgPreviewLength)
	if len(value) > len(preview) {
		preview += "..."
	}
	return fmt.Sprintf("%s (len=%d)", preview, len(value))
}

func looksLikeBinaryString(value string) bool {
	if len(value) < toolArgInlineLengthLimit {
		return false
	}
	sample := value
	const sampleSize = 128
	if len(sample) > sampleSize {
		sample = sample[:sampleSize]
	}
	for i := 0; i < len(sample); i++ {
		c := sample[i]
		if (c >= 'a' && c <= 'z') ||
			(c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') ||
			c == '+' || c == '/' || c == '=' || c == '-' || c == '_' {
			continue
		}
		return false
	}
	return true
}

func truncateStringForLog(value string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runeCount := 0
	for idx := range value {
		if runeCount == limit {
			return value[:idx]
		}
		runeCount++
	}
	return value
}

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

// splitMessagesForLLM separates messages that are safe for the model from
// system-only entries (e.g., attachment catalogs) while deep-cloning them.
func splitMessagesForLLM(messages []Message) ([]Message, []Message) {
	if len(messages) == 0 {
		return nil, nil
	}
	filtered := make([]Message, 0, len(messages))
	excluded := make([]Message, 0)
	for _, msg := range messages {
		cloned := cloneMessageForLLM(msg)
		switch msg.Source {
		case ports.MessageSourceDebug, ports.MessageSourceEvaluation:
			excluded = append(excluded, cloned)
		default:
			filtered = append(filtered, cloned)
		}
	}
	return filtered, excluded
}

func cloneMessageForLLM(msg Message) Message {
	cloned := msg
	if len(msg.ToolCalls) > 0 {
		cloned.ToolCalls = append([]ToolCall(nil), msg.ToolCalls...)
	}
	if len(msg.ToolResults) > 0 {
		cloned.ToolResults = make([]ToolResult, len(msg.ToolResults))
		for i, result := range msg.ToolResults {
			cloned.ToolResults[i] = cloneToolResultForLLM(result)
		}
	}
	if len(msg.Metadata) > 0 {
		metadata := make(map[string]any, len(msg.Metadata))
		for key, value := range msg.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(msg.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(msg.Attachments)
	}
	return cloned
}

func cloneToolResultForLLM(result ToolResult) ToolResult {
	cloned := result
	if len(result.Metadata) > 0 {
		metadata := make(map[string]any, len(result.Metadata))
		for key, value := range result.Metadata {
			metadata[key] = value
		}
		cloned.Metadata = metadata
	}
	if len(result.Attachments) > 0 {
		cloned.Attachments = ports.CloneAttachmentMap(result.Attachments)
	}
	return cloned
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

func messageMaterialStatus(msg Message) materialapi.MaterialStatus {
	role := strings.ToLower(strings.TrimSpace(msg.Role))
	switch role {
	case "user":
		return materialapi.MaterialStatusInput
	case "assistant":
		return materialapi.MaterialStatusFinal
	case "tool":
		return materialapi.MaterialStatusIntermediate
	}
	switch msg.Source {
	case ports.MessageSourceUserInput, ports.MessageSourceUserHistory:
		return materialapi.MaterialStatusInput
	case ports.MessageSourceAssistantReply:
		return materialapi.MaterialStatusFinal
	case ports.MessageSourceToolResult:
		return materialapi.MaterialStatusIntermediate
	default:
		return materialapi.MaterialStatusIntermediate
	}
}

func messageMaterialOrigin(msg Message) string {
	if msg.Source != "" {
		return string(msg.Source)
	}
	if msg.ToolCallID != "" {
		return msg.ToolCallID
	}
	role := strings.TrimSpace(msg.Role)
	if role != "" {
		return role
	}
	return "message_history"
}

func isPreloadedContextMessage(msg Message) bool {
	if len(msg.Metadata) == 0 {
		return false
	}
	value, ok := msg.Metadata["rag_preload"]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case bool:
		return v
	case string:
		parsed, err := strconv.ParseBool(strings.TrimSpace(v))
		return err == nil && parsed
	case float64:
		return v != 0
	case int:
		return v != 0
	case int64:
		return v != 0
	case uint:
		return v != 0
	case uint64:
		return v != 0
	default:
		return false
	}
}

func isCurrentPreloadedContextMessage(msg Message, taskID string) bool {
	if taskID == "" || !isPreloadedContextMessage(msg) {
		return false
	}
	value, ok := msg.Metadata["rag_preload_task_id"]
	if !ok {
		return false
	}
	switch v := value.(type) {
	case string:
		return strings.TrimSpace(v) == taskID
	default:
		return false
	}
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
		description := strings.TrimSpace(att.Description)
		if description != "" {
			builder.WriteString(" — " + description)
		}
		builder.WriteString("\n")
	}

	builder.WriteString("\nUse the placeholders verbatim to work with these attachments in follow-up steps.")

	return strings.TrimSpace(builder.String())
}

func findAttachmentCatalogMessageIndex(state *TaskState) int {
	if state == nil || len(state.Messages) == 0 {
		return -1
	}
	for i := len(state.Messages) - 1; i >= 0; i-- {
		msg := state.Messages[i]
		if msg.Metadata == nil {
			continue
		}
		if flag, ok := msg.Metadata[attachmentCatalogMetadataKey]; ok {
			if enabled, ok := flag.(bool); ok && enabled {
				return i
			}
		}
	}
	return -1
}

func removeAttachmentCatalogMessage(state *TaskState) {
	idx := findAttachmentCatalogMessageIndex(state)
	if idx < 0 {
		return
	}
	state.Messages = append(state.Messages[:idx], state.Messages[idx+1:]...)
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

// snapshotSummaryFromMessages builds a short textual digest of the message
// history for context snapshots.
func snapshotSummaryFromMessages(messages []ports.Message) string {
	for i := len(messages) - 1; i >= 0; i-- {
		msg := messages[i]
		content := normalizeWhitespace(msg.Content)
		if content == "" {
			continue
		}
		prefix := roleSummaryPrefix(msg.Role)
		summary := prefix + content
		return truncateWithEllipsis(summary, snapshotSummaryLimit)
	}
	return ""
}

func normalizeWhitespace(input string) string {
	fields := strings.Fields(input)
	return strings.Join(fields, " ")
}

func roleSummaryPrefix(role string) string {
	trimmed := strings.TrimSpace(role)
	if trimmed == "" {
		return ""
	}
	lower := strings.ToLower(trimmed)
	switch lower {
	case "assistant":
		return "Assistant: "
	case "user":
		return "User: "
	case "tool":
		return "Tool: "
	case "system":
		return ""
	default:
		if len(trimmed) == 1 {
			return strings.ToUpper(trimmed) + ": "
		}
		return strings.ToUpper(trimmed[:1]) + strings.ToLower(trimmed[1:]) + ": "
	}
}

func truncateWithEllipsis(input string, limit int) string {
	if limit <= 0 {
		return ""
	}
	runes := []rune(input)
	if len(runes) <= limit {
		return input
	}
	if limit == 1 {
		return "…"
	}
	trimmed := strings.TrimSpace(string(runes[:limit-1]))
	if trimmed == "" {
		trimmed = string(runes[:limit-1])
	}
	return trimmed + "…"
}

func buildContextTurnRecord(state *ports.TaskState, messages []ports.Message, timestamp time.Time, summary string) ports.ContextTurnRecord {
	record := ports.ContextTurnRecord{
		Timestamp: timestamp,
		Summary:   summary,
		Messages:  append([]ports.Message(nil), messages...),
	}
	if state == nil {
		return record
	}
	record.SessionID = state.SessionID
	record.TurnID = state.Iterations
	record.LLMTurnSeq = state.Iterations
	record.Plans = clonePlanNodes(state.Plans)
	record.Beliefs = cloneBeliefs(state.Beliefs)
	record.KnowledgeRefs = cloneKnowledgeReferences(state.KnowledgeRefs)
	record.World = cloneMapAny(state.WorldState)
	record.Diff = cloneMapAny(state.WorldDiff)
	record.Feedback = cloneFeedbackSignals(state.FeedbackSignals)
	return record
}

func clonePlanNodes(nodes []ports.PlanNode) []ports.PlanNode {
	if len(nodes) == 0 {
		return nil
	}
	cloned := make([]ports.PlanNode, 0, len(nodes))
	for _, node := range nodes {
		copyNode := ports.PlanNode{
			ID:          node.ID,
			Title:       node.Title,
			Status:      node.Status,
			Description: node.Description,
		}
		copyNode.Children = clonePlanNodes(node.Children)
		cloned = append(cloned, copyNode)
	}
	return cloned
}

func cloneBeliefs(beliefs []ports.Belief) []ports.Belief {
	if len(beliefs) == 0 {
		return nil
	}
	cloned := make([]ports.Belief, 0, len(beliefs))
	for _, belief := range beliefs {
		cloned = append(cloned, ports.Belief{
			Statement:  belief.Statement,
			Confidence: belief.Confidence,
			Source:     belief.Source,
		})
	}
	return cloned
}

func cloneKnowledgeReferences(refs []ports.KnowledgeReference) []ports.KnowledgeReference {
	if len(refs) == 0 {
		return nil
	}
	cloned := make([]ports.KnowledgeReference, 0, len(refs))
	for _, ref := range refs {
		copyRef := ports.KnowledgeReference{
			ID:          ref.ID,
			Description: ref.Description,
		}
		copyRef.SOPRefs = append([]string(nil), ref.SOPRefs...)
		copyRef.RAGCollections = append([]string(nil), ref.RAGCollections...)
		copyRef.MemoryKeys = append([]string(nil), ref.MemoryKeys...)
		cloned = append(cloned, copyRef)
	}
	return cloned
}

func cloneFeedbackSignals(signals []ports.FeedbackSignal) []ports.FeedbackSignal {
	if len(signals) == 0 {
		return nil
	}
	cloned := make([]ports.FeedbackSignal, len(signals))
	copy(cloned, signals)
	return cloned
}

func cloneMapAny(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	cloned := make(map[string]any, len(input))
	for key, value := range input {
		cloned[key] = cloneWorldValue(value)
	}
	return cloned
}

func cloneWorldValue(value any) any {
	switch v := value.(type) {
	case map[string]any:
		return cloneMapAny(v)
	case []map[string]any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]map[string]any, len(v))
		for i := range v {
			cloned[i] = cloneMapAny(v[i])
		}
		return cloned
	case []string:
		return append([]string(nil), v...)
	case []any:
		if len(v) == 0 {
			return nil
		}
		cloned := make([]any, len(v))
		for i := range v {
			cloned[i] = cloneWorldValue(v[i])
		}
		return cloned
	default:
		return v
	}
}

func ensureWorldStateMap(state *TaskState) {
	if state.WorldState == nil {
		state.WorldState = make(map[string]any)
	}
}

// summarizeToolResultForWorld trims tool results down to safe, compact
// metadata for world-state persistence.
func summarizeToolResultForWorld(result ToolResult) map[string]any {
	entry := map[string]any{
		"call_id": strings.TrimSpace(result.CallID),
	}
	status := "success"
	if result.Error != nil {
		status = "error"
		entry["error"] = result.Error.Error()
	}
	entry["status"] = status
	if preview := summarizeForWorld(result.Content, toolResultPreviewRunes); preview != "" {
		entry["output_preview"] = preview
	}
	if metadata := summarizeWorldMetadata(result.Metadata); len(metadata) > 0 {
		entry["metadata"] = metadata
	}
	if names := summarizeAttachmentNames(result.Attachments); len(names) > 0 {
		entry["attachments"] = names
	}
	return entry
}

func summarizeForWorld(content string, limit int) string {
	trimmed := strings.TrimSpace(content)
	if trimmed == "" || limit <= 0 {
		return ""
	}
	runes := []rune(trimmed)
	if len(runes) <= limit {
		return trimmed
	}
	return strings.TrimSpace(string(runes[:limit])) + "…"
}

func summarizeWorldMetadata(metadata map[string]any) map[string]any {
	if len(metadata) == 0 {
		return nil
	}
	keys := make([]string, 0, len(metadata))
	for key := range metadata {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	summarized := make(map[string]any, len(keys))
	for _, key := range keys {
		value := summarizeMetadataValue(metadata[key])
		if value == nil {
			continue
		}
		summarized[key] = value
	}
	if len(summarized) == 0 {
		return nil
	}
	return summarized
}

func summarizeMetadataValue(value any) any {
	switch v := value.(type) {
	case string:
		return summarizeForWorld(v, toolResultPreviewRunes/2)
	case fmt.Stringer:
		return summarizeForWorld(v.String(), toolResultPreviewRunes/2)
	case float64, float32, int, int64, int32, uint64, uint32, bool:
		return v
	case []string:
		copySlice := make([]string, 0, len(v))
		for _, item := range v {
			copySlice = append(copySlice, summarizeForWorld(item, toolResultPreviewRunes/4))
		}
		return copySlice
	case map[string]any:
		return summarizeWorldMetadata(v)
	default:
		if v == nil {
			return nil
		}
		return summarizeForWorld(fmt.Sprintf("%v", v), toolResultPreviewRunes/3)
	}
}

func summarizeAttachmentNames(attachments map[string]ports.Attachment) []string {
	if len(attachments) == 0 {
		return nil
	}
	names := make([]string, 0, len(attachments))
	for key, att := range attachments {
		name := strings.TrimSpace(att.Name)
		if name == "" {
			name = strings.TrimSpace(key)
		}
		if name == "" {
			continue
		}
		names = append(names, name)
	}
	if len(names) == 0 {
		return nil
	}
	sort.Strings(names)
	return names
}

func deriveFeedbackValue(result ToolResult) float64 {
	if reward, ok := extractRewardValue(result.Metadata); ok {
		return reward
	}
	if result.Error != nil {
		return -1
	}
	return 1
}

func buildFeedbackMessage(result ToolResult) string {
	label := strings.TrimSpace(result.CallID)
	if label == "" {
		label = "tool"
	}
	status := "completed"
	if result.Error != nil {
		status = "errored"
	}
	if preview := summarizeForWorld(result.Content, toolResultPreviewRunes/3); preview != "" {
		return fmt.Sprintf("%s %s: %s", label, status, preview)
	}
	return fmt.Sprintf("%s %s", label, status)
}

func extractRewardValue(metadata map[string]any) (float64, bool) {
	if len(metadata) == 0 {
		return 0, false
	}
	for _, key := range []string{"reward", "score", "value"} {
		raw, ok := metadata[key]
		if !ok {
			continue
		}
		switch v := raw.(type) {
		case float64:
			return v, true
		case float32:
			return float64(v), true
		case int:
			return float64(v), true
		case int64:
			return float64(v), true
		case int32:
			return float64(v), true
		case uint64:
			return float64(v), true
		case uint32:
			return float64(v), true
		case string:
			if parsed, err := strconv.ParseFloat(strings.TrimSpace(v), 64); err == nil {
				return parsed, true
			}
		}
	}
	return 0, false
}

// ensureAttachmentPlaceholders appends missing attachment placeholders to a
// final answer so downstream channels surface linked artifacts.
func ensureAttachmentPlaceholders(answer string, attachments map[string]ports.Attachment) string {
	normalized := strings.TrimSpace(answer)

	// Strip unknown placeholders entirely when we have no attachment catalog.
	if len(attachments) == 0 {
		replaced := contentPlaceholderPattern.ReplaceAllString(normalized, "")
		return strings.TrimSpace(replaced)
	}

	used := make(map[string]bool, len(attachments))
	replaced := contentPlaceholderPattern.ReplaceAllStringFunc(normalized, func(match string) string {
		name := strings.TrimSpace(match[1 : len(match)-1])
		if name == "" {
			return ""
		}
		if _, ok := attachments[name]; !ok {
			return ""
		}
		used[name] = true
		return fmt.Sprintf("[%s]", name)
	})

	replaced = strings.TrimSpace(replaced)

	var missing []string
	for key := range attachments {
		name := strings.TrimSpace(key)
		if name == "" || used[name] {
			continue
		}
		missing = append(missing, name)
	}

	if len(missing) == 0 {
		return replaced
	}

	sort.Strings(missing)
	var builder strings.Builder
	if replaced != "" {
		builder.WriteString(replaced)
		builder.WriteString("\n\n")
	}
	builder.WriteString("Attachments available:\n")
	for _, name := range missing {
		fmt.Fprintf(&builder, "- [%s]\n", name)
	}
	return strings.TrimSpace(builder.String())
}
