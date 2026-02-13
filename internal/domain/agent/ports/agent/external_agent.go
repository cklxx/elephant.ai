package agent

import (
	"context"
	"time"
)

// ExternalAgentExecutor abstracts execution of external code agent processes
// (e.g., Claude Code CLI, Cursor, custom agents).
type ExternalAgentExecutor interface {
	// Execute runs an external agent with the given prompt and returns the result.
	Execute(ctx context.Context, req ExternalAgentRequest) (*ExternalAgentResult, error)
	// SupportedTypes returns the agent types this executor handles.
	SupportedTypes() []string
}

// InputRequestType defines the kind of input requested by an external agent.
type InputRequestType string

const (
	InputRequestPermission    InputRequestType = "permission"
	InputRequestClarification InputRequestType = "clarification"
)

// ExternalAgentRequest contains the parameters for an external agent invocation.
type ExternalAgentRequest struct {
	TaskID      string
	Prompt      string
	AgentType   string
	WorkingDir  string
	Config      map[string]string
	SessionID   string
	CausationID string
	// ExecutionMode controls the high-level execution intent:
	// "execute" (default) or "plan" (plan-only mode).
	ExecutionMode string
	// AutonomyLevel controls approval/sandbox behavior policy:
	// "controlled" (default), "semi", or "full".
	AutonomyLevel string

	// OnProgress is called whenever the external agent reports progress.
	OnProgress func(ExternalAgentProgress)

	// OnBridgeStarted is called when a detached bridge process launches.
	// The caller can use this to persist bridge metadata for resilience.
	// The callback receives opaque bridge-specific info (e.g. PID, output file).
	OnBridgeStarted func(info any)
}

// ExternalAgentResult contains the output from an external agent execution.
type ExternalAgentResult struct {
	Answer     string
	Iterations int
	TokensUsed int
	Error      string
	Metadata   map[string]any
}

// InputRequest represents a request from an external agent for user/main-agent input.
type InputRequest struct {
	TaskID    string
	AgentType string
	RequestID string
	Type      InputRequestType
	Summary   string
	ToolCall  *InputToolCall
	Options   []InputOption
	Deadline  time.Time
}

type InputToolCall struct {
	Name      string
	Arguments map[string]any
	FilePaths []string
}

type InputOption struct {
	ID          string
	Label       string
	Description string
}

// InputResponse is the main agent's reply to an InputRequest.
type InputResponse struct {
	TaskID    string
	RequestID string
	Approved  bool
	OptionID  string
	Text      string
}

// ExternalAgentProgress is a real-time snapshot of what an external agent is doing.
type ExternalAgentProgress struct {
	Iteration    int
	MaxIter      int
	TokensUsed   int
	CostUSD      float64
	CurrentTool  string
	CurrentArgs  string
	FilesTouched []string
	LastActivity time.Time
}

// InputRequestSummary is a lightweight view of a pending input request.
type InputRequestSummary struct {
	RequestID string
	Type      InputRequestType
	Summary   string
	Since     time.Time
}

// WorkspaceMode defines how an external agent's workspace is isolated.
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

// MergeStrategy defines how an agent's branch is integrated back.
type MergeStrategy string

const (
	MergeStrategyAuto   MergeStrategy = "auto"
	MergeStrategySquash MergeStrategy = "squash"
	MergeStrategyRebase MergeStrategy = "rebase"
	MergeStrategyReview MergeStrategy = "review"
)

// MergeResult contains the outcome of merging an agent's work back.
type MergeResult struct {
	TaskID       string
	Branch       string
	Strategy     MergeStrategy
	Success      bool
	CommitHash   string
	FilesChanged []string
	Conflicts    []string
	DiffSummary  string
}

// TaskDependency defines ordering between tasks.
type TaskDependency struct {
	DependsOn      []string
	InheritContext bool
}

// InteractiveExternalExecutor extends ExternalAgentExecutor with input handling.
type InteractiveExternalExecutor interface {
	ExternalAgentExecutor
	InputRequests() <-chan InputRequest
	Reply(ctx context.Context, resp InputResponse) error
}
