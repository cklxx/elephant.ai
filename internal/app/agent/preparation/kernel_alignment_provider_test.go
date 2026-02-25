package preparation

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestKernelAlignmentContextProvider_LoadsSoulUserGoal(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	memoryDir := filepath.Join(home, ".alex", "memory")
	kernelDir := filepath.Join(home, ".alex", "kernel", "default")
	if err := os.MkdirAll(memoryDir, 0o755); err != nil {
		t.Fatalf("mkdir memory dir: %v", err)
	}
	if err := os.MkdirAll(kernelDir, 0o755); err != nil {
		t.Fatalf("mkdir kernel dir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "SOUL.md"), []byte("soul-values"), 0o644); err != nil {
		t.Fatalf("write SOUL.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(memoryDir, "USER.md"), []byte("serve-user"), 0o644); err != nil {
		t.Fatalf("write USER.md: %v", err)
	}
	if err := os.WriteFile(filepath.Join(kernelDir, "GOAL.md"), []byte("finish real objective"), 0o644); err != nil {
		t.Fatalf("write GOAL.md: %v", err)
	}

	provider := NewKernelAlignmentContextProvider(KernelAlignmentContextConfig{KernelID: "default"})
	got := provider()
	for _, snippet := range []string{
		"Service user: cklxx",
		"finish real objective",
		"soul-values",
		"serve-user",
		"bg_dispatch",
		"codex",
		"claude_code",
	} {
		if !strings.Contains(got, snippet) {
			t.Fatalf("expected output to include %q, got: %s", snippet, got)
		}
	}
}
