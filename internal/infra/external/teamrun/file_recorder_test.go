package teamrun

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	"gopkg.in/yaml.v3"
)

func TestFileRecorder_RecordTeamRunWritesYAMLFile(t *testing.T) {
	baseDir := t.TempDir()
	recorder, err := NewFileRecorder(baseDir, nil)
	if err != nil {
		t.Fatalf("NewFileRecorder() error = %v", err)
	}

	record := agent.TeamRunRecord{
		TeamName: "Execute And Report",
		Goal:     "Ship feature",
	}
	path, err := recorder.RecordTeamRun(context.Background(), record)
	if err != nil {
		t.Fatalf("RecordTeamRun() error = %v", err)
	}
	if strings.TrimSpace(path) == "" {
		t.Fatal("expected non-empty record path")
	}
	if filepath.Dir(path) != baseDir {
		t.Fatalf("expected record under base dir %q, got %q", baseDir, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read record file: %v", err)
	}
	var persisted persistedTeamRunRecord
	if err := yaml.Unmarshal(data, &persisted); err != nil {
		t.Fatalf("unmarshal persisted record: %v", err)
	}
	if persisted.Version != 1 {
		t.Fatalf("expected version=1, got %d", persisted.Version)
	}
	if persisted.Record.RunID == "" {
		t.Fatalf("expected generated run id, got %+v", persisted.Record)
	}
	if persisted.Record.DispatchState != "dispatched" {
		t.Fatalf("expected default dispatch_state=dispatched, got %q", persisted.Record.DispatchState)
	}
}

func TestNewFileRecorderRequiresBaseDir(t *testing.T) {
	recorder, err := NewFileRecorder("   ", nil)
	if err == nil {
		t.Fatalf("expected error for empty base dir, got recorder=%v", recorder)
	}
}

func TestSanitizeFileToken(t *testing.T) {
	if got := sanitizeFileToken("  Team Alpha/Bravo  "); got != "team-alpha-bravo" {
		t.Fatalf("unexpected sanitized token: %q", got)
	}
	if got := sanitizeFileToken("!!!"); got != "" {
		t.Fatalf("expected empty token for punctuation-only input, got %q", got)
	}
}
