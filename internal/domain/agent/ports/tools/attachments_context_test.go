package tools

import (
	"context"
	"testing"

	core "alex/internal/domain/agent/ports"
)

func TestAttachmentContextRoundTripClonesAttachmentsAndIterations(t *testing.T) {
	attachments := map[string]core.Attachment{
		"diagram.png": {
			Name: "diagram.png",
			URI:  "https://example.com/diagram.png",
		},
	}
	iterations := map[string]int{
		"diagram.png": 2,
	}

	ctx := WithAttachmentContext(context.Background(), attachments, iterations)

	attachments["diagram.png"] = core.Attachment{Name: "diagram.png", URI: "https://mutated.example.com/diagram.png"}
	iterations["diagram.png"] = 9

	gotAttachments, gotIterations := GetAttachmentContext(ctx)
	if gotAttachments["diagram.png"].URI != "https://example.com/diagram.png" {
		t.Fatalf("expected stored attachments to be cloned, got %+v", gotAttachments)
	}
	if gotIterations["diagram.png"] != 2 {
		t.Fatalf("expected stored iterations to be cloned, got %+v", gotIterations)
	}

	gotAttachments["diagram.png"] = core.Attachment{Name: "diagram.png", URI: "https://changed-again.example.com/diagram.png"}
	gotIterations["diagram.png"] = 99

	gotAttachments2, gotIterations2 := GetAttachmentContext(ctx)
	if gotAttachments2["diagram.png"].URI != "https://example.com/diagram.png" {
		t.Fatalf("expected retrieval clone isolation for attachments, got %+v", gotAttachments2)
	}
	if gotIterations2["diagram.png"] != 2 {
		t.Fatalf("expected retrieval clone isolation for iterations, got %+v", gotIterations2)
	}
}
