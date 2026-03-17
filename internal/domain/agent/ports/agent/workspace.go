package agent

// WorkspaceMode defines how an agent task's workspace is isolated.
type WorkspaceMode string

const (
	WorkspaceModeShared   WorkspaceMode = "shared"
	WorkspaceModeBranch   WorkspaceMode = "branch"
	WorkspaceModeWorktree WorkspaceMode = "worktree"
)

// WorkspaceAllocation is created by WorkspaceManager when a task is dispatched.
type WorkspaceAllocation struct {
	Mode       WorkspaceMode
	WorkingDir string
	Branch     string
	BaseBranch string
	FileScope  []string
}

// MergeStrategy defines how a task's branch is integrated back.
type MergeStrategy string

const (
	MergeStrategyAuto    MergeStrategy = "auto"
	MergeStrategySquash  MergeStrategy = "squash"
	MergeStrategyRebase  MergeStrategy = "rebase"
	MergeStrategyReview  MergeStrategy = "review"
	MergeStrategyResolve MergeStrategy = "resolve"
)

// MergeResult contains the outcome of merging a task's work back.
type MergeResult struct {
	TaskID       string
	Branch       string
	Strategy     MergeStrategy
	Success      bool
	CommitHash   string
	FilesChanged []string
	Conflicts    []string
	DiffSummary  string
	ConflictDiff string
}
