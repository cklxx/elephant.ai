package react

import (
	"context"
	"sync"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	materialports "alex/internal/domain/materials/ports"
)

// ReactEngine orchestrates the Think-Act-Observe cycle
type ReactEngine struct {
	maxIterations       int
	stopReasons         []string
	logger              agent.Logger
	clock               agent.Clock
	idGenerator         agent.IDGenerator
	idContextReader     agent.IDContextReader
	latencyReporter     agent.LatencyReporterFunc
	jsonCodec           agent.JSONMarshalFunc
	goRunner            agent.GoRunnerFunc
	workingDirResolver  agent.WorkingDirResolverFunc
	workspaceMgrFactory agent.WorkspaceManagerFactoryFunc
	eventListener       EventListener // Optional event listener for TUI
	completion          completionConfig
	finalAnswerReview   FinalAnswerReviewConfig
	attachmentMigrator  materialports.Migrator
	attachmentPersister ports.AttachmentPersister // Optional: eagerly persists inline attachment payloads
	checkpointStore     CheckpointStore           // Optional: persists execution checkpoints
	workflow            WorkflowTracker
	seq                 domain.SeqCounter // Monotonic event sequence per run
	iterationHook       agent.IterationHook
	sessionPersister    agent.SessionPersister // Optional: async save session after each iteration

	// Background task support: executor closure for internal subagent delegation.
	backgroundExecutor func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error)
	// Optional shared background manager (e.g., per-session pool).
	backgroundManager *BackgroundTaskManager
	// Optional external agent executor for non-internal agent types.
	externalExecutor agent.ExternalAgentExecutor
	// Pre-configured team definitions for team_dispatch tool.
	teamDefinitions []agent.TeamDefinition
}

type workflowRecorder struct {
	tracker WorkflowTracker
}

type reactWorkflow struct {
	recorder *workflowRecorder
}

type toolCallBatch struct {
	engine               *ReactEngine
	ctx                  context.Context
	state                *TaskState
	iteration            int
	registry             tools.ToolRegistry
	limiter              tools.ToolExecutionLimiter
	tracker              *reactWorkflow
	attachments          map[string]ports.Attachment
	attachmentIterations map[string]int
	subagentSnapshots    []*agent.TaskState
	calls                []ToolCall
	callNodes            []string
	results              []ToolResult
	attachmentsMu        sync.RWMutex
	stateMu              sync.Mutex
}

type completionConfig struct {
	temperature   float64
	maxTokens     int
	topP          float64
	stopSequences []string
}

// WorkflowTracker captures the minimal workflow operations the ReAct engine
// needs for debugging and event emission. Implementations are provided by the
// application layer (e.g., agentWorkflow) to avoid domain-level coupling to a
// specific workflow implementation.
type WorkflowTracker interface {
	EnsureNode(id string, input any)
	StartNode(id string)
	CompleteNodeSuccess(id string, output any)
	CompleteNodeFailure(id string, err error)
}

// CompletionDefaults defines optional overrides for LLM completion behaviour.
type CompletionDefaults struct {
	Temperature   *float64
	MaxTokens     *int
	TopP          *float64
	StopSequences []string
}

// ReactEngineConfig captures the dependencies required to construct a ReactEngine.
type ReactEngineConfig struct {
	MaxIterations       int
	StopReasons         []string
	Logger              agent.Logger
	Clock               agent.Clock
	IDGenerator         agent.IDGenerator
	IDContextReader     agent.IDContextReader
	LatencyReporter     agent.LatencyReporterFunc
	JSONCodec           agent.JSONMarshalFunc
	GoRunner            agent.GoRunnerFunc
	WorkingDirResolver  agent.WorkingDirResolverFunc
	WorkspaceMgrFactory agent.WorkspaceManagerFactoryFunc
	EventListener       EventListener
	CompletionDefaults  CompletionDefaults
	FinalAnswerReview   FinalAnswerReviewConfig
	AttachmentMigrator  materialports.Migrator
	AttachmentPersister ports.AttachmentPersister // Optional: eagerly persists attachment payloads to a durable store.
	CheckpointStore     CheckpointStore           // Optional: persists execution checkpoints.
	Workflow            WorkflowTracker
	IterationHook       agent.IterationHook
	SessionPersister    agent.SessionPersister // Optional: async save session after each iteration.

	// BackgroundExecutor is a closure that delegates to coordinator.ExecuteTask
	// for background subagent tasks. When nil, bg_dispatch is unavailable.
	BackgroundExecutor func(ctx context.Context, prompt, sessionID string,
		listener agent.EventListener) (*agent.TaskResult, error)
	// BackgroundManager optionally supplies a shared background task manager.
	BackgroundManager *BackgroundTaskManager
	// ExternalExecutor handles external code agents (e.g., Claude Code CLI).
	// Optional; when nil only "internal" agent type is supported.
	ExternalExecutor agent.ExternalAgentExecutor
	// TeamDefinitions are pre-configured agent teams available to the team_dispatch tool.
	TeamDefinitions []agent.TeamDefinition
}

type FinalAnswerReviewConfig struct {
	Enabled            bool
	MaxExtraIterations int
}
