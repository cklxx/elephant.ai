package prompts

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/agent-infra/sandbox-sdk-go/file"
	"github.com/agent-infra/sandbox-sdk-go/shell"
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

	oldWd, err := os.Getwd()
	if err != nil {
		t.Fatalf("failed to get working directory: %v", err)
	}
	defer func() {
		_ = os.Chdir(oldWd)
	}()

	if err := os.Chdir(tmpDir); err != nil {
		t.Fatalf("failed to change working directory: %v", err)
	}

	prompt, err := loader.GetSystemPrompt("Generate a deck", nil)
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

func TestWithSandboxIgnoresNilImplementation(t *testing.T) {
	var sandbox *stubSandbox
	loader := New(WithSandbox(sandbox))

	if loader.sandbox != nil {
		t.Fatalf("expected sandbox to remain nil when provided implementation is nil")
	}

	if _, err := loader.GetSystemPrompt("goal", nil); err != nil {
		t.Fatalf("GetSystemPrompt should not fail without sandbox: %v", err)
	}
}

type stubSandbox struct{}

func (*stubSandbox) Initialize(context.Context) error { return nil }
func (*stubSandbox) Shell() *shell.Client             { return nil }
func (*stubSandbox) File() *file.Client               { return nil }
