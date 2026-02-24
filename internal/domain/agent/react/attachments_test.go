package react

import (
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestNormalizeToolAttachmentsFillsMissingName(t *testing.T) {
	input := map[string]ports.Attachment{
		"seed.png": {
			URI: "https://example.com/seed.png",
		},
	}

	got := normalizeToolAttachments(input)
	if len(got) != 1 {
		t.Fatalf("expected one normalized attachment, got %+v", got)
	}

	att := got["seed.png"]
	if att.Name != "seed.png" {
		t.Fatalf("expected missing name to be filled from placeholder, got %q", att.Name)
	}
	if att.URI != "https://example.com/seed.png" {
		t.Fatalf("expected attachment payload to be preserved, got %+v", att)
	}
}

func TestNormalizeToolAttachmentsKeepsWhitespaceOnlyName(t *testing.T) {
	input := map[string]ports.Attachment{
		"seed.png": {
			Name: "   ",
			URI:  "https://example.com/seed.png",
		},
	}

	got := normalizeToolAttachments(input)
	att := got["seed.png"]
	if att.Name != "   " {
		t.Fatalf("expected whitespace-only name to remain unchanged, got %q", att.Name)
	}
}

func TestNormalizeAttachmentMapFillsWhitespaceOnlyName(t *testing.T) {
	input := map[string]ports.Attachment{
		"seed.png": {
			Name: "   ",
			URI:  "https://example.com/seed.png",
		},
	}

	got := normalizeAttachmentMap(input)
	att := got["seed.png"]
	if att.Name != "seed.png" {
		t.Fatalf("expected whitespace-only name to be replaced by placeholder, got %q", att.Name)
	}
}
