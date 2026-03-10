package react

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/shared/utils"
)

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

	prefix := utils.TrimLower(base[:underscore])
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
		mediaType := utils.TrimLower(att.MediaType)
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
	if state == nil || len(state.Attachments) == 0 || utils.IsBlank(content) {
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
