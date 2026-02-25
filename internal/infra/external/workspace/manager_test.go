package workspace

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/shared/logging"
)

func TestAllocateShared(t *testing.T) {
	dir := initRepo(t)
	manager := NewManager(dir, logging.Nop())

	base := currentBranch(t, dir)
	alloc, err := manager.Allocate(context.Background(), "task1", agent.WorkspaceModeShared, nil)
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	if alloc.WorkingDir != dir {
		t.Fatalf("expected working dir %s, got %s", dir, alloc.WorkingDir)
	}
	if alloc.Branch != base {
		t.Fatalf("expected branch %s, got %s", base, alloc.Branch)
	}
}

func TestAllocateWorktreeAndCleanup(t *testing.T) {
	dir := initRepo(t)
	manager := NewManager(dir, logging.Nop())

	alloc, err := manager.Allocate(context.Background(), "fix-auth", agent.WorkspaceModeWorktree, nil)
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	if alloc.WorkingDir == "" {
		t.Fatalf("expected working dir set")
	}
	if _, err := os.Stat(alloc.WorkingDir); err != nil {
		t.Fatalf("expected worktree to exist: %v", err)
	}
	if err := manager.Cleanup(context.Background(), alloc, true); err != nil {
		t.Fatalf("cleanup failed: %v", err)
	}
	if _, err := os.Stat(alloc.WorkingDir); err == nil {
		t.Fatalf("expected worktree to be removed")
	}
}

func TestValidateFileScope(t *testing.T) {
	dir := initRepo(t)
	manager := NewManager(dir, logging.Nop())

	alloc, err := manager.Allocate(context.Background(), "scope-task", agent.WorkspaceModeBranch, []string{"allowed/"})
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}
	writeFile(t, filepath.Join(dir, "other.txt"), "oops")
	runGit(t, dir, "add", "other.txt")
	runGit(t, dir, "commit", "-m", "update other")

	out, err := manager.ValidateFileScope(context.Background(), alloc)
	if err != nil {
		t.Fatalf("validate failed: %v", err)
	}
	if len(out) == 0 {
		t.Fatalf("expected out of scope files")
	}
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	writeFile(t, filepath.Join(dir, "README.md"), "init")
	runGit(t, dir, "add", "README.md")
	runGit(t, dir, "commit", "-m", "init")
	return dir
}

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
}

func runGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s failed: %v (%s)", strings.Join(args, " "), err, string(out))
	}
}

func currentBranch(t *testing.T, dir string) string {
	t.Helper()
	cmd := exec.Command("git", "rev-parse", "--abbrev-ref", "HEAD")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		t.Fatalf("git rev-parse failed: %v", err)
	}
	return strings.TrimSpace(string(out))
}
