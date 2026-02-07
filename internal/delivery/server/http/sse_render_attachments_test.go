package http

import (
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
)

func TestAttachmentDigest_ChangesWithPreviewAsset(t *testing.T) {
	att := ports.Attachment{
		Name:      "report.html",
		MediaType: "text/html",
		URI:       "/api/data/a",
		PreviewAssets: []ports.AttachmentPreviewAsset{
			{AssetID: "a1", CDNURL: "/api/data/a", MimeType: "text/html", PreviewType: "iframe"},
		},
	}
	before := attachmentDigest(att)

	att.PreviewAssets[0].CDNURL = "/api/data/b"
	after := attachmentDigest(att)

	if before == after {
		t.Fatalf("expected digest to change when preview asset changes")
	}
}

func TestSanitizeAttachmentsForStream_SkipsUnchangedURIAttachments(t *testing.T) {
	sent := newStringLRU(8)
	cache := NewDataCache(16, time.Minute)

	attachments := map[string]ports.Attachment{
		"artifact.html": {
			Name:      "artifact.html",
			MediaType: "text/html",
			URI:       "/api/data/abc",
		},
	}

	first := sanitizeAttachmentsForStream(attachments, sent, cache, false)
	if len(first) != 1 {
		t.Fatalf("expected first pass to include attachment, got %d", len(first))
	}

	second := sanitizeAttachmentsForStream(attachments, sent, cache, false)
	if second != nil {
		t.Fatalf("expected second pass to skip unchanged attachment, got %v", second)
	}

	updated := map[string]ports.Attachment{
		"artifact.html": {
			Name:      "artifact.html",
			MediaType: "text/html",
			URI:       "/api/data/xyz",
		},
	}
	resent := sanitizeAttachmentsForStream(updated, sent, cache, false)
	if len(resent) != 1 {
		t.Fatalf("expected updated attachment to be resent, got %d", len(resent))
	}
}
