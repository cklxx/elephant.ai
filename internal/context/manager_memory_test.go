package context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storage "alex/internal/agent/ports/storage"
	"alex/internal/memory"
)

func TestLoadMemorySnapshotIncludesLongTermAndDaily(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	userID := "user-1"
	now := time.Now()
	_, err := engine.AppendDaily(context.Background(), userID, memory.DailyEntry{
		Title:     "Today",
		Content:   "Discussed API approach.",
		CreatedAt: now,
	})
	if err != nil {
		t.Fatalf("AppendDaily today: %v", err)
	}
	_, err = engine.AppendDaily(context.Background(), userID, memory.DailyEntry{
		Title:     "Yesterday",
		Content:   "Reviewed architecture notes.",
		CreatedAt: now.AddDate(0, 0, -1),
	})
	if err != nil {
		t.Fatalf("AppendDaily yesterday: %v", err)
	}

	userDir := filepath.Join(root, userID)
	if err := os.MkdirAll(userDir, 0o755); err != nil {
		t.Fatalf("mkdir user dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(userDir, "MEMORY.md"), []byte("# Long-Term\n\nPrefers Go."), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	mgr := &manager{memoryEngine: engine}
	session := &storage.Session{Metadata: map[string]string{"user_id": userID}}
	snapshot := mgr.loadMemorySnapshot(context.Background(), session)

	if !strings.Contains(snapshot, "Long-term Memory") {
		t.Fatalf("expected long-term section, got: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Prefers Go") {
		t.Fatalf("expected long-term content, got: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Discussed API approach") {
		t.Fatalf("expected daily content, got: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Reviewed architecture notes") {
		t.Fatalf("expected yesterday content, got: %s", snapshot)
	}
}

func TestLoadMemorySnapshotRespectsGate(t *testing.T) {
	engine := memory.NewMarkdownEngine(t.TempDir())
	mgr := &manager{
		memoryEngine: engine,
		memoryGate: func(context.Context) bool {
			return false
		},
	}

	snapshot := mgr.loadMemorySnapshot(context.Background(), &storage.Session{})
	if snapshot != "" {
		t.Fatalf("expected empty snapshot when gated, got: %s", snapshot)
	}
}
