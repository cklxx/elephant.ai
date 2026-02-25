package attachments

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
)

func TestAsyncStorePersister_PersistsInBackground(t *testing.T) {
	store := newTestStore(t)
	p := NewAsyncStorePersister(store, WithAsyncPersistWorkers(1), WithAsyncPersistQueueSize(4))
	defer p.Close()

	content := []byte("async attachment content")
	att := ports.Attachment{
		Name:      "async.txt",
		MediaType: "text/plain",
		Data:      base64.StdEncoding.EncodeToString(content),
	}

	got, err := p.Persist(context.Background(), att)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.URI == "" || !strings.HasPrefix(got.URI, "/api/attachments/") {
		t.Fatalf("expected async persister to return attachment URI, got %q", got.URI)
	}
	if got.Fingerprint == "" {
		t.Fatalf("expected fingerprint to be populated")
	}

	filename := strings.TrimPrefix(got.URI, "/api/attachments/")
	pathOnDisk := filepath.Join(store.LocalDir(), filename)

	deadline := time.After(2 * time.Second)
	for {
		if _, err := os.Stat(pathOnDisk); err == nil {
			break
		}
		select {
		case <-deadline:
			t.Fatalf("async persist did not write file: %s", pathOnDisk)
		case <-time.After(10 * time.Millisecond):
		}
	}

	data, err := os.ReadFile(pathOnDisk)
	if err != nil {
		t.Fatalf("read persisted file: %v", err)
	}
	if string(data) != string(content) {
		t.Fatalf("persisted content mismatch: got %q", string(data))
	}
}
