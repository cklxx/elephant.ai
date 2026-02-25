package journal

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestFileReaderReadAll(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewFileWriter(dir)
	if err != nil {
		t.Fatalf("new writer: %v", err)
	}
	entry := TurnJournalEntry{SessionID: "sess-1", TurnID: 1, Summary: "ready"}
	if err := writer.Write(context.Background(), entry); err != nil {
		t.Fatalf("write journal: %v", err)
	}
	reader, err := NewFileReader(dir)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	entries, err := reader.ReadAll(context.Background(), "sess-1")
	if err != nil {
		t.Fatalf("ReadAll returned error: %v", err)
	}
	if len(entries) != 1 || entries[0].Summary != "ready" {
		t.Fatalf("unexpected entries: %+v", entries)
	}
}

func TestFileReaderStreamHonorsContext(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "sess-2.jsonl"), []byte("{"), 0o644); err != nil {
		t.Fatalf("seed corrupt journal: %v", err)
	}
	reader, err := NewFileReader(dir)
	if err != nil {
		t.Fatalf("new reader: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	err = reader.Stream(ctx, "sess-2", func(entry TurnJournalEntry) error { return nil })
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context canceled, got %v", err)
	}
}

func TestFileReaderStreamValidatesInput(t *testing.T) {
	reader := &FileReader{dir: t.TempDir()}
	if err := reader.Stream(context.Background(), "", func(TurnJournalEntry) error { return nil }); err == nil {
		t.Fatalf("expected error for empty session id")
	}
	if err := reader.Stream(context.Background(), "sess", nil); err == nil {
		t.Fatalf("expected error for nil callback")
	}
}
