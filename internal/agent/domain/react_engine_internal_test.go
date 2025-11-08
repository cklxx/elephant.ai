package domain

import (
	"testing"

	"alex/internal/agent/ports"
)

func TestCollectGeneratedAttachmentsFiltersByIteration(t *testing.T) {
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
	if len(got) != 1 {
		t.Fatalf("expected only one attachment for iteration 2, got %d", len(got))
	}
	if _, ok := got["v2.png"]; !ok {
		t.Fatalf("expected attachment from iteration 2 to be present, got %+v", got)
	}
	if _, ok := got["v1.png"]; ok {
		t.Fatalf("did not expect iteration 1 attachment in result: %+v", got)
	}
	if _, ok := got["user.png"]; ok {
		t.Fatalf("did not expect user-uploaded attachment in result: %+v", got)
	}
}
