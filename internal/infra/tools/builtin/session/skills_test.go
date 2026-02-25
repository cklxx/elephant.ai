package session

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestSkillsToolListAndShow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALEX_SKILLS_DIR", dir)
	skillDir := filepath.Join(dir, "sample_skill")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
name: sample_skill
description: Sample description.
capabilities: [lark_chat]
governance_level: medium
activation_mode: auto
---
# Sample Skill

Hello world.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	tool := NewSkills()

	listResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"action": "list",
		},
	})
	if err != nil {
		t.Fatalf("execute list: %v", err)
	}
	if listResult.Error != nil {
		t.Fatalf("list returned error: %v", listResult.Error)
	}
	if !strings.Contains(listResult.Content, "`sample_skill`") {
		t.Fatalf("expected list to include skill name, got %q", listResult.Content)
	}

	showResult, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-2",
		Arguments: map[string]any{
			"action": "show",
			"name":   "sample_skill",
		},
	})
	if err != nil {
		t.Fatalf("execute show: %v", err)
	}
	if showResult.Error != nil {
		t.Fatalf("show returned error: %v", showResult.Error)
	}
	if !strings.Contains(showResult.Content, "Hello world") {
		t.Fatalf("expected show to return body content, got %q", showResult.Content)
	}
	if showResult.Metadata["governance_level"] != "medium" {
		t.Fatalf("expected governance metadata, got %+v", showResult.Metadata)
	}
	if showResult.Metadata["activation_mode"] != "auto" {
		t.Fatalf("expected activation metadata, got %+v", showResult.Metadata)
	}
	caps, ok := showResult.Metadata["capabilities"].([]string)
	if !ok || len(caps) != 1 || caps[0] != "lark_chat" {
		t.Fatalf("expected capabilities metadata, got %+v", showResult.Metadata["capabilities"])
	}
}

func TestSkillsToolSearch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALEX_SKILLS_DIR", dir)
	skillDir := filepath.Join(dir, "ppt-deck")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
name: ppt-deck
description: Presentation playbook.
---
# PPT Deck

Body.
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	tool := NewSkills()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-3",
		Arguments: map[string]any{
			"action": "search",
			"query":  "present",
		},
	})
	if err != nil {
		t.Fatalf("execute search: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("search returned error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "`ppt-deck`") {
		t.Fatalf("expected search to list match, got %q", result.Content)
	}
}

func TestSkillsToolSupportsSkillDirectories(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALEX_SKILLS_DIR", dir)

	skillDir := filepath.Join(dir, "data-analysis")
	if err := os.Mkdir(skillDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}

	content := `---
name: data-analysis
description: Analyze datasets and summarize insights.
---
# Data Analysis

Steps...
`
	if err := os.WriteFile(filepath.Join(skillDir, "SKILL.md"), []byte(content), 0o644); err != nil {
		t.Fatalf("write skill: %v", err)
	}

	tool := NewSkills()
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-4",
		Arguments: map[string]any{
			"action": "list",
		},
	})
	if err != nil {
		t.Fatalf("execute list: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("list returned error: %v", result.Error)
	}
	if !strings.Contains(result.Content, "data-analysis") {
		t.Fatalf("expected list to include skill name, got %q", result.Content)
	}
}
