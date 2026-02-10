package context

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadBootstrapRecords_GlobalFirst(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home: %v", err)
	}

	globalRoot := filepath.Join(home, ".alex", "memory")
	repoRoot := t.TempDir()
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	if err := os.WriteFile(filepath.Join(globalRoot, "AGENTS.md"), []byte("global content"), 0o644); err != nil {
		t.Fatalf("write global file: %v", err)
	}
	if err := os.WriteFile(filepath.Join(repoRoot, "AGENTS.md"), []byte("workspace content"), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	records := loadBootstrapRecords(repoRoot, []string{"AGENTS.md"}, 20000)
	if len(records) != 1 {
		t.Fatalf("expected one record, got %d", len(records))
	}
	if records[0].Source != "global" {
		t.Fatalf("expected global source, got %s", records[0].Source)
	}
	if !strings.Contains(records[0].Content, "global content") {
		t.Fatalf("expected global content, got %q", records[0].Content)
	}
}

func TestLoadBootstrapRecords_WorkspaceFallbackAndMissing(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	repoRoot := t.TempDir()
	if err := os.WriteFile(filepath.Join(repoRoot, "USER.md"), []byte("workspace user"), 0o644); err != nil {
		t.Fatalf("write workspace file: %v", err)
	}

	records := loadBootstrapRecords(repoRoot, []string{"USER.md", "SOUL.md"}, 20000)
	if len(records) != 2 {
		t.Fatalf("expected 2 records, got %d", len(records))
	}
	if records[0].Source != "workspace" {
		t.Fatalf("expected workspace source, got %s", records[0].Source)
	}
	if records[1].Source != "missing" || !records[1].Missing {
		t.Fatalf("expected missing marker, got %+v", records[1])
	}
}

func TestLoadBootstrapRecords_TruncatesContent(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	home, err := os.UserHomeDir()
	if err != nil {
		t.Fatalf("user home: %v", err)
	}
	globalRoot := filepath.Join(home, ".alex", "memory")
	if err := os.MkdirAll(globalRoot, 0o755); err != nil {
		t.Fatalf("mkdir global: %v", err)
	}
	large := strings.Repeat("a", 64)
	if err := os.WriteFile(filepath.Join(globalRoot, "HEARTBEAT.md"), []byte(large), 0o644); err != nil {
		t.Fatalf("write heartbeat: %v", err)
	}

	records := loadBootstrapRecords(t.TempDir(), []string{"HEARTBEAT.md"}, 16)
	if len(records) != 1 {
		t.Fatalf("expected 1 record, got %d", len(records))
	}
	if !records[0].Truncated {
		t.Fatalf("expected truncation marker")
	}
	if !strings.Contains(records[0].Content, "...[TRUNCATED]") {
		t.Fatalf("expected truncation suffix, got %q", records[0].Content)
	}
}

