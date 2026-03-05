package main

import (
	"encoding/json"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestTeamStatusCLIRealInvocation(t *testing.T) {
	root := filepath.Join(t.TempDir(), "_team_runtime")
	now := time.Now().UTC().Truncate(time.Second)
	writeTeamRuntimeFixture(t, root, "session-integration", "team-integration", now)

	repoRoot, err := filepath.Abs(filepath.Join("..", ".."))
	if err != nil {
		t.Fatalf("resolve repo root: %v", err)
	}

	cmd := exec.Command("go", "run", "./cmd/alex", "team", "status", "--runtime-root", root, "--json", "--all", "--tail", "2")
	cmd.Dir = repoRoot
	output, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("team status command failed: %v\noutput: %s", err, string(output))
	}

	var report struct {
		Count   int `json:"count"`
		Entries []struct {
			SessionID    string `json:"session_id"`
			TeamID       string `json:"team_id"`
			RecentEvents []any  `json:"recent_events"`
		} `json:"entries"`
	}
	if err := json.Unmarshal(output, &report); err != nil {
		t.Fatalf("unmarshal team status json: %v\noutput: %s", err, string(output))
	}
	if report.Count != 1 {
		t.Fatalf("expected count=1, got %d", report.Count)
	}
	if len(report.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(report.Entries))
	}
	if report.Entries[0].SessionID != "session-integration" || report.Entries[0].TeamID != "team-integration" {
		t.Fatalf("unexpected entry: %+v", report.Entries[0])
	}
	if len(report.Entries[0].RecentEvents) != 2 {
		t.Fatalf("expected tail=2 events, got %d", len(report.Entries[0].RecentEvents))
	}
}
