package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadParsesFrontMatterAndTitle(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `---
name: video_production
description: Create a video from brief to export.
---
# Video Production

Some body text.
`
	if err := os.WriteFile(filepath.Join(dir, "video.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	skill, ok := lib.Get("video_production")
	if !ok {
		t.Fatalf("expected skill to be present")
	}
	if skill.Name != "video_production" {
		t.Fatalf("expected name video_production, got %q", skill.Name)
	}
	if skill.Description == "" {
		t.Fatalf("expected description to be populated")
	}
	if skill.Title != "Video Production" {
		t.Fatalf("expected title %q, got %q", "Video Production", skill.Title)
	}
	if !strings.Contains(skill.Body, "Some body text") {
		t.Fatalf("expected body text to be preserved")
	}
}

func TestLoadSupportsSkillDirectories(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "pdf-processing")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
name: pdf_processing
description: Extract text and tables from PDFs.
---
# PDF Processing

Steps...
`
	sourcePath := filepath.Join(skillDir, "SKILL.md")
	if err := os.WriteFile(sourcePath, []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	skill, ok := lib.Get("pdf_processing")
	if !ok {
		t.Fatalf("expected skill to be present")
	}
	if skill.SourcePath != sourcePath {
		t.Fatalf("expected source path %s, got %s", sourcePath, skill.SourcePath)
	}
}

func TestLoadRejectsMissingFrontMatter(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `# Untitled

No front matter here.
`
	if err := os.WriteFile(filepath.Join(dir, "bad.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatalf("expected error for missing front matter")
	}
}

func TestIndexMarkdownIncludesSkillList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `---
name: ppt_deck
description: Build a PPT deck playbook.
---
# PPT Deck

Body.
`
	if err := os.WriteFile(filepath.Join(dir, "ppt.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	index := IndexMarkdown(lib)
	if !strings.Contains(index, "Skills Catalog") {
		t.Fatalf("expected header in index, got %q", index)
	}
	if !strings.Contains(index, "`ppt_deck`") {
		t.Fatalf("expected skill name in index, got %q", index)
	}
}
