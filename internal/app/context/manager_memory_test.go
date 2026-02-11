package context

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	storage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/memory"
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

	if err := os.WriteFile(filepath.Join(root, "MEMORY.md"), []byte("# Long-Term\n\nPrefers Go."), 0o644); err != nil {
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

func TestLoadMemorySnapshotBootstrapsSoulAndUserFiles(t *testing.T) {
	root := t.TempDir()
	engine := memory.NewMarkdownEngine(root)
	if err := engine.EnsureSchema(context.Background()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}

	now := time.Now()
	userID := "user-identity"
	if _, err := engine.AppendDaily(context.Background(), userID, memory.DailyEntry{
		Title:     "Today",
		Content:   "Daily content.",
		CreatedAt: now,
	}); err != nil {
		t.Fatalf("AppendDaily: %v", err)
	}

	if err := os.WriteFile(filepath.Join(root, "MEMORY.md"), []byte("# Long-Term\n\nPersistent fact."), 0o644); err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	mgr := &manager{memoryEngine: engine}
	snapshot := mgr.loadMemorySnapshot(context.Background(), &storage.Session{
		ID:       "sess-identity",
		Metadata: map[string]string{"user_id": userID},
	})

	soulPath := filepath.Join(root, "SOUL.md")
	userPath := filepath.Join(root, "USER.md")
	if _, err := os.Stat(soulPath); err != nil {
		t.Fatalf("expected SOUL.md to be created at %s: %v", soulPath, err)
	}
	if _, err := os.Stat(userPath); err != nil {
		t.Fatalf("expected USER.md to be created at %s: %v", userPath, err)
	}

	soulBytes, err := os.ReadFile(soulPath)
	if err != nil {
		t.Fatalf("read SOUL.md: %v", err)
	}
	if !strings.Contains(string(soulBytes), "# Perfect Subordinate — System Prompt") {
		t.Fatalf("expected SOUL.md bootstrap content from default persona voice, got: %s", string(soulBytes))
	}

	if !strings.Contains(snapshot, "Identity (SOUL.md") {
		t.Fatalf("expected SOUL section in snapshot, got: %s", snapshot)
	}
	if !strings.Contains(snapshot, "Identity (USER.md") {
		t.Fatalf("expected USER section in snapshot, got: %s", snapshot)
	}

	soulIdx := strings.Index(snapshot, "Identity (SOUL.md")
	userIdx := strings.Index(snapshot, "Identity (USER.md")
	todayIdx := strings.Index(snapshot, "Daily Log ("+now.Format("2006-01-02")+")")
	memoryIdx := strings.Index(snapshot, "Long-term Memory (MEMORY.md)")
	if soulIdx == -1 || userIdx == -1 || todayIdx == -1 || memoryIdx == -1 {
		t.Fatalf("expected identity/daily/memory sections in snapshot, got: %s", snapshot)
	}
	if !(soulIdx < userIdx && userIdx < todayIdx && todayIdx < memoryIdx) {
		t.Fatalf("expected SOUL -> USER -> daily -> long-term order, got: %s", snapshot)
	}
}

func TestRenderSoulTemplateUsesPersonaVoiceVerbatim(t *testing.T) {
	root := t.TempDir()
	personaDir := filepath.Join(root, "personas")
	if err := os.MkdirAll(personaDir, 0o755); err != nil {
		t.Fatalf("mkdir personas: %v", err)
	}
	if err := os.WriteFile(filepath.Join(personaDir, "default.yaml"), []byte(`id: default
voice: |
  # Custom SOUL

  Keep calm and execute.`), 0o644); err != nil {
		t.Fatalf("write default persona: %v", err)
	}

	mgr := &manager{configRoot: root}
	got := mgr.renderSoulTemplate()
	want := "# Custom SOUL\n\nKeep calm and execute.\n"
	if got != want {
		t.Fatalf("unexpected SOUL template: got %q want %q", got, want)
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
