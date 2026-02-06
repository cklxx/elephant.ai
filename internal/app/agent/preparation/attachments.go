package preparation

import (
	"fmt"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	storage "alex/internal/domain/agent/ports/storage"
)

func collectSessionAttachments(session *storage.Session) map[string]ports.Attachment {
	attachments := make(map[string]ports.Attachment)
	if session == nil {
		return attachments
	}

	mergeAttachmentMaps(attachments, session.Attachments)
	for _, msg := range session.Messages {
		mergeAttachmentMaps(attachments, msg.Attachments)
	}
	return attachments
}

func mergeAttachmentMaps(target map[string]ports.Attachment, source map[string]ports.Attachment) {
	if len(source) == 0 {
		return
	}
	for key, att := range source {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		target[name] = att
	}
}

func collectSessionImportant(session *storage.Session) map[string]ports.ImportantNote {
	notes := make(map[string]ports.ImportantNote)
	if session == nil {
		return notes
	}
	mergeImportantNotes(notes, session.Important)
	return notes
}

func mergeImportantNotes(target map[string]ports.ImportantNote, source map[string]ports.ImportantNote) {
	if len(source) == 0 {
		return
	}
	for key, note := range source {
		id := strings.TrimSpace(key)
		if id == "" {
			id = strings.TrimSpace(note.ID)
		}
		if id == "" {
			continue
		}
		if note.ID == "" {
			note.ID = id
		}
		target[id] = note
	}
}

func buildImportantNotesMessage(notes map[string]ports.ImportantNote) *ports.Message {
	if len(notes) == 0 {
		return nil
	}
	type annotated struct {
		id   string
		note ports.ImportantNote
	}
	items := make([]annotated, 0, len(notes))
	for id, note := range notes {
		content := strings.TrimSpace(note.Content)
		if content == "" {
			continue
		}
		note.Content = content
		items = append(items, annotated{id: id, note: note})
	}
	if len(items) == 0 {
		return nil
	}
	sort.SliceStable(items, func(i, j int) bool {
		left := items[i].note.CreatedAt
		right := items[j].note.CreatedAt
		if !left.Equal(right) {
			return left.Before(right)
		}
		return items[i].id < items[j].id
	})

	var builder strings.Builder
	builder.WriteString("Important session notes (auto-recalled after compression):\n")
	for idx, item := range items {
		builder.WriteString(fmt.Sprintf("%d. %s", idx+1, item.note.Content))
		var metaParts []string
		if len(item.note.Tags) > 0 {
			metaParts = append(metaParts, fmt.Sprintf("tags: %s", strings.Join(item.note.Tags, ",")))
		}
		if source := strings.TrimSpace(item.note.Source); source != "" {
			metaParts = append(metaParts, fmt.Sprintf("source: %s", source))
		}
		if !item.note.CreatedAt.IsZero() {
			metaParts = append(metaParts, fmt.Sprintf("recorded: %s", item.note.CreatedAt.Format(time.RFC3339)))
		}
		if len(metaParts) > 0 {
			builder.WriteString(" (")
			builder.WriteString(strings.Join(metaParts, "; "))
			builder.WriteString(")")
		}
		if idx < len(items)-1 {
			builder.WriteString("\n")
		}
	}

	return &ports.Message{
		Role:    "system",
		Content: builder.String(),
		Source:  ports.MessageSourceImportant,
	}
}

var visionPlaceholderPattern = regexp.MustCompile(`\[([^\[\]]+)\]`)

func taskNeedsVision(task string, attachments map[string]ports.Attachment, userAttachments []ports.Attachment) bool {
	for _, att := range userAttachments {
		if isImageAttachment(att) {
			return true
		}
	}

	if strings.TrimSpace(task) == "" || len(attachments) == 0 {
		return false
	}

	matches := visionPlaceholderPattern.FindAllStringSubmatch(task, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		placeholder := strings.TrimSpace(match[1])
		if placeholder == "" {
			continue
		}
		att, ok := lookupAttachmentByName(attachments, placeholder)
		if !ok {
			continue
		}
		if isImageAttachment(att) {
			return true
		}
	}

	return false
}

func lookupAttachmentByName(attachments map[string]ports.Attachment, name string) (ports.Attachment, bool) {
	if len(attachments) == 0 {
		return ports.Attachment{}, false
	}
	if att, ok := attachments[name]; ok {
		return att, true
	}
	for key, att := range attachments {
		if strings.EqualFold(key, name) || strings.EqualFold(att.Name, name) {
			return att, true
		}
	}
	return ports.Attachment{}, false
}

func isImageAttachment(att ports.Attachment) bool {
	mediaType := strings.ToLower(strings.TrimSpace(att.MediaType))
	if strings.HasPrefix(mediaType, "image/") {
		return true
	}
	name := strings.TrimSpace(att.Name)
	if name == "" {
		return false
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".tif", ".tiff":
		return true
	default:
		return false
	}
}
