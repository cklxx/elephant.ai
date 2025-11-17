package attachments

import (
	"path"
	"regexp"
	"strings"

	"alex/internal/agent/ports"
)

const (
	// DefaultWorkspaceSessionDir mirrors the sandbox path where attachments are
	// staged for each session.
	DefaultWorkspaceSessionDir = "/workspace/.alex/sessions"
	attachmentsSubdir          = "attachments"
)

var (
	sessionSanitizer = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
	fileSanitizer    = regexp.MustCompile(`[^a-zA-Z0-9._-]`)
)

// SanitizeSessionID normalizes session identifiers so they can be used in
// sandbox directory names without shell escaping.
func SanitizeSessionID(sessionID string) string {
	session := strings.TrimSpace(sessionID)
	session = sessionSanitizer.ReplaceAllString(session, "_")
	return strings.Trim(session, "._-")
}

// SanitizeFileName converts an attachment placeholder or filename into a
// filesystem-safe value. Media type is used to backfill extensions when missing.
func SanitizeFileName(candidate, attachmentName, mediaType string) string {
	name := strings.TrimSpace(attachmentName)
	if name == "" {
		name = strings.TrimSpace(candidate)
	}
	if name == "" {
		return ""
	}
	name = path.Base(name)
	name = fileSanitizer.ReplaceAllString(name, "_")
	if idx := strings.LastIndex(name, "."); idx == -1 {
		if ext := InferExtension(mediaType); ext != "" {
			name = name + "." + ext
		}
	}
	return name
}

// InferExtension attempts to infer a suitable extension based on the media type.
func InferExtension(mediaType string) string {
	switch strings.ToLower(strings.TrimSpace(mediaType)) {
	case "image/png":
		return "png"
	case "image/jpeg", "image/jpg":
		return "jpg"
	case "image/gif":
		return "gif"
	case "text/plain":
		return "txt"
	case "text/html":
		return "html"
	case "text/markdown":
		return "md"
	case "application/pdf":
		return "pdf"
	default:
		return ""
	}
}

// WorkspaceDir returns the sandbox directory that mirrors the current session's
// attachments. When baseDir is empty DefaultWorkspaceSessionDir is used.
func WorkspaceDir(baseDir, sessionID string) string {
	dir := strings.TrimSpace(baseDir)
	if dir == "" {
		dir = DefaultWorkspaceSessionDir
	}
	session := SanitizeSessionID(sessionID)
	if session == "" {
		session = "session"
	}
	return path.Join(dir, session, attachmentsSubdir)
}

// WorkspacePath returns the sandbox path for the provided attachment.
func WorkspacePath(baseDir, sessionID, placeholder, attachmentName, mediaType string) string {
	filename := SanitizeFileName(placeholder, attachmentName, mediaType)
	if filename == "" {
		return ""
	}
	return path.Join(WorkspaceDir(baseDir, sessionID), filename)
}

// PopulateWorkspacePaths annotates attachments with their sandbox path so tools
// and downstream systems can reference files directly.
func PopulateWorkspacePaths(baseDir, sessionID string, attachments map[string]ports.Attachment) {
	if len(attachments) == 0 {
		return
	}
	if strings.TrimSpace(baseDir) == "" {
		baseDir = DefaultWorkspaceSessionDir
	}
	for key, att := range attachments {
		if att.WorkspacePath != "" {
			continue
		}
		placeholder := strings.TrimSpace(key)
		if placeholder == "" {
			placeholder = strings.TrimSpace(att.Name)
		}
		path := WorkspacePath(baseDir, sessionID, placeholder, att.Name, att.MediaType)
		if path == "" {
			continue
		}
		att.WorkspacePath = path
		attachments[key] = att
	}
}
