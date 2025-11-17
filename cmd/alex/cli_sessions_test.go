package main

import (
	"bytes"
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"alex/internal/di"
	sessionstate "alex/internal/session/state_store"
)

func TestPullSessionSnapshotByTurn(t *testing.T) {
	store := sessionstate.NewInMemoryStore()
	snapshot := sessionstate.Snapshot{
		SessionID:  "sess-1",
		TurnID:     1,
		LLMTurnSeq: 2,
		CreatedAt:  time.Date(2024, 11, 11, 10, 0, 0, 0, time.UTC),
		Summary:    "observed",
	}
	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}

	cli := &CLI{container: &Container{Container: &di.Container{StateStore: store}}}
	var buf bytes.Buffer
	tempDir := t.TempDir()
	outputPath := filepath.Join(tempDir, "turn.json")
	args := []string{"sess-1", "--turn", "1", "--output", outputPath}
	if err := cli.pullSessionSnapshotsWithWriter(context.Background(), args, &buf); err != nil {
		t.Fatalf("pull snapshots: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "Turn 1 (LLM turn 2)") {
		t.Fatalf("expected turn summary in output, got %q", out)
	}
	data, err := os.ReadFile(outputPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if !strings.Contains(string(data), "\"turn_id\": 1") {
		t.Fatalf("expected JSON payload in file, got %s", data)
	}
}

func TestPullSessionSnapshotList(t *testing.T) {
	store := sessionstate.NewInMemoryStore()
	snapshot := sessionstate.Snapshot{
		SessionID:  "sess-list",
		TurnID:     3,
		LLMTurnSeq: 3,
		CreatedAt:  time.Date(2024, 11, 11, 11, 0, 0, 0, time.UTC),
		Summary:    "loop",
	}
	if err := store.SaveSnapshot(context.Background(), snapshot); err != nil {
		t.Fatalf("seed snapshot: %v", err)
	}
	cli := &CLI{container: &Container{Container: &di.Container{StateStore: store}}}
	var buf bytes.Buffer
	if err := cli.pullSessionSnapshotsWithWriter(context.Background(), []string{"sess-list"}, &buf); err != nil {
		t.Fatalf("list snapshots: %v", err)
	}
	if !strings.Contains(buf.String(), "Snapshots for session sess-list") {
		t.Fatalf("expected list header, got %q", buf.String())
	}
}

func TestPullSessionSnapshotByLLMTurn(t *testing.T) {
	store := sessionstate.NewInMemoryStore()
	entries := []sessionstate.Snapshot{
		{SessionID: "sess-llm", TurnID: 1, LLMTurnSeq: 1, CreatedAt: time.Now()},
		{SessionID: "sess-llm", TurnID: 2, LLMTurnSeq: 5, CreatedAt: time.Now()},
	}
	for _, snap := range entries {
		if err := store.SaveSnapshot(context.Background(), snap); err != nil {
			t.Fatalf("seed snapshot: %v", err)
		}
	}
	cli := &CLI{container: &Container{Container: &di.Container{StateStore: store}}}
	var buf bytes.Buffer
	if err := cli.pullSessionSnapshotsWithWriter(context.Background(), []string{"sess-llm", "--llm-turn", "5"}, &buf); err != nil {
		t.Fatalf("pull by llm turn: %v", err)
	}
	if !strings.Contains(buf.String(), "LLM turn 5") {
		t.Fatalf("expected llm turn output, got %q", buf.String())
	}
}
