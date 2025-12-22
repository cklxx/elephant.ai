package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestLoadFabricFromRoot(t *testing.T) {
	root := t.TempDir()
	patternDir := filepath.Join(root, "data", "patterns", "summarize")
	if err := os.MkdirAll(patternDir, 0o755); err != nil {
		t.Fatalf("create pattern dir: %v", err)
	}

	system := "# Identity\nYou are an expert summarizer. Provide concise outputs."
	user := "Input: {{content}}"
	if err := os.WriteFile(filepath.Join(patternDir, "system.md"), []byte(system), 0o644); err != nil {
		t.Fatalf("write system: %v", err)
	}
	if err := os.WriteFile(filepath.Join(patternDir, "user.md"), []byte(user), 0o644); err != nil {
		t.Fatalf("write user: %v", err)
	}

	library, err := loadFabricFromRoot(root)
	if err != nil {
		t.Fatalf("load fabric: %v", err)
	}

	skills := library.List()
	if len(skills) != 1 {
		t.Fatalf("expected 1 skill, got %d", len(skills))
	}

	skill := skills[0]
	if skill.Name != "summarize" {
		t.Fatalf("unexpected skill name: %s", skill.Name)
	}
	if skill.Title != "Summarize" {
		t.Fatalf("unexpected title: %s", skill.Title)
	}
	if !strings.Contains(skill.Body, "User Template") {
		t.Fatalf("expected user template section, got: %s", skill.Body)
	}
	if !strings.Contains(skill.Description, "expert summarizer") {
		t.Fatalf("description did not capture summary: %s", skill.Description)
	}
}

func TestDefaultLibraryMergesFabric(t *testing.T) {
	baseDir := t.TempDir()
	t.Setenv(skillsDirEnvVar, baseDir)

	baseSkill := `---
name: base_skill
description: Base description
---
# Base Skill
Content`
	if err := os.WriteFile(filepath.Join(baseDir, "base.md"), []byte(baseSkill), 0o644); err != nil {
		t.Fatalf("write base skill: %v", err)
	}

	fabricRoot := t.TempDir()
	t.Setenv(fabricEnvVar, fabricRoot)
	patternDir := filepath.Join(fabricRoot, "data", "patterns", "plan")
	if err := os.MkdirAll(patternDir, 0o755); err != nil {
		t.Fatalf("create fabric pattern dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(patternDir, "system.md"), []byte("You are a planner."), 0o644); err != nil {
		t.Fatalf("write fabric system: %v", err)
	}
	if err := os.WriteFile(filepath.Join(patternDir, "user.md"), []byte("Plan for {{goal}}"), 0o644); err != nil {
		t.Fatalf("write fabric user: %v", err)
	}

	library, err := DefaultLibrary()
	if err != nil {
		t.Fatalf("default library: %v", err)
	}

	if _, ok := library.Get("base_skill"); !ok {
		t.Fatalf("missing base skill in merged library")
	}
	if _, ok := library.Get("plan"); !ok {
		t.Fatalf("missing fabric skill in merged library")
	}
}
