package ports

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"alex/internal/workflow"
)

// ToolExecutor executes a single tool call
type ToolExecutor interface {
	// Execute runs the tool with given arguments
	Execute(ctx context.Context, call ToolCall) (*ToolResult, error)

	// Definition returns the tool's schema for LLM
	Definition() ToolDefinition

	// Metadata returns tool metadata
	Metadata() ToolMetadata
}

// ServiceBundle contains all dependencies required by the domain engine
type ServiceBundle struct {
	LLM          StreamingLLMClient
	ToolExecutor ToolRegistry
	Parser       FunctionCallParser
	Context      ContextManager
}

// ExecutionEnvironment contains the prepared state for running a task
type ExecutionEnvironment struct {
	State        *TaskState
	Services     ServiceBundle
	Session      *Session
	SystemPrompt string
	TaskAnalysis *TaskAnalysis // Pre-analysis result (action name, goal, approach)
}

// TaskAnalysis contains structured task pre-analysis result
type TaskAnalysis struct {
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

// TaskState tracks execution state during ReAct loop
type TaskState struct {
	SystemPrompt           string
	Messages               []Message
	Iterations             int
	TokenCount             int
	ToolResults            []ToolResult
	Complete               bool
	FinalAnswer            string
	SessionID              string
	TaskID                 string
	ParentTaskID           string
	Attachments            map[string]Attachment
	AttachmentIterations   map[string]int
	PendingUserAttachments map[string]Attachment
	Plans                  []PlanNode
	Beliefs                []Belief
	KnowledgeRefs          []KnowledgeReference
	WorldState             map[string]any
	WorldDiff              map[string]any
	FeedbackSignals        []FeedbackSignal
}

// AgentCoordinator represents the main agent coordinator for subagent delegation
type AgentCoordinator interface {
	// ExecuteTask executes a task with optional event listener and returns the result
	ExecuteTask(ctx context.Context, task string, sessionID string, listener EventListener) (*TaskResult, error)

	// PrepareExecution prepares the execution environment (session, state, services) without running the task
	PrepareExecution(ctx context.Context, task string, sessionID string) (*ExecutionEnvironment, error)

	// SaveSessionAfterExecution saves session state after task completion
	SaveSessionAfterExecution(ctx context.Context, session *Session, result *TaskResult) error

	// ListSessions lists all available sessions
	ListSessions(ctx context.Context) ([]string, error)

	// GetConfig returns the coordinator configuration
	GetConfig() AgentConfig

	// GetLLMClient returns an LLM client
	GetLLMClient() (LLMClient, error)

	// GetToolRegistry returns the tool registry (without subagent for nested calls)
	GetToolRegistryWithoutSubagent() ToolRegistry

	// GetParser returns the function call parser
	GetParser() FunctionCallParser

	// GetContextManager returns the context manager
	GetContextManager() ContextManager

	// GetSystemPrompt returns the system prompt
	GetSystemPrompt() string
}

// AgentConfig exposes the subset of coordinator configuration required by tools
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

// TaskResult represents the result of task execution
type TaskResult struct {
	Answer       string
	Messages     []Message
	Iterations   int
	TokensUsed   int
	StopReason   string
	SessionID    string // The session ID used for this task
	TaskID       string // The unique task identifier for this execution
	ParentTaskID string // The parent task identifier when invoked as a subtask
	Duration     time.Duration
	Workflow     *workflow.WorkflowSnapshot
}

// StreamCallback is called during task execution to stream events
type StreamCallback func(event StreamEvent)

// StreamEvent represents different types of events during execution
type StreamEvent struct {
	Type    string // "tool_start", "tool_end", "thought", "error"
	Tool    string
	Args    map[string]any
	Result  string
	Error   error
	Content string
}

// ToolRegistry manages available tools
type ToolRegistry interface {
	// Register adds a tool to the registry
	Register(tool ToolExecutor) error

	// Get retrieves a tool by name
	Get(name string) (ToolExecutor, error)

	// List returns all available tools
	List() []ToolDefinition

	// Unregister removes a tool
	Unregister(name string) error
}

// ToolCall represents a request to execute a tool
type ToolCall struct {
	ID           string         `json:"id"`
	Name         string         `json:"name"`
	Arguments    map[string]any `json:"arguments"`
	SessionID    string         `json:"session_id,omitempty"`
	TaskID       string         `json:"task_id,omitempty"`
	ParentTaskID string         `json:"parent_task_id,omitempty"`
}

// ToolResult is the execution result
type ToolResult struct {
	CallID       string                `json:"call_id"`
	Content      string                `json:"content"`
	Error        error                 `json:"error,omitempty"`
	Metadata     map[string]any        `json:"metadata,omitempty"`
	SessionID    string                `json:"session_id,omitempty"`
	TaskID       string                `json:"task_id,omitempty"`
	ParentTaskID string                `json:"parent_task_id,omitempty"`
	Attachments  map[string]Attachment `json:"attachments,omitempty"`
}

// MarshalJSON customizes ToolResult JSON encoding to support the error interface.
func (r ToolResult) MarshalJSON() ([]byte, error) {
	type Alias struct {
		CallID      string                `json:"call_id"`
		Content     string                `json:"content"`
		Error       any                   `json:"error,omitempty"`
		Metadata    map[string]any        `json:"metadata,omitempty"`
		Attachments map[string]Attachment `json:"attachments,omitempty"`
	}

	alias := Alias{
		CallID:      r.CallID,
		Content:     r.Content,
		Metadata:    r.Metadata,
		Attachments: r.Attachments,
	}

	if r.Error != nil {
		alias.Error = r.Error.Error()
	}

	return json.Marshal(alias)
}

// UnmarshalJSON customizes ToolResult decoding to accept both string and object error representations.
func (r *ToolResult) UnmarshalJSON(data []byte) error {
	type Alias struct {
		CallID      string                `json:"call_id"`
		Content     string                `json:"content"`
		Error       json.RawMessage       `json:"error"`
		Metadata    map[string]any        `json:"metadata,omitempty"`
		Attachments map[string]Attachment `json:"attachments,omitempty"`
	}

	var aux Alias
	if err := json.Unmarshal(data, &aux); err != nil {
		return err
	}

	r.CallID = aux.CallID
	r.Content = aux.Content
	r.Metadata = aux.Metadata
	r.Attachments = aux.Attachments
	r.Error = nil

	raw := strings.TrimSpace(string(aux.Error))
	if raw == "" || raw == "null" {
		return nil
	}

	var errStr string
	if err := json.Unmarshal(aux.Error, &errStr); err == nil {
		if errStr != "" {
			r.Error = errors.New(errStr)
		}
		return nil
	}

	var errObj map[string]any
	if err := json.Unmarshal(aux.Error, &errObj); err == nil {
		if msg, ok := errObj["message"].(string); ok && msg != "" {
			r.Error = errors.New(msg)
			return nil
		}
		if msg, ok := errObj["error"].(string); ok && msg != "" {
			r.Error = errors.New(msg)
			return nil
		}
	}

	// Fallback: use the raw JSON string as the error message
	if raw != "" {
		r.Error = errors.New(raw)
	}

	return nil
}

// ToolDefinition describes a tool for the LLM
type ToolMaterialCapabilities struct {
	Consumes          []string `json:"consumes,omitempty"`
	Produces          []string `json:"produces,omitempty"`
	ProducesArtifacts []string `json:"produces_artifacts,omitempty"`
}

// IsZero allows ToolMaterialCapabilities to honor json omitempty semantics.
func (c ToolMaterialCapabilities) IsZero() bool {
	return len(c.Consumes) == 0 && len(c.Produces) == 0 && len(c.ProducesArtifacts) == 0
}

// ToolDefinition describes a tool for the LLM
type ToolDefinition struct {
	Name                 string                   `json:"name"`
	Description          string                   `json:"description"`
	Parameters           ParameterSchema          `json:"parameters"`
	MaterialCapabilities ToolMaterialCapabilities `json:"material_capabilities,omitempty"`
}

// ToolMetadata contains tool information
type ToolMetadata struct {
	Name                 string                   `json:"name"`
	Version              string                   `json:"version"`
	Category             string                   `json:"category"`
	Tags                 []string                 `json:"tags"`
	Dangerous            bool                     `json:"dangerous"`
	MaterialCapabilities ToolMaterialCapabilities `json:"material_capabilities,omitempty"`
}

// ParameterSchema defines tool parameters (JSON Schema format)
type ParameterSchema struct {
	Type       string              `json:"type"`
	Properties map[string]Property `json:"properties"`
	Required   []string            `json:"required,omitempty"`
}

// Property defines a single parameter
type Property struct {
	Type        string `json:"type"`
	Description string `json:"description"`
	Enum        []any  `json:"enum,omitempty"`
}
