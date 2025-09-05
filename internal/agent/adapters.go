package agent

import (
	"context"

	"alex/internal/config"
	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/tools/builtin"
	"alex/pkg/types"
)

// LLMClientAdapter wraps the existing LLM client to implement our interface
type LLMClientAdapter struct {
	client llm.Client
}

// NewLLMClientAdapter creates a new LLM client adapter
func NewLLMClientAdapter(client llm.Client) *LLMClientAdapter {
	return &LLMClientAdapter{client: client}
}

// Chat implements the LLMClient interface
func (a *LLMClientAdapter) Chat(ctx context.Context, req *llm.ChatRequest, sessionID string) (*llm.ChatResponse, error) {
	return a.client.Chat(ctx, req, sessionID)
}

// ChatStream implements the LLMClient interface
func (a *LLMClientAdapter) ChatStream(ctx context.Context, req *llm.ChatRequest, sessionID string) (<-chan llm.StreamDelta, error) {
	return a.client.ChatStream(ctx, req, sessionID)
}

// ToolExecutorAdapter wraps the existing ToolRegistry to implement our interface
type ToolExecutorAdapter struct {
	registry *ToolRegistry
}

// NewToolExecutorAdapter creates a new tool executor adapter
func NewToolExecutorAdapter(registry *ToolRegistry) *ToolExecutorAdapter {
	return &ToolExecutorAdapter{registry: registry}
}

// Execute implements the ToolExecutor interface
func (a *ToolExecutorAdapter) Execute(ctx context.Context, name string, args map[string]interface{}, callID string) (*types.ReactToolResult, error) {
	tool, err := a.registry.GetTool(ctx, name)
	if err != nil {
		return &types.ReactToolResult{
			Success:  false,
			Error:    err.Error(),
			ToolName: name,
			ToolArgs: args,
			CallID:   callID,
		}, nil
	}

	result, err := tool.Execute(ctx, args)
	if err != nil {
		return &types.ReactToolResult{
			Success:  false,
			Error:    err.Error(),
			ToolName: name,
			ToolArgs: args,
			CallID:   callID,
		}, nil
	}

	return &types.ReactToolResult{
		Success:  true,
		Content:  result.Content,
		ToolName: name,
		ToolArgs: args,
		CallID:   callID,
		Data:     result.Data,
	}, nil
}

// ListTools implements the ToolExecutor interface
func (a *ToolExecutorAdapter) ListTools(ctx context.Context) []string {
	return a.registry.ListTools(ctx)
}

// GetAllToolDefinitions implements the ToolExecutor interface
func (a *ToolExecutorAdapter) GetAllToolDefinitions(ctx context.Context) []llm.Tool {
	return a.registry.GetAllToolDefinitions(ctx)
}

// GetTool implements the ToolExecutor interface
func (a *ToolExecutorAdapter) GetTool(ctx context.Context, name string) (builtin.Tool, error) {
	return a.registry.GetTool(ctx, name)
}

// SessionManagerAdapter wraps the existing session manager to implement our interface
type SessionManagerAdapter struct {
	manager        *session.Manager
	currentSession *session.Session
}

// NewSessionManagerAdapter creates a new session manager adapter
func NewSessionManagerAdapter(manager *session.Manager) *SessionManagerAdapter {
	return &SessionManagerAdapter{manager: manager}
}

// StartSession implements the SessionManager interface
func (a *SessionManagerAdapter) StartSession(sessionID string) (*session.Session, error) {
	sess, err := a.manager.StartSession(sessionID)
	if err != nil {
		return nil, err
	}
	a.currentSession = sess
	return sess, nil
}

// RestoreSession implements the SessionManager interface
func (a *SessionManagerAdapter) RestoreSession(sessionID string) (*session.Session, error) {
	sess, err := a.manager.RestoreSession(sessionID)
	if err != nil {
		return nil, err
	}
	a.currentSession = sess
	return sess, nil
}

// GetCurrentSession implements the SessionManager interface
func (a *SessionManagerAdapter) GetCurrentSession() *session.Session {
	return a.currentSession
}

// SaveSession implements the SessionManager interface
func (a *SessionManagerAdapter) SaveSession(sess *session.Session) error {
	a.currentSession = sess
	return a.manager.SaveSession(sess)
}

// ReactEngineAdapter wraps the existing ReactCore to implement our interface
type ReactEngineAdapter struct {
	core *ReactCore
}

// NewReactEngineAdapter creates a new react engine adapter
func NewReactEngineAdapter(core *ReactCore) *ReactEngineAdapter {
	return &ReactEngineAdapter{core: core}
}

// ProcessTask implements the ReactEngine interface
func (a *ReactEngineAdapter) ProcessTask(ctx context.Context, task string, callback StreamCallback) (*types.ReactTaskResult, error) {
	return a.core.SolveTask(ctx, task, callback)
}

// ExecuteTaskCore implements the ReactEngine interface
func (a *ReactEngineAdapter) ExecuteTaskCore(ctx context.Context, execCtx *TaskExecutionContext, callback StreamCallback) (*types.ReactExecutionResult, error) {
	return a.core.ExecuteTaskCore(ctx, execCtx, callback)
}

// LegacyAgentFactory creates a new Agent using existing components for backward compatibility
func LegacyAgentFactory(configManager *config.Manager) (*Agent, error) {
	// Create the existing ReactAgent
	reactAgent, err := NewReactAgent(configManager)
	if err != nil {
		return nil, err
	}

	// Extract components and wrap them with adapters
	llmClientAdapter := NewLLMClientAdapter(reactAgent.llm)
	toolExecutorAdapter := NewToolExecutorAdapter(reactAgent.toolRegistry)
	sessionManagerAdapter := NewSessionManagerAdapter(reactAgent.sessionManager)
	reactEngineAdapter := NewReactEngineAdapter(reactAgent.reactCore.(*ReactCore))

	// Create new Agent with adapted components
	agentConfig := AgentConfig{
		LLMClient:      llmClientAdapter,
		ToolExecutor:   toolExecutorAdapter,
		SessionManager: sessionManagerAdapter,
		ReactEngine:    reactEngineAdapter,
		Config:         configManager.GetConfig(),
	}

	return NewAgent(agentConfig)
}