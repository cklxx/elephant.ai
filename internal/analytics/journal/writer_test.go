package journal

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/agent/ports"
)

func TestFileWriterAppendsEntries(t *testing.T) {
	dir := t.TempDir()
	writer, err := NewFileWriter(dir)
	if err != nil {
		t.Fatalf("NewFileWriter: %v", err)
	}

	entry := TurnJournalEntry{
		SessionID:  "sess-1",
		TurnID:     1,
		LLMTurnSeq: 2,
		Timestamp:  time.Unix(1700000000, 0),
		Summary:    "answered user",
		Plans:      []ports.PlanNode{{ID: "plan-1", Title: "Do thing"}},
	}
	if err := writer.Write(context.Background(), entry); err != nil {
		t.Fatalf("Write: %v", err)
	}

	path := filepath.Join(dir, "sess-1.jsonl")
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	var stored TurnJournalEntry
	if err := json.Unmarshal(data[:len(data)-1], &stored); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if stored.SessionID != entry.SessionID || stored.TurnID != entry.TurnID {
		t.Fatalf("unexpected stored entry: %+v", stored)
	}
}
