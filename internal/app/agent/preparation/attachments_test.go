package preparation

import (
	"testing"

	"alex/internal/domain/agent/ports"
	storage "alex/internal/domain/agent/ports/storage"
)

func TestCollectSessionAttachmentsMergesAndOverrides(t *testing.T) {
	session := &storage.Session{
		Attachments: map[string]ports.Attachment{
			"report.md": {Name: "report.md", MediaType: "text/plain"},
		},
		Messages: []ports.Message{
			{
				Attachments: map[string]ports.Attachment{
					"report.md": {Name: "report.md", MediaType: "application/pdf"},
					"  ":        {Name: "diagram.png", MediaType: "image/png"},
				},
			},
		},
	}

	attachments := collectSessionAttachments(session)
	if len(attachments) != 2 {
		t.Fatalf("expected 2 attachments, got %d", len(attachments))
	}
	if attachments["report.md"].MediaType != "application/pdf" {
		t.Fatalf("expected later attachment to override, got %q", attachments["report.md"].MediaType)
	}
	if _, ok := attachments["diagram.png"]; !ok {
		t.Fatalf("expected fallback to attachment name, got %#v", attachments)
	}
}

func TestTaskNeedsVisionByPlaceholderImageExtension(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"Scan.TIFF": {Name: "Scan.TIFF"},
	}
	if !taskNeedsVision("Please analyze [scan.tiff]", attachments, nil) {
		t.Fatal("expected placeholder image extension to require vision")
	}
}

func TestTaskNeedsVisionSkipsNonImageAttachments(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"report.pdf": {Name: "report.pdf", MediaType: "application/pdf"},
	}
	if taskNeedsVision("Please analyze [report.pdf]", attachments, nil) {
		t.Fatal("expected non-image attachment to skip vision")
	}
}
