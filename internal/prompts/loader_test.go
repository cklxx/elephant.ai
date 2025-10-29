package prompts

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGetSystemPromptIncludesSkillsInfo(t *testing.T) {
	tmpDir := t.TempDir()
	skillsDir := filepath.Join(tmpDir, "skills")
	if err := os.Mkdir(skillsDir, 0o755); err != nil {
		t.Fatalf("failed to create skills dir: %v", err)
	}

	skillContent := "# Sample Skill\n\n- Outline"
	if err := os.WriteFile(filepath.Join(skillsDir, "sample.md"), []byte(skillContent), 0o644); err != nil {
		t.Fatalf("failed to write skill file: %v", err)
	}

	loader := New()
	prompt, err := loader.GetSystemPrompt(tmpDir, "Generate a deck", nil)
	if err != nil {
		t.Fatalf("GetSystemPrompt returned error: %v", err)
	}

	if !strings.Contains(prompt, "Custom Skills") {
		t.Fatalf("expected skills section in prompt, got: %s", prompt)
	}

	if !strings.Contains(prompt, "Sample Skill") {
		t.Fatalf("expected skill title in prompt, got: %s", prompt)
	}

	if !strings.Contains(prompt, "file_read(\"skills/sample.md\")") {
		t.Fatalf("expected skill access guidance in prompt, got: %s", prompt)
	}
}
