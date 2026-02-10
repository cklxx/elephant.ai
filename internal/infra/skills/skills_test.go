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
	skillDir := filepath.Join(dir, "video-production")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := `---
name: video-production
description: Create a video from brief to export.
---
# Video Production

Some body text.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	skill, ok := lib.Get("video-production")
	if !ok {
		t.Fatalf("expected skill to be present")
	}
	if skill.Name != "video-production" {
		t.Fatalf("expected name video-production, got %q", skill.Name)
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
name: pdf-processing
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

	skill, ok := lib.Get("pdf-processing")
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
	skillDir := filepath.Join(dir, "bad-skill")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := `# Untitled

No front matter here.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	if _, err := Load(dir); err == nil {
		t.Fatalf("expected error for missing front matter")
	}
}

func TestLoadIgnoresFlatMarkdownFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	content := `---
name: flat-skill
description: Flat file should be ignored.
---
# Flat Skill

Body.
`
	if err := os.WriteFile(filepath.Join(dir, "flat.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(lib.List()) != 0 {
		t.Fatalf("expected no skills loaded from flat files, got %d", len(lib.List()))
	}
}

func TestIndexMarkdownIncludesSkillList(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "ppt-deck")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := `---
name: ppt-deck
description: Build a PPT deck playbook.
---
# PPT Deck

Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
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
	if !strings.Contains(index, "`ppt-deck`") {
		t.Fatalf("expected skill name in index, got %q", index)
	}
}

func TestLoadParsesRequiresToolsAndDetectsRunScript(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "config-management")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `---
name: config-management
description: Manage agent config.
requires_tools: [bash]
max_tokens: 200
---
# config-management

Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	// Create run.py alongside SKILL.md
	if err := os.WriteFile(filepath.Join(skillDir, "run.py"), []byte("# stub"), 0o644); err != nil {
		t.Fatalf("write run.py: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	skill, ok := lib.Get("config-management")
	if !ok {
		t.Fatal("expected skill to be present")
	}
	if len(skill.RequiresTools) != 1 || skill.RequiresTools[0] != "bash" {
		t.Fatalf("expected requires_tools=[bash], got %v", skill.RequiresTools)
	}
	if !skill.HasRunScript {
		t.Fatal("expected HasRunScript=true when run.py exists")
	}
	if skill.MaxTokens != 200 {
		t.Fatalf("expected max_tokens=200, got %d", skill.MaxTokens)
	}
}

func TestLoadDetectsNoRunScript(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "guide-only")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	content := `---
name: guide-only
description: A Markdown-only skill.
---
# Guide Only

Just instructions.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	// No run.py

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	skill, ok := lib.Get("guide-only")
	if !ok {
		t.Fatal("expected skill to be present")
	}
	if skill.HasRunScript {
		t.Fatal("expected HasRunScript=false when no run.py")
	}
}

func TestIndexMarkdownShowsPyMarkerForPythonSkills(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	// Python skill (with run.py)
	pyDir := filepath.Join(dir, "timer-management")
	if err := os.Mkdir(pyDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	pyContent := `---
name: timer-management
description: Manage timers.
---
# timer-management

Body.
`
	if err := os.WriteFile(filepath.Join(pyDir, "SKILL.md"), []byte(pyContent), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(pyDir, "run.py"), []byte("# stub"), 0o644); err != nil {
		t.Fatalf("write run.py: %v", err)
	}

	// Markdown-only skill (no run.py)
	mdDir := filepath.Join(dir, "guide")
	if err := os.Mkdir(mdDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	mdContent := `---
name: guide
description: A guide skill.
---
# guide

Body.
`
	if err := os.WriteFile(filepath.Join(mdDir, "SKILL.md"), []byte(mdContent), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	index := IndexMarkdown(lib)
	if !strings.Contains(index, "`timer-management` [py]") {
		t.Fatalf("expected [py] marker for Python skill, got %q", index)
	}
	if strings.Contains(index, "`guide` [py]") {
		t.Fatalf("expected no [py] marker for Markdown-only skill, got %q", index)
	}
}

func TestAvailableSkillsXMLIncludesPythonType(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "my-skill")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	content := `---
name: my-skill
description: A Python skill.
---
# my-skill

Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}
	if err := os.WriteFile(filepath.Join(skillDir, "run.py"), []byte("# stub"), 0o644); err != nil {
		t.Fatalf("write run.py: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	xml := AvailableSkillsXML(lib)
	if !strings.Contains(xml, "<type>python</type>") {
		t.Fatalf("expected <type>python</type> for Python skill, got %q", xml)
	}
	if !strings.Contains(xml, "<exec>python3 skills/my-skill/run.py") {
		t.Fatalf("expected <exec> element for Python skill, got %q", xml)
	}
}

func TestAvailableSkillsXMLIncludesMetadata(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	skillDir := filepath.Join(dir, "alpha")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	content := `---
name: alpha
description: A & B workflow.
---
# Alpha

Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	xml := AvailableSkillsXML(lib)
	if !strings.Contains(xml, "<available_skills>") {
		t.Fatalf("expected available skills wrapper, got %q", xml)
	}
	if !strings.Contains(xml, "<name>alpha</name>") {
		t.Fatalf("expected skill name in xml, got %q", xml)
	}
	if !strings.Contains(xml, "<description>A &amp; B workflow.</description>") {
		t.Fatalf("expected escaped description in xml, got %q", xml)
	}
	if !strings.Contains(xml, "SKILL.md") {
		t.Fatalf("expected location to include SKILL.md, got %q", xml)
	}
}
