package attachments

import (
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	dir := t.TempDir()
	store, err := NewStore(StoreConfig{Provider: ProviderLocal, Dir: dir})
	if err != nil {
		t.Fatalf("NewStore: %v", err)
	}
	return store
}

func TestStorePersister_AlreadyHasExternalURI(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	att := ports.Attachment{
		Name:      "photo.png",
		MediaType: "image/png",
		URI:       "https://cdn.example.com/abc.png",
	}
	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.URI != att.URI {
		t.Errorf("URI changed: got %q, want %q", got.URI, att.URI)
	}
	if got.Data != "" {
		t.Errorf("Data should remain empty, got %q", got.Data)
	}
}

func TestStorePersister_Base64Data(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	content := []byte("hello world binary content that is large enough")
	encoded := base64.StdEncoding.EncodeToString(content)

	att := ports.Attachment{
		Name:      "file.bin",
		MediaType: "application/octet-stream",
		Data:      encoded,
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.URI == "" {
		t.Fatal("URI should be populated after persist")
	}
	if !strings.HasPrefix(got.URI, "/api/attachments/") {
		t.Errorf("URI should start with /api/attachments/, got %q", got.URI)
	}
	// Binary content above retention limit → Data should be cleared.
	if got.Data != "" {
		t.Errorf("Data should be cleared for binary content, got len=%d", len(got.Data))
	}
}

func TestStorePersister_DataURI(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	content := []byte("some binary payload for data uri test")
	encoded := base64.StdEncoding.EncodeToString(content)

	att := ports.Attachment{
		Name:      "data.bin",
		MediaType: "application/pdf",
		URI:       "data:application/pdf;base64," + encoded,
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.URI == "" || strings.HasPrefix(got.URI, "data:") {
		t.Fatalf("URI should be a store path, got %q", got.URI)
	}
	if got.Data != "" {
		t.Errorf("Data should be cleared for binary, got len=%d", len(got.Data))
	}
}

func TestStorePersister_SmallTextRetainsData(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	content := []byte("# Small Markdown Note\n\nHello world.")
	encoded := base64.StdEncoding.EncodeToString(content)

	att := ports.Attachment{
		Name:      "note.md",
		MediaType: "text/markdown",
		Data:      encoded,
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.URI == "" {
		t.Fatal("URI should be populated")
	}
	// Small text → Data retained.
	if got.Data == "" {
		t.Error("Data should be retained for small text/markdown")
	}
	decoded, _ := base64.StdEncoding.DecodeString(got.Data)
	if string(decoded) != string(content) {
		t.Errorf("retained Data mismatch: got %q", string(decoded))
	}
}

func TestStorePersister_LargeTextClearsData(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	// Generate content larger than inlineRetentionLimit (4096).
	content := []byte(strings.Repeat("x", inlineRetentionLimit+100))
	encoded := base64.StdEncoding.EncodeToString(content)

	att := ports.Attachment{
		Name:      "big.md",
		MediaType: "text/markdown",
		Data:      encoded,
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if got.URI == "" {
		t.Fatal("URI should be populated")
	}
	if got.Data != "" {
		t.Errorf("Data should be cleared for large text, got len=%d", len(got.Data))
	}
}

func TestStorePersister_EmptyPayload(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	att := ports.Attachment{
		Name:      "empty.txt",
		MediaType: "text/plain",
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// No data → returned unchanged.
	if got.URI != "" {
		t.Errorf("URI should remain empty for no-data attachment, got %q", got.URI)
	}
}

func TestStorePersister_NilStore(t *testing.T) {
	p := NewStorePersister(nil)
	att := ports.Attachment{
		Name:      "file.bin",
		MediaType: "application/octet-stream",
		Data:      base64.StdEncoding.EncodeToString([]byte("test")),
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// Nil store → returned unchanged (graceful degradation).
	if got.Data != att.Data {
		t.Error("Data should remain unchanged with nil store")
	}
}

func TestStorePersister_FileWritten(t *testing.T) {
	store := newTestStore(t)
	p := NewStorePersister(store)
	content := []byte("persistent content for disk check")
	encoded := base64.StdEncoding.EncodeToString(content)

	att := ports.Attachment{
		Name:      "check.bin",
		MediaType: "application/octet-stream",
		Data:      encoded,
	}

	got, err := p.Persist(att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify file exists on disk.
	filename := strings.TrimPrefix(got.URI, "/api/attachments/")
	path := filepath.Join(store.LocalDir(), filename)
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("file not written: %v", err)
	}
	if string(data) != string(content) {
		t.Errorf("file content mismatch: got %q, want %q", string(data), string(content))
	}
}

func TestStorePersister_Idempotent(t *testing.T) {
	p := NewStorePersister(newTestStore(t))
	content := []byte("idempotent test content")
	encoded := base64.StdEncoding.EncodeToString(content)

	att := ports.Attachment{
		Name:      "idem.bin",
		MediaType: "application/octet-stream",
		Data:      encoded,
	}

	first, err := p.Persist(att)
	if err != nil {
		t.Fatalf("first persist: %v", err)
	}

	// Persist the result again → should be a no-op (already has URI, no Data).
	second, err := p.Persist(first)
	if err != nil {
		t.Fatalf("second persist: %v", err)
	}

	if second.URI != first.URI {
		t.Errorf("URI changed on second persist: %q → %q", first.URI, second.URI)
	}
}
