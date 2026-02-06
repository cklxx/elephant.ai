package workspace

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	agent "alex/internal/agent/ports/agent"
	"alex/internal/logging"
)

// Manager handles workspace allocation and cleanup for external agent tasks.
type Manager struct {
	projectDir  string
	worktreeDir string
	logger      logging.Logger
	mu          sync.Mutex
}

func NewManager(projectDir string, logger logging.Logger) *Manager {
	projectDir = strings.TrimSpace(projectDir)
	worktreeDir := filepath.Join(projectDir, ".elephant", "worktrees")
	return &Manager{
		projectDir:  projectDir,
		worktreeDir: worktreeDir,
		logger:      logging.OrNop(logger),
	}
}

// Allocate creates an isolated workspace for a task based on the requested mode.
func (m *Manager) Allocate(ctx context.Context, taskID string, mode agent.WorkspaceMode, fileScope []string) (*agent.WorkspaceAllocation, error) {
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return nil, fmt.Errorf("taskID is required")
	}

	baseBranch, err := m.currentBranch(ctx)
	if err != nil {
		return nil, err
	}

	alloc := &agent.WorkspaceAllocation{
		Mode:       mode,
		BaseBranch: baseBranch,
		FileScope:  append([]string(nil), fileScope...),
	}

	switch mode {
	case agent.WorkspaceModeShared:
		alloc.WorkingDir = m.projectDir
		alloc.Branch = baseBranch
		return alloc, nil
	case agent.WorkspaceModeBranch:
		branch := branchName(taskID)
		m.mu.Lock()
		defer m.mu.Unlock()
		if err := m.git(ctx, "checkout", "-b", branch); err != nil {
			return nil, err
		}
		alloc.WorkingDir = m.projectDir
		alloc.Branch = branch
		return alloc, nil
	case agent.WorkspaceModeWorktree:
		branch := branchName(taskID)
		m.mu.Lock()
		defer m.mu.Unlock()
		if err := os.MkdirAll(m.worktreeDir, 0o755); err != nil {
			return nil, fmt.Errorf("create worktree dir: %w", err)
		}
		worktreePath := filepath.Join(m.worktreeDir, taskID)
		if err := m.git(ctx, "worktree", "add", worktreePath, "-b", branch); err != nil {
			return nil, err
		}
		alloc.WorkingDir = worktreePath
		alloc.Branch = branch
		return alloc, nil
	default:
		return nil, fmt.Errorf("unsupported workspace mode: %s", mode)
	}
}

// Merge integrates an agent's branch back into the base branch.
func (m *Manager) Merge(ctx context.Context, alloc *agent.WorkspaceAllocation, strategy agent.MergeStrategy) (*agent.MergeResult, error) {
	if alloc == nil {
		return nil, fmt.Errorf("workspace allocation is required")
	}
	taskBranch := alloc.Branch
	if taskBranch == "" {
		return nil, fmt.Errorf("workspace branch is empty")
	}
	if strategy == "" {
		strategy = agent.MergeStrategyAuto
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	if err := m.git(ctx, "checkout", alloc.BaseBranch); err != nil {
		return nil, err
	}

	result := &agent.MergeResult{
		TaskID:   strings.TrimPrefix(taskBranch, "elephant/"),
		Branch:   taskBranch,
		Strategy: strategy,
	}

	switch strategy {
	case agent.MergeStrategyReview:
		files, _ := m.gitOutput(ctx, "diff", "--name-only", alloc.BaseBranch+".."+taskBranch)
		result.FilesChanged = splitLines(files)
		stats, _ := m.gitOutput(ctx, "diff", "--stat", alloc.BaseBranch+".."+taskBranch)
		result.DiffSummary = strings.TrimSpace(stats)
		result.Success = true
		return result, nil
	case agent.MergeStrategySquash:
		if err := m.git(ctx, "merge", "--squash", taskBranch); err != nil {
			result.Success = false
			result.Conflicts = m.mergeConflicts(ctx)
			return result, err
		}
		if err := m.git(ctx, "commit", "-m", fmt.Sprintf("Merge external task %s", taskBranch)); err != nil {
			return nil, err
		}
	case agent.MergeStrategyRebase:
		if err := m.git(ctx, "checkout", taskBranch); err != nil {
			return nil, err
		}
		if err := m.git(ctx, "rebase", alloc.BaseBranch); err != nil {
			return nil, err
		}
		if err := m.git(ctx, "checkout", alloc.BaseBranch); err != nil {
			return nil, err
		}
		if err := m.git(ctx, "merge", taskBranch); err != nil {
			result.Success = false
			result.Conflicts = m.mergeConflicts(ctx)
			return result, err
		}
	default:
		if err := m.git(ctx, "merge", "--no-edit", taskBranch); err != nil {
			result.Success = false
			result.Conflicts = m.mergeConflicts(ctx)
			return result, err
		}
	}

	result.Success = true
	result.CommitHash = strings.TrimSpace(m.gitOutputOrEmpty(ctx, "rev-parse", "HEAD"))
	result.FilesChanged = splitLines(m.gitOutputOrEmpty(ctx, "diff", "--name-only", "HEAD~1..HEAD"))
	result.DiffSummary = strings.TrimSpace(m.gitOutputOrEmpty(ctx, "diff", "--stat", "HEAD~1..HEAD"))
	return result, nil
}

// Cleanup removes a worktree and optionally deletes the branch.
func (m *Manager) Cleanup(ctx context.Context, alloc *agent.WorkspaceAllocation, deleteBranch bool) error {
	if alloc == nil {
		return nil
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if alloc.Mode == agent.WorkspaceModeWorktree && alloc.WorkingDir != "" {
		_ = m.git(ctx, "worktree", "remove", "--force", alloc.WorkingDir)
	}
	if deleteBranch && alloc.Branch != "" {
		_ = m.git(ctx, "branch", "-D", alloc.Branch)
	}
	return nil
}

// ValidateFileScope checks if a task's actual file changes match its declared scope.
func (m *Manager) ValidateFileScope(ctx context.Context, alloc *agent.WorkspaceAllocation) ([]string, error) {
	if alloc == nil || alloc.Branch == "" || len(alloc.FileScope) == 0 {
		return nil, nil
	}
	filesRaw, err := m.gitOutput(ctx, "diff", "--name-only", alloc.BaseBranch+".."+alloc.Branch)
	if err != nil {
		return nil, err
	}
	files := splitLines(filesRaw)
	var outOfScope []string
	for _, file := range files {
		if !withinScope(file, alloc.FileScope) {
			outOfScope = append(outOfScope, file)
		}
	}
	return outOfScope, nil
}

// CheckScopeOverlap detects overlap between new scope and running task scopes.
func (m *Manager) CheckScopeOverlap(newScope []string, running []agent.BackgroundTaskSummary) []ScopeConflict {
	if len(newScope) == 0 {
		return nil
	}
	var conflicts []ScopeConflict
	for _, task := range running {
		if len(task.FileScope) == 0 {
			continue
		}
		overlap := overlapPaths(newScope, task.FileScope)
		if len(overlap) > 0 {
			conflicts = append(conflicts, ScopeConflict{
				TaskID:       task.ID,
				OverlapPaths: overlap,
			})
		}
	}
	return conflicts
}

// ScopeConflict represents overlapping file scope between tasks.
type ScopeConflict struct {
	TaskID       string
	OverlapPaths []string
}

func (m *Manager) currentBranch(ctx context.Context) (string, error) {
	out, err := m.gitOutput(ctx, "rev-parse", "--abbrev-ref", "HEAD")
	if err != nil {
		return "", err
	}
	branch := strings.TrimSpace(out)
	if branch == "" {
		return "", fmt.Errorf("unable to resolve current branch")
	}
	return branch, nil
}

func (m *Manager) git(ctx context.Context, args ...string) error {
	_, err := m.gitOutput(ctx, args...)
	return err
}

func (m *Manager) gitOutput(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = m.projectDir
	var stderr bytes.Buffer
	cmd.Stderr = &stderr
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("git %s: %w: %s", strings.Join(args, " "), err, stderr.String())
	}
	return string(out), nil
}

func (m *Manager) gitOutputOrEmpty(ctx context.Context, args ...string) string {
	out, err := m.gitOutput(ctx, args...)
	if err != nil {
		return ""
	}
	return out
}

func (m *Manager) mergeConflicts(ctx context.Context) []string {
	out, err := m.gitOutput(ctx, "diff", "--name-only", "--diff-filter=U")
	if err != nil {
		return nil
	}
	return splitLines(out)
}

func branchName(taskID string) string {
	sanitized := strings.TrimSpace(taskID)
	sanitized = strings.ReplaceAll(sanitized, " ", "-")
	sanitized = strings.ReplaceAll(sanitized, "/", "-")
	sanitized = strings.ReplaceAll(sanitized, "\\", "-")
	if sanitized == "" {
		sanitized = "task"
	}
	return fmt.Sprintf("elephant/%s", sanitized)
}

func splitLines(raw string) []string {
	lines := strings.Split(strings.TrimSpace(raw), "\n")
	var out []string
	for _, line := range lines {
		if trimmed := strings.TrimSpace(line); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func withinScope(path string, scope []string) bool {
	for _, prefix := range scope {
		prefix = strings.TrimSpace(prefix)
		if prefix == "" {
			continue
		}
		if strings.HasPrefix(path, prefix) {
			return true
		}
	}
	return false
}

func overlapPaths(a, b []string) []string {
	var overlap []string
	for _, pathA := range a {
		for _, pathB := range b {
			if strings.HasPrefix(pathA, pathB) || strings.HasPrefix(pathB, pathA) {
				overlap = append(overlap, pathA)
				break
			}
		}
	}
	return dedupe(overlap)
}

func dedupe(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}
