package attachments

import (
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestPopulateWorkspacePathsAnnotatesAttachments(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"analysis.txt": {Name: "analysis.txt", MediaType: "text/plain"},
		"diagram":      {Name: "diagram", MediaType: "image/png"},
		"":             {Name: "fallback", MediaType: "application/pdf"},
	}

	PopulateWorkspacePaths(DefaultWorkspaceSessionDir, " session-123 ", attachments)

	for key, att := range attachments {
		if att.WorkspacePath == "" {
			t.Fatalf("expected workspace path for %s", key)
		}
		if !strings.HasPrefix(att.WorkspacePath, "/workspace/.alex/sessions/session-123/attachments/") {
			t.Fatalf("unexpected path prefix for %s: %s", key, att.WorkspacePath)
		}
	}

	if !strings.HasSuffix(attachments["diagram"].WorkspacePath, ".png") {
		t.Fatalf("expected extension inference for png: %s", attachments["diagram"].WorkspacePath)
	}
	if !strings.HasSuffix(attachments[""].WorkspacePath, ".pdf") {
		t.Fatalf("expected extension inference for pdf: %s", attachments[""].WorkspacePath)
	}
}
