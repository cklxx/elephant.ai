package builtin

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestSkillsToolListAndShow(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALEX_SKILLS_DIR", dir)

	content := `---
name: sample_skill
description: Sample description.
---
# Sample Skill

Hello world.
`
	if err := os.WriteFile(filepath.Join(dir, "sample.md"), []byte(content), 0o644); err != nil {
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
}

func TestSkillsToolSearch(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("ALEX_SKILLS_DIR", dir)

	content := `---
name: ppt_deck
description: Presentation playbook.
---
# PPT Deck

Body.
`
	if err := os.WriteFile(filepath.Join(dir, "ppt.md"), []byte(content), 0o644); err != nil {
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
	if !strings.Contains(result.Content, "`ppt_deck`") {
		t.Fatalf("expected search to list match, got %q", result.Content)
	}
}
