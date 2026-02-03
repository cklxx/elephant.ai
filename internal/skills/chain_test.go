package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestResolveChain(t *testing.T) {
	dir := t.TempDir()
	stepOneDir := filepath.Join(dir, "step-one")
	if err := os.Mkdir(stepOneDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	stepTwoDir := filepath.Join(dir, "step-two")
	if err := os.Mkdir(stepTwoDir, 0o755); err != nil {
		t.Fatalf("mkdir skill dir: %v", err)
	}
	step1 := `---
name: step-one
description: first step
---
# Step One
do the first thing
`
	step2 := `---
name: step-two
description: second step
---
# Step Two
do the second thing
`
	if err := os.WriteFile(filepath.Join(stepOneDir, "SKILL.md"), []byte(step1), 0o644); err != nil {
		t.Fatalf("write step1: %v", err)
	}
	if err := os.WriteFile(filepath.Join(stepTwoDir, "SKILL.md"), []byte(step2), 0o644); err != nil {
		t.Fatalf("write step2: %v", err)
	}
	lib, err := Load(dir)
	if err != nil {
		t.Fatalf("load: %v", err)
	}

	out, err := lib.ResolveChain(SkillChain{Steps: []ChainStep{
		{SkillName: "step-one", OutputAs: "draft"},
		{SkillName: "step-two", InputFrom: "draft"},
	}})
	if err != nil {
		t.Fatalf("resolve chain: %v", err)
	}
	if !strings.Contains(out, "Step 1: step-one") || !strings.Contains(out, "Step 2: step-two") {
		t.Fatalf("expected chain output to include both steps, got %q", out)
	}
}

func TestResolveChainMissingSkill(t *testing.T) {
	lib := Library{}
	if _, err := lib.ResolveChain(SkillChain{Steps: []ChainStep{{SkillName: "missing"}}}); err == nil {
		t.Fatal("expected error for missing skill")
	}
}
