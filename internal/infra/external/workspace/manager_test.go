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

const mergeConflictFixtureFile = "merge-conflict-fixture.txt"

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

func TestMergeConflictPopulatesConflictDiff(t *testing.T) {
	dir := initRepo(t)
	manager := NewManager(dir, logging.Nop())

	// Allocate a branch workspace.
	alloc, err := manager.Allocate(context.Background(), "conflict-task", agent.WorkspaceModeBranch, nil)
	if err != nil {
		t.Fatalf("allocate failed: %v", err)
	}

	// Commit a change on the task branch.
	writeFile(t, filepath.Join(dir, mergeConflictFixtureFile), "branch version\nline2")
	runGit(t, dir, "add", mergeConflictFixtureFile)
	runGit(t, dir, "commit", "-m", "branch change")

	// Switch back to base and commit a conflicting change.
	base := alloc.BaseBranch
	runGit(t, dir, "checkout", base)
	writeFile(t, filepath.Join(dir, mergeConflictFixtureFile), "base version\nline2")
	runGit(t, dir, "add", mergeConflictFixtureFile)
	runGit(t, dir, "commit", "-m", "base change")

	// Merge should fail with conflicts.
	result, err := manager.Merge(context.Background(), alloc, agent.MergeStrategyAuto)
	if err == nil {
		t.Fatal("expected merge conflict error")
	}
	if result == nil {
		t.Fatal("result should be non-nil even on conflict")
	}
	if result.Success {
		t.Error("result.Success should be false on conflict")
	}
	if len(result.Conflicts) == 0 {
		t.Error("result.Conflicts should be populated")
	}
	if result.ConflictDiff == "" {
		t.Error("result.ConflictDiff should be populated on conflict")
	}

	// Best-effort abort: MERGE_HEAD may not exist on all git versions.
	_ = exec.Command("git", "-C", dir, "merge", "--abort").Run()
}

func initRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	runGit(t, dir, "init")
	// Ensure merge creates real conflicts instead of refusing non-fast-forward.
	runGit(t, dir, "config", "merge.ff", "false")
	// Keep merge/checkout behavior independent from runner-global git identity.
	runGit(t, dir, "config", "user.name", "Test")
	runGit(t, dir, "config", "user.email", "test@example.com")
	writeFile(t, filepath.Join(dir, "README.md"), "init")
	writeFile(t, filepath.Join(dir, mergeConflictFixtureFile), "init")
	runGit(t, dir, "add", "README.md", mergeConflictFixtureFile)
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
	// Build a clean env to avoid inheriting GIT_DIR/GIT_WORK_TREE from the
	// parent process (e.g. CI runners), which would cause git to target the
	// project repo instead of the test temp directory.
	env := make([]string, 0, len(os.Environ())+4)
	for _, e := range os.Environ() {
		if strings.HasPrefix(e, "GIT_DIR=") || strings.HasPrefix(e, "GIT_WORK_TREE=") {
			continue
		}
		env = append(env, e)
	}
	env = append(env,
		"GIT_AUTHOR_NAME=Test",
		"GIT_AUTHOR_EMAIL=test@example.com",
		"GIT_COMMITTER_NAME=Test",
		"GIT_COMMITTER_EMAIL=test@example.com",
	)
	cmd.Env = env
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
