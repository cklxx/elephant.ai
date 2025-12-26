package http

import (
	"encoding/base64"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/attachments"
)

func TestParseAttachmentsPreservesInlineBase64ForImages(t *testing.T) {
	store, err := NewAttachmentStore(attachments.StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("create attachment store: %v", err)
	}
	handler := &APIHandler{attachmentStore: store}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	b64 := base64.StdEncoding.EncodeToString(png)

	attachments, err := handler.parseAttachments([]AttachmentPayload{{
		Name:      "cat.png",
		MediaType: "image/png",
		Data:      b64,
	}})
	if err != nil {
		t.Fatalf("parseAttachments: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	att := attachments[0]
	if att.Data != b64 {
		t.Fatalf("expected inline base64 to be preserved, got %q", att.Data)
	}
	if !strings.HasPrefix(att.URI, "/api/attachments/") {
		t.Fatalf("expected attachment URI to be stored, got %q", att.URI)
	}
	filename := strings.TrimPrefix(att.URI, "/api/attachments/")
	if _, statErr := os.Stat(filepath.Join(store.LocalDir(), filename)); statErr != nil {
		t.Fatalf("expected stored file to exist: %v", statErr)
	}
}

func TestParseAttachmentsPreservesInlineBase64ForImageDataURI(t *testing.T) {
	store, err := NewAttachmentStore(attachments.StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("create attachment store: %v", err)
	}
	handler := &APIHandler{attachmentStore: store}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a}
	b64 := base64.StdEncoding.EncodeToString(png)

	attachments, err := handler.parseAttachments([]AttachmentPayload{{
		Name:      "cat.png",
		MediaType: "image/png",
		URI:       fmt.Sprintf("data:image/png;base64,%s", b64),
	}})
	if err != nil {
		t.Fatalf("parseAttachments: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	att := attachments[0]
	if att.Data != b64 {
		t.Fatalf("expected inline base64 to be preserved, got %q", att.Data)
	}
	if !strings.HasPrefix(att.URI, "/api/attachments/") {
		t.Fatalf("expected attachment URI to be stored, got %q", att.URI)
	}
}

func TestParseAttachmentsDoesNotRetainInlineBase64ForNonImages(t *testing.T) {
	store, err := NewAttachmentStore(attachments.StoreConfig{Dir: t.TempDir()})
	if err != nil {
		t.Fatalf("create attachment store: %v", err)
	}
	handler := &APIHandler{attachmentStore: store}

	payload := base64.StdEncoding.EncodeToString([]byte("hello"))

	attachments, err := handler.parseAttachments([]AttachmentPayload{{
		Name:      "note.txt",
		MediaType: "text/plain",
		Data:      payload,
	}})
	if err != nil {
		t.Fatalf("parseAttachments: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	att := attachments[0]
	if att.Data != "" {
		t.Fatalf("expected inline payload to be omitted for non-image attachment, got %q", att.Data)
	}
	if !strings.HasPrefix(att.URI, "/api/attachments/") {
		t.Fatalf("expected attachment URI to be stored, got %q", att.URI)
	}
}

func TestParseAttachmentsPreservesSource(t *testing.T) {
	handler := &APIHandler{}

	attachments, err := handler.parseAttachments([]AttachmentPayload{{
		Name:      "scene.png",
		MediaType: "image/png",
		URI:       "https://example.com/scene.png",
		Source:    "camera_upload",
	}})
	if err != nil {
		t.Fatalf("parseAttachments: %v", err)
	}
	if len(attachments) != 1 {
		t.Fatalf("expected 1 attachment, got %d", len(attachments))
	}
	if attachments[0].Source != "camera_upload" {
		t.Fatalf("expected source to be preserved, got %q", attachments[0].Source)
	}
}
