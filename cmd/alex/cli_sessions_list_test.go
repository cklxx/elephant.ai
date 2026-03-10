package main

import (
	"bytes"
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"

	"alex/internal/app/agent/coordinator"
	appconfig "alex/internal/app/agent/config"
	"alex/internal/app/di"
	core "alex/internal/domain/agent/ports"
	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/session/filestore"
)

// newTestCLI creates a CLI with a file-based session store seeded with the given sessions.
func newTestCLI(t *testing.T, sessions []*agentstorage.Session) *CLI {
	t.Helper()
	dir := t.TempDir()
	store := filestore.New(dir)
	ctx := context.Background()

	for _, sess := range sessions {
		created, err := store.Create(ctx)
		if err != nil {
			t.Fatalf("create session: %v", err)
		}
		// Overwrite with desired data
		sess.ID = created.ID
		if err := store.Save(ctx, sess); err != nil {
			t.Fatalf("save session: %v", err)
		}
	}

	coord := coordinator.NewAgentCoordinator(nil, nil, store, nil, nil, nil, nil, appconfig.Config{})
	return &CLI{container: &Container{Container: &di.Container{
		SessionStore:     store,
		AgentCoordinator: coord,
	}}}
}

func TestListSessions_Table(t *testing.T) {
	now := time.Now()
	sessions := []*agentstorage.Session{
		{
			Messages:  []core.Message{{Role: "user", Content: "hello"}, {Role: "assistant", Content: "hi"}},
			Metadata:  map[string]string{"title": "Test Chat"},
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now.Add(-10 * time.Minute),
		},
	}
	cli := newTestCLI(t, sessions)

	var buf bytes.Buffer
	if err := cli.listSessionsWithWriter(context.Background(), &buf, false); err != nil {
		t.Fatalf("list sessions: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Sessions: 1") {
		t.Errorf("expected header, got: %s", out)
	}
	if !strings.Contains(out, "TITLE") {
		t.Errorf("expected table header, got: %s", out)
	}
	if !strings.Contains(out, "Test Chat") {
		t.Errorf("expected session title, got: %s", out)
	}
	if !strings.Contains(out, "MSGS") {
		t.Errorf("expected MSGS column, got: %s", out)
	}
}

func TestListSessions_JSON(t *testing.T) {
	now := time.Now()
	sessions := []*agentstorage.Session{
		{
			Messages:  []core.Message{{Role: "user", Content: "test"}},
			Metadata:  map[string]string{"title": "JSON Test"},
			CreatedAt: now.Add(-2 * time.Hour),
			UpdatedAt: now,
		},
	}
	cli := newTestCLI(t, sessions)

	var buf bytes.Buffer
	if err := cli.listSessionsWithWriter(context.Background(), &buf, true); err != nil {
		t.Fatalf("list sessions json: %v", err)
	}

	var rows []sessionListRow
	if err := json.Unmarshal(buf.Bytes(), &rows); err != nil {
		t.Fatalf("unmarshal json: %v\nraw: %s", err, buf.String())
	}
	if len(rows) != 1 {
		t.Fatalf("expected 1 row, got %d", len(rows))
	}
	if rows[0].Title != "JSON Test" {
		t.Errorf("expected title 'JSON Test', got %q", rows[0].Title)
	}
	if rows[0].Messages != 1 {
		t.Errorf("expected 1 message, got %d", rows[0].Messages)
	}
}

func TestListSessions_Empty(t *testing.T) {
	cli := newTestCLI(t, nil)

	var buf bytes.Buffer
	if err := cli.listSessionsWithWriter(context.Background(), &buf, false); err != nil {
		t.Fatalf("list sessions: %v", err)
	}
	if !strings.Contains(buf.String(), "No sessions found") {
		t.Errorf("expected 'No sessions found', got: %s", buf.String())
	}
}

func TestListSessions_EmptyJSON(t *testing.T) {
	cli := newTestCLI(t, nil)

	var buf bytes.Buffer
	if err := cli.listSessionsWithWriter(context.Background(), &buf, true); err != nil {
		t.Fatalf("list sessions json: %v", err)
	}
	if strings.TrimSpace(buf.String()) != "[]" {
		t.Errorf("expected empty JSON array, got: %s", buf.String())
	}
}

func TestInspectSession(t *testing.T) {
	now := time.Now()
	sessions := []*agentstorage.Session{
		{
			Messages: []core.Message{
				{Role: "user", Content: "What is Go?"},
				{Role: "assistant", Content: "Go is a programming language."},
				{Role: "user", Content: "Thanks"},
			},
			Todos:     []agentstorage.Todo{{ID: "t1", Description: "Learn Go", Status: "pending"}},
			Metadata:  map[string]string{"title": "Go Chat", "session_id": "sess-inspect"},
			CreatedAt: now.Add(-1 * time.Hour),
			UpdatedAt: now,
		},
	}
	cli := newTestCLI(t, sessions)

	// Get the actual session ID from the store
	ids, _ := cli.listAllSessions(context.Background())
	if len(ids) != 1 {
		t.Fatalf("expected 1 session, got %d", len(ids))
	}
	sessionID := ids[0]

	var buf bytes.Buffer
	if err := cli.inspectSessionWithWriter(context.Background(), sessionID, &buf, false); err != nil {
		t.Fatalf("inspect session: %v", err)
	}

	out := buf.String()
	if !strings.Contains(out, "Session:") {
		t.Errorf("expected 'Session:' header, got: %s", out)
	}
	if !strings.Contains(out, "Go Chat") {
		t.Errorf("expected title 'Go Chat', got: %s", out)
	}
	if !strings.Contains(out, "Messages: 3") {
		t.Errorf("expected 'Messages: 3', got: %s", out)
	}
	if !strings.Contains(out, "user=2") {
		t.Errorf("expected 'user=2' in breakdown, got: %s", out)
	}
	if !strings.Contains(out, "assistant=1") {
		t.Errorf("expected 'assistant=1' in breakdown, got: %s", out)
	}
	if !strings.Contains(out, "Todos:    1") {
		t.Errorf("expected 'Todos:    1', got: %s", out)
	}
}

func TestInspectSession_JSON(t *testing.T) {
	now := time.Now()
	sessions := []*agentstorage.Session{
		{
			Messages:  []core.Message{{Role: "user", Content: "hi"}, {Role: "assistant", Content: "hey"}},
			Metadata:  map[string]string{"title": "JSON Inspect"},
			CreatedAt: now.Add(-30 * time.Minute),
			UpdatedAt: now,
		},
	}
	cli := newTestCLI(t, sessions)

	ids, _ := cli.listAllSessions(context.Background())
	sessionID := ids[0]

	var buf bytes.Buffer
	if err := cli.inspectSessionWithWriter(context.Background(), sessionID, &buf, true); err != nil {
		t.Fatalf("inspect session json: %v", err)
	}

	var result sessionInspectResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("unmarshal: %v\nraw: %s", err, buf.String())
	}
	if result.Title != "JSON Inspect" {
		t.Errorf("expected 'JSON Inspect', got %q", result.Title)
	}
	if result.MessageCount != 2 {
		t.Errorf("expected 2 messages, got %d", result.MessageCount)
	}
	if result.Roles["user"] != 1 || result.Roles["assistant"] != 1 {
		t.Errorf("unexpected roles: %v", result.Roles)
	}
}

func TestInspectSession_NotFound(t *testing.T) {
	cli := newTestCLI(t, nil)

	var buf bytes.Buffer
	err := cli.inspectSessionWithWriter(context.Background(), "nonexistent", &buf, false)
	if err == nil {
		t.Fatal("expected error for nonexistent session")
	}
}

func TestFormatAge(t *testing.T) {
	tests := []struct {
		d    time.Duration
		want string
	}{
		{30 * time.Second, "30s"},
		{5 * time.Minute, "5m"},
		{90 * time.Minute, "1h"},
		{25 * time.Hour, "1d"},
		{72 * time.Hour, "3d"},
	}
	for _, tt := range tests {
		got := formatAge(tt.d)
		if got != tt.want {
			t.Errorf("formatAge(%v) = %q, want %q", tt.d, got, tt.want)
		}
	}
}

func TestTruncateStr(t *testing.T) {
	if got := truncateStr("short", 10); got != "short" {
		t.Errorf("truncateStr('short', 10) = %q", got)
	}
	if got := truncateStr("a very long title that exceeds limit", 20); got != "a very long title..." {
		t.Errorf("truncateStr long = %q", got)
	}
}

func TestTopModel(t *testing.T) {
	if got := topModel(nil); got != "" {
		t.Errorf("topModel(nil) = %q", got)
	}
	m := map[string]float64{"gpt-4": 0.5, "claude": 1.2}
	if got := topModel(m); got != "claude" {
		t.Errorf("topModel = %q, want 'claude'", got)
	}
}
