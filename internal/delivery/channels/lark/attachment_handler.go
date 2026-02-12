package lark

import (
	"context"
	"fmt"
	"mime"
	"path/filepath"
	"sort"
	"strings"

	artifactruntime "alex/internal/app/artifactruntime"
	toolcontext "alex/internal/app/toolcontext"
	ports "alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	toolports "alex/internal/domain/agent/ports/tools"
)

func (g *Gateway) sendAttachments(ctx context.Context, chatID, messageID string, result *agent.TaskResult) {
	if result == nil || g.messenger == nil {
		return
	}

	attachments := filterNonA2UIAttachments(result.Attachments)
	if len(attachments) == 0 {
		return
	}

	ctx = toolcontext.WithAllowLocalFetch(ctx)
	ctx = toolports.WithAttachmentContext(ctx, attachments, nil)
	client := artifactruntime.NewAttachmentHTTPClient(artifactruntime.AttachmentFetchTimeout, "LarkAttachment")
	maxBytes, allowExts := autoUploadLimits(ctx)

	names := sortedAttachmentNames(attachments)
	for _, name := range names {
		att := attachments[name]
		payload, mediaType, err := artifactruntime.ResolveAttachmentBytes(ctx, "["+name+"]", client)
		if err != nil {
			g.logger.Warn("Lark attachment %s resolve failed: %v", name, err)
			continue
		}

		fileName := fileNameForAttachment(att, name)
		if !allowExtension(filepath.Ext(fileName), allowExts) {
			g.logger.Warn("Lark attachment %s blocked by allowlist", fileName)
			continue
		}
		if maxBytes > 0 && len(payload) > maxBytes {
			g.logger.Warn("Lark attachment %s exceeds max size %d bytes", fileName, maxBytes)
			continue
		}

		target := replyTarget(messageID, true)

		if isImageAttachment(att, mediaType, name) {
			imageKey, err := g.uploadImage(ctx, payload)
			if err != nil {
				g.logger.Warn("Lark image upload failed (%s): %v", name, err)
				continue
			}
			g.dispatch(ctx, chatID, target, "image", imageContent(imageKey))
			continue
		}

		fileType := larkFileType(fileTypeForAttachment(fileName, mediaType))
		fileKey, err := g.uploadFile(ctx, payload, fileName, fileType)
		if err != nil {
			g.logger.Warn("Lark file upload failed (%s): %v", name, err)
			continue
		}
		g.dispatch(ctx, chatID, target, "file", fileContent(fileKey))
	}
}

func autoUploadLimits(ctx context.Context) (int, []string) {
	cfg := toolcontext.GetAutoUploadConfig(ctx)
	maxBytes := cfg.MaxBytes
	if maxBytes <= 0 {
		maxBytes = 2 * 1024 * 1024
	}
	return maxBytes, normalizeExtensions(cfg.AllowExts)
}

func allowExtension(ext string, allowlist []string) bool {
	if len(allowlist) == 0 {
		return true
	}
	ext = strings.ToLower(strings.TrimSpace(ext))
	if ext == "" {
		return false
	}
	for _, item := range allowlist {
		if strings.ToLower(strings.TrimSpace(item)) == ext {
			return true
		}
	}
	return false
}

func filterNonA2UIAttachments(attachments map[string]ports.Attachment) map[string]ports.Attachment {
	if len(attachments) == 0 {
		return nil
	}
	filtered := make(map[string]ports.Attachment, len(attachments))
	for name, att := range attachments {
		if isA2UIAttachment(att) {
			continue
		}
		filtered[name] = att
	}
	if len(filtered) == 0 {
		return nil
	}
	return filtered
}

func collectAttachmentsFromResult(result *agent.TaskResult) map[string]ports.Attachment {
	if result == nil || len(result.Messages) == 0 {
		return nil
	}

	attachments := make(map[string]ports.Attachment)
	for _, msg := range result.Messages {
		mergeAttachments(attachments, msg.Attachments)
		if len(msg.ToolResults) > 0 {
			for _, res := range msg.ToolResults {
				mergeAttachments(attachments, res.Attachments)
			}
		}
	}
	if len(attachments) == 0 {
		return nil
	}
	return attachments
}

func mergeAttachments(out map[string]ports.Attachment, incoming map[string]ports.Attachment) {
	if len(incoming) == 0 {
		return
	}
	for key, att := range incoming {
		name := strings.TrimSpace(key)
		if name == "" {
			name = strings.TrimSpace(att.Name)
		}
		if name == "" {
			continue
		}
		if _, exists := out[name]; exists {
			continue
		}
		if att.Name == "" {
			att.Name = name
		}
		out[name] = att
	}
}

// buildAttachmentSummary creates a text summary of non-A2UI attachments
// with CDN URLs appended to the reply. This consolidates attachment
// references into the summary message so users see everything in one place.
func buildAttachmentSummary(result *agent.TaskResult) string {
	if result == nil {
		return ""
	}
	attachments := result.Attachments
	if len(attachments) == 0 {
		return ""
	}
	names := sortedAttachmentNames(attachments)
	var lines []string
	for _, name := range names {
		att := attachments[name]
		if isA2UIAttachment(att) {
			continue
		}
		uri := strings.TrimSpace(att.URI)
		if uri == "" || strings.HasPrefix(strings.ToLower(uri), "data:") {
			lines = append(lines, fmt.Sprintf("- %s", name))
			continue
		}
		lines = append(lines, fmt.Sprintf("- %s: %s", name, uri))
	}
	if len(lines) == 0 {
		return ""
	}
	return "---\n[Attachments]\n" + strings.Join(lines, "\n")
}

func sortedAttachmentNames(attachments map[string]ports.Attachment) []string {
	names := make([]string, 0, len(attachments))
	for name := range attachments {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

func isA2UIAttachment(att ports.Attachment) bool {
	media := strings.ToLower(strings.TrimSpace(att.MediaType))
	format := strings.ToLower(strings.TrimSpace(att.Format))
	profile := strings.ToLower(strings.TrimSpace(att.PreviewProfile))
	return strings.Contains(media, "a2ui") || format == "a2ui" || strings.Contains(profile, "a2ui")
}

func isImageAttachment(att ports.Attachment, mediaType, name string) bool {
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(mediaType)), "image/") {
		return true
	}
	if strings.HasPrefix(strings.ToLower(strings.TrimSpace(att.MediaType)), "image/") {
		return true
	}
	switch strings.ToLower(filepath.Ext(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".bmp", ".webp":
		return true
	default:
		return false
	}
}

func fileNameForAttachment(att ports.Attachment, fallback string) string {
	name := strings.TrimSpace(att.Name)
	if name == "" {
		name = strings.TrimSpace(fallback)
	}
	if name == "" {
		name = "attachment"
	}
	if filepath.Ext(name) == "" {
		if ext := extensionForMediaType(att.MediaType); ext != "" {
			name += ext
		}
	}
	return name
}

// larkSupportedFileTypes lists the file_type values accepted by the Lark
// im/v1/files upload API. Any extension not in this set must be sent as "stream".
var larkSupportedFileTypes = map[string]bool{
	"opus": true, "mp4": true, "pdf": true,
	"doc": true, "xls": true, "ppt": true,
	"stream": true,
}

// larkFileType maps a raw file extension to a Lark-compatible file_type value.
func larkFileType(ext string) string {
	lower := strings.ToLower(strings.TrimSpace(ext))
	if larkSupportedFileTypes[lower] {
		return lower
	}
	return "stream"
}

func fileTypeForAttachment(name, mediaType string) string {
	if ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(name)), "."); ext != "" {
		return ext
	}
	if ext := strings.TrimPrefix(extensionForMediaType(mediaType), "."); ext != "" {
		return ext
	}
	return "bin"
}

func extensionForMediaType(mediaType string) string {
	trimmed := strings.TrimSpace(mediaType)
	if trimmed == "" {
		return ""
	}
	exts, err := mime.ExtensionsByType(trimmed)
	if err != nil || len(exts) == 0 {
		return ""
	}
	return exts[0]
}

func (g *Gateway) uploadImage(ctx context.Context, payload []byte) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UploadImage(ctx, payload)
}

func (g *Gateway) uploadFile(ctx context.Context, payload []byte, fileName, fileType string) (string, error) {
	if g.messenger == nil {
		return "", fmt.Errorf("lark messenger not initialized")
	}
	return g.messenger.UploadFile(ctx, payload, fileName, fileType)
}
