package domain

import (
	"alex/internal/agent/ports"
	attachments "alex/internal/attachments"
)

// ApplyWorkspacePaths ensures attachments include a workspace path that mirrors
// the sandbox file location for the current session.
func ApplyWorkspacePaths(sessionID string, values map[string]ports.Attachment) {
	attachments.PopulateWorkspacePaths(attachments.DefaultWorkspaceSessionDir, sessionID, values)
}
