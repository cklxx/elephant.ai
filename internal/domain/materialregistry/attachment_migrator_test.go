package materialregistry

import (
	"context"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	materialports "alex/internal/domain/materialregistry/ports"
	"alex/internal/shared/logging"
)

type recordingStore struct {
	records []storedAttachment
	err     error
}

type storedAttachment struct {
	name      string
	mediaType string
	data      []byte
}

func (s *recordingStore) StoreBytes(name, mediaType string, data []byte) (string, error) {
	s.records = append(s.records, storedAttachment{name: name, mediaType: mediaType, data: data})
	if s.err != nil {
		return "", s.err
	}
	return "https://cdn.example.com/" + name, nil
}

type mockFetcher struct {
	data        []byte
	contentType string
	err         error
}

func (f *mockFetcher) Fetch(_ context.Context, _ string, _ string) ([]byte, string, error) {
	return f.data, f.contentType, f.err
}

func TestAttachmentStoreMigratorUploadsInlinePayloads(t *testing.T) {
	store := &recordingStore{}
	migrator := NewAttachmentStoreMigrator(store, nil, "https://cdn.example.com", logging.Nop())

	attachments := map[string]ports.Attachment{
		"image.png": {Name: "image.png", MediaType: "image/png", Data: "aW1hZ2U=", URI: "data:image/png;base64,aW1hZ2U="},
	}

	result, err := migrator.Normalize(context.Background(), materialports.MigrationRequest{Attachments: attachments})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(store.records) != 1 {
		t.Fatalf("expected one upload, got %d", len(store.records))
	}
	if got := string(store.records[0].data); got != "image" {
		t.Fatalf("unexpected payload: %s", got)
	}

	att := result["image.png"]
	if att.Data != "" {
		t.Fatalf("expected inline data to be stripped, got %q", att.Data)
	}
	if !strings.HasPrefix(att.URI, "https://cdn.example.com") {
		t.Fatalf("expected URI to use CDN, got %q", att.URI)
	}
}

func TestAttachmentStoreMigratorFetchesRemoteContent(t *testing.T) {
	store := &recordingStore{}
	fetcher := &mockFetcher{data: []byte("hello"), contentType: "text/plain"}
	migrator := NewAttachmentStoreMigrator(store, fetcher, "", logging.Nop())

	result, err := migrator.Normalize(context.Background(), materialports.MigrationRequest{
		Attachments: map[string]ports.Attachment{
			"note.txt": {Name: "note.txt", URI: "https://example.com/file"},
		},
	})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(store.records) != 1 {
		t.Fatalf("expected one upload, got %d", len(store.records))
	}
	if store.records[0].mediaType != "text/plain" {
		t.Fatalf("expected media type from response, got %q", store.records[0].mediaType)
	}
	if got := string(store.records[0].data); got != "hello" {
		t.Fatalf("unexpected payload %q", got)
	}

	if att := result["note.txt"]; att.URI == "" {
		t.Fatalf("expected migrated uri to be set")
	}
}

func TestAttachmentStoreMigratorSkipsHostedAttachments(t *testing.T) {
	store := &recordingStore{}
	migrator := NewAttachmentStoreMigrator(store, nil, "https://cdn.example.com", logging.Nop())

	attachments := map[string]ports.Attachment{
		"hosted.png": {Name: "hosted.png", URI: "https://cdn.example.com/abc.png"},
	}

	result, err := migrator.Normalize(context.Background(), materialports.MigrationRequest{Attachments: attachments})
	if err != nil {
		t.Fatalf("expected nil error, got %v", err)
	}
	if len(store.records) != 0 {
		t.Fatalf("expected no uploads, got %d", len(store.records))
	}
	if result["hosted.png"].URI != attachments["hosted.png"].URI {
		t.Fatalf("expected URI to remain unchanged")
	}
}
