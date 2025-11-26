package meta

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"alex/internal/agent/ports"
	"alex/internal/analytics/journal"
)

func TestStewardBuildsMetaFromJournal(t *testing.T) {
	dir := t.TempDir()
	writer, err := journal.NewFileWriter(dir)
	if err != nil {
		t.Fatalf("failed to create writer: %v", err)
	}
	entry := journal.TurnJournalEntry{
		SessionID: "sess-1",
		TurnID:    1,
		Timestamp: time.Now(),
		Summary:   "User asked about travel",
		Feedback:  []ports.FeedbackSignal{{Note: "Prefer concise summaries"}},
	}
	if err := writer.Write(context.Background(), entry); err != nil {
		t.Fatalf("failed to write journal: %v", err)
	}

	out := filepath.Join(dir, "meta.json")
	steward := NewSteward()
	metaCtx, err := steward.Run(context.Background(), ReplayConfig{InputDir: dir, OutputPath: out, PersonaID: "p1", PersonaVersion: "v1"})
	if err != nil {
		t.Fatalf("steward run failed: %v", err)
	}
	if len(metaCtx.Memories) != 1 {
		t.Fatalf("expected 1 memory, got %d", len(metaCtx.Memories))
	}
	if len(metaCtx.Recommendations) != 1 {
		t.Fatalf("expected 1 recommendation, got %d", len(metaCtx.Recommendations))
	}

	loaded, err := LoadMetaContext(out)
	if err != nil {
		t.Fatalf("failed to load meta output: %v", err)
	}
	if _, ok := loaded["p1"]; !ok {
		t.Fatalf("expected persona key in output file")
	}
}

func TestStewardRequiresPersonaID(t *testing.T) {
	dir := t.TempDir()
	if _, err := NewSteward().Run(context.Background(), ReplayConfig{InputDir: dir}); err == nil {
		t.Fatalf("expected error when persona id missing")
	}
}

func TestValidateOutputRejectsEmptyMemory(t *testing.T) {
	err := ValidateOutput(ports.MetaContext{Memories: []ports.MemoryFragment{{Content: ""}}})
	if err == nil {
		t.Fatalf("expected empty memory content to fail validation")
	}
}
