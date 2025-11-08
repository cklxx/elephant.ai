package domain

import (
	"testing"

	"alex/internal/agent/ports"
)

func TestCollectGeneratedAttachmentsIncludesAllGeneratedUpToIteration(t *testing.T) {
	state := &TaskState{
		Attachments: map[string]ports.Attachment{
			"v1.png": {
				Name:      "v1.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"v2.png": {
				Name:      "v2.png",
				MediaType: "image/png",
				Source:    "seedream",
			},
			"user.png": {
				Name:      "user.png",
				MediaType: "image/png",
				Source:    "user_upload",
			},
		},
		AttachmentIterations: map[string]int{
			"v1.png": 1,
			"v2.png": 2,
		},
	}

	got := collectGeneratedAttachments(state, 2)
	if len(got) != 2 {
		t.Fatalf("expected two generated attachments, got %d", len(got))
	}
	if _, ok := got["v1.png"]; !ok {
		t.Fatalf("expected earlier iteration attachment to be present: %+v", got)
	}
	if _, ok := got["v2.png"]; !ok {
		t.Fatalf("expected latest iteration attachment to be present: %+v", got)
	}
	if _, ok := got["user.png"]; ok {
		t.Fatalf("did not expect user-uploaded attachment in result: %+v", got)
	}
}
