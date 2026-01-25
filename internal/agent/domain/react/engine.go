package react

import (
	"context"
	"sync"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	tools "alex/internal/agent/ports/tools"
	materialports "alex/internal/materials/ports"
)

// ReactEngine orchestrates the Think-Act-Observe cycle
type ReactEngine struct {
	maxIterations      int
	stopReasons        []string
	logger             agent.Logger
	clock              agent.Clock
	eventListener      EventListener // Optional event listener for TUI
	completion         completionConfig
	attachmentMigrator materialports.Migrator
	workflow           WorkflowTracker
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
	MaxIterations      int
	StopReasons        []string
	Logger             agent.Logger
	Clock              agent.Clock
	EventListener      EventListener
	CompletionDefaults CompletionDefaults
	AttachmentMigrator materialports.Migrator
	Workflow           WorkflowTracker
}
