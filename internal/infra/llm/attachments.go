package llm

import (
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"alex/internal/agent/ports"
)

var attachmentPlaceholderPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

type attachmentDescriptor struct {
	Placeholder string
	Attachment  ports.Attachment
}

func orderedImageAttachments(content string, attachments map[string]ports.Attachment) []attachmentDescriptor {
	if len(attachments) == 0 {
		return nil
	}

	index := buildAttachmentIndex(attachments)

	used := make(map[string]bool)
	ordered := make([]attachmentDescriptor, 0, len(attachments))

	for _, match := range attachmentPlaceholderPattern.FindAllStringSubmatch(content, -1) {
		if len(match) < 2 {
			continue
		}
		placeholder := strings.TrimSpace(match[1])
		if placeholder == "" {
			continue
		}
		att, canonical, ok := index.resolve(placeholder)
		if !ok {
			continue
		}
		if used[canonical] {
			continue
		}
		if !isImageAttachment(att, canonical) {
			continue
		}

		used[canonical] = true
		ordered = append(ordered, attachmentDescriptor{
			Placeholder: canonical,
			Attachment:  att,
		})
	}

	remaining := index.sortedKeys()
	for _, key := range remaining {
		if used[key] {
			continue
		}
		att := attachments[key]
		if !isImageAttachment(att, key) {
			continue
		}
		ordered = append(ordered, attachmentDescriptor{
			Placeholder: key,
			Attachment:  att,
		})
	}

	if len(ordered) == 0 {
		return nil
	}
	return ordered
}

type attachmentIndex struct {
	attachments map[string]ports.Attachment
	byLower     map[string]string
	keys        []string
}

func buildAttachmentIndex(attachments map[string]ports.Attachment) attachmentIndex {
	index := attachmentIndex{
		attachments: attachments,
		byLower:     make(map[string]string, len(attachments)*2),
	}

	type sortableKey struct {
		key     string
		sortKey string
	}
	keys := make([]sortableKey, 0, len(attachments))
	for key, att := range attachments {
		canonical := strings.TrimSpace(key)
		if canonical == "" {
			continue
		}
		keys = append(keys, sortableKey{key: key, sortKey: strings.ToLower(canonical)})

		lowerKey := strings.ToLower(canonical)
		if _, exists := index.byLower[lowerKey]; !exists {
			index.byLower[lowerKey] = key
		}

		if name := strings.TrimSpace(att.Name); name != "" {
			lowerName := strings.ToLower(name)
			if _, exists := index.byLower[lowerName]; !exists {
				index.byLower[lowerName] = key
			}
		}
	}

	sort.Slice(keys, func(i, j int) bool {
		return keys[i].sortKey < keys[j].sortKey
	})
	if len(keys) > 0 {
		index.keys = make([]string, 0, len(keys))
		for _, item := range keys {
			index.keys = append(index.keys, item.key)
		}
	}
	return index
}

func (i attachmentIndex) resolve(placeholder string) (ports.Attachment, string, bool) {
	if len(i.attachments) == 0 {
		return ports.Attachment{}, "", false
	}

	name := strings.TrimSpace(placeholder)
	if name == "" {
		return ports.Attachment{}, "", false
	}

	if att, ok := i.attachments[name]; ok {
		return att, name, true
	}

	if canonical, ok := i.byLower[strings.ToLower(name)]; ok {
		att, ok := i.attachments[canonical]
		if ok {
			return att, canonical, true
		}
	}

	return ports.Attachment{}, "", false
}

func (i attachmentIndex) sortedKeys() []string {
	if len(i.keys) == 0 {
		return nil
	}
	return append([]string(nil), i.keys...)
}

// embedAttachmentImages walks message content, resolves inline [placeholder] references
// to image attachments, and appends any remaining unembedded images at the end.
// onInlineImage is called for images resolved from content placeholders.
// onTrailingImage is called for remaining images not referenced in content.
// Both callbacks return true if the image was consumed (marks the key as used).
func embedAttachmentImages(
	content string,
	attachments map[string]ports.Attachment,
	appendText func(string),
	onInlineImage func(att ports.Attachment, key string) bool,
	onTrailingImage func(att ports.Attachment, key string) bool,
) {
	index := buildAttachmentIndex(attachments)
	used := make(map[string]bool)

	cursor := 0
	matches := attachmentPlaceholderPattern.FindAllStringSubmatchIndex(content, -1)
	for _, match := range matches {
		if len(match) < 4 {
			continue
		}
		if match[0] > cursor {
			appendText(content[cursor:match[0]])
		}
		placeholderToken := content[match[0]:match[1]]
		appendText(placeholderToken)

		name := strings.TrimSpace(content[match[2]:match[3]])
		if name == "" {
			cursor = match[1]
			continue
		}
		if att, key, ok := index.resolve(name); ok && isImageAttachment(att, key) && !used[key] {
			if onInlineImage(att, key) {
				used[key] = true
			}
		}
		cursor = match[1]
	}
	if cursor < len(content) {
		appendText(content[cursor:])
	}

	for _, desc := range orderedImageAttachments(content, attachments) {
		key := desc.Placeholder
		if key == "" || used[key] {
			continue
		}
		if onTrailingImage(desc.Attachment, key) {
			used[key] = true
		}
	}
}

func isImageAttachment(att ports.Attachment, placeholder string) bool {
	mediaType := strings.ToLower(strings.TrimSpace(att.MediaType))
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}

	name := strings.TrimSpace(att.Name)
	if name == "" {
		name = strings.TrimSpace(placeholder)
	}
	if name == "" {
		return false
	}

	ext := strings.ToLower(filepath.Ext(name))
	switch ext {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}
