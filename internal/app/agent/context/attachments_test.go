package context

import (
	"context"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestInheritedAttachmentsRoundTripClonesAttachmentsAndIterations(t *testing.T) {
	attachments := map[string]ports.Attachment{
		"artifact.md": {
			Name: "artifact.md",
			URI:  "https://example.com/artifact.md",
		},
	}
	iterations := map[string]int{
		"artifact.md": 3,
	}

	ctx := WithInheritedAttachments(context.Background(), attachments, iterations)

	attachments["artifact.md"] = ports.Attachment{Name: "artifact.md", URI: "https://mutated.example.com/artifact.md"}
	iterations["artifact.md"] = 10

	gotAttachments, gotIterations := GetInheritedAttachments(ctx)
	if gotAttachments["artifact.md"].URI != "https://example.com/artifact.md" {
		t.Fatalf("expected stored attachments to be cloned, got %+v", gotAttachments)
	}
	if gotIterations["artifact.md"] != 3 {
		t.Fatalf("expected stored iterations to be cloned, got %+v", gotIterations)
	}

	gotAttachments["artifact.md"] = ports.Attachment{Name: "artifact.md", URI: "https://changed-again.example.com/artifact.md"}
	gotIterations["artifact.md"] = 42

	gotAttachments2, gotIterations2 := GetInheritedAttachments(ctx)
	if gotAttachments2["artifact.md"].URI != "https://example.com/artifact.md" {
		t.Fatalf("expected retrieval clone isolation for attachments, got %+v", gotAttachments2)
	}
	if gotIterations2["artifact.md"] != 3 {
		t.Fatalf("expected retrieval clone isolation for iterations, got %+v", gotIterations2)
	}
}
