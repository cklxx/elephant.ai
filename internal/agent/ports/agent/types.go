package agent

import (
	"time"

	core "alex/internal/agent/ports"
	"alex/internal/agent/ports/llm"
	"alex/internal/agent/ports/storage"
	"alex/internal/agent/ports/tools"
	"alex/internal/workflow"
)

// ServiceBundle contains all dependencies required by the domain engine.
type ServiceBundle struct {
	LLM          llm.StreamingLLMClient
	ToolExecutor tools.ToolRegistry
	ToolLimiter  tools.ToolExecutionLimiter
	Parser       tools.FunctionCallParser
	Context      ContextManager
}

// ExecutionEnvironment contains the prepared state for running a task.
type ExecutionEnvironment struct {
	State        *TaskState
	Services     ServiceBundle
	Session      *storage.Session
	SystemPrompt string
	TaskAnalysis *TaskAnalysis // Pre-analysis result (action name, goal, approach)
}

// TaskAnalysis contains structured task pre-analysis result.
type TaskAnalysis struct {
	Complexity      string
	ActionName      string
	Goal            string
	Approach        string
	SuccessCriteria []string
	TaskBreakdown   []TaskAnalysisStep
	Retrieval       TaskRetrievalPlan
}

// TaskAnalysisStep captures a step in the pre-analysis task plan.
type TaskAnalysisStep struct {
	Description          string
	NeedsExternalContext bool
	Rationale            string
}

// TaskRetrievalPlan captures retrieval specific directives extracted during analysis.
type TaskRetrievalPlan struct {
	ShouldRetrieve bool
	LocalQueries   []string
	SearchQueries  []string
	CrawlURLs      []string
	KnowledgeGaps  []string
	Notes          string
}

// TaskState tracks execution state during ReAct loop.
type TaskState struct {
	SystemPrompt           string
	Messages               []core.Message
	Iterations             int
	TokenCount             int
	ToolResults            []core.ToolResult
	Complete               bool
	FinalAnswer            string
	SessionID   string
	RunID       string
	ParentRunID string
	Attachments            map[string]core.Attachment
	AttachmentIterations   map[string]int
	PendingUserAttachments map[string]core.Attachment
	Important              map[string]core.ImportantNote
	Plans                  []PlanNode
	Beliefs                []Belief
	KnowledgeRefs          []KnowledgeReference
	WorldState             map[string]any
	WorldDiff              map[string]any
	FeedbackSignals        []FeedbackSignal
	LatestGoalPrompt       string
	LatestPlanPrompt       string
}

// AgentConfig exposes the subset of coordinator configuration required by tools.
type AgentConfig struct {
	LLMProvider   string
	LLMModel      string
	MaxTokens     int
	MaxIterations int
	Temperature   float64
	TopP          float64
	StopSequences []string
	AgentPreset   string
	ToolPreset    string
	ToolMode      string
}

// TaskResult represents the result of task execution.
type TaskResult struct {
	Answer       string
	Messages     []core.Message
	Iterations   int
	TokensUsed   int
	StopReason   string
	SessionID   string // The session ID used for this run
	RunID       string // The unique run identifier for this execution
	ParentRunID string // The parent run identifier when invoked as a subtask
	Duration     time.Duration
	Important    map[string]core.ImportantNote
	Workflow     *workflow.WorkflowSnapshot
}

// StreamCallback is called during task execution to stream events.
type StreamCallback func(event StreamEvent)

// StreamEvent represents different types of events during execution.
type StreamEvent struct {
	Type    string // "tool_start", "tool_end", "thought", "error"
	Tool    string
	Args    map[string]any
	Result  string
	Error   error
	Content string
}
