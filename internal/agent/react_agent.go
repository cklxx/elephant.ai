package agent

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"alex/internal/config"
	"alex/internal/llm"
	"alex/internal/prompts"
	"alex/internal/session"
	"alex/pkg/types"
)

// ReactCoreInterface - ReAct核心接口
type ReactCoreInterface interface {
	SolveTask(ctx context.Context, task string, streamCallback StreamCallback) (*types.ReactTaskResult, error)
}

// ReactAgent - 简化的ReAct引擎
type ReactAgent struct {
	// 核心组件
	llm            llm.Client
	configManager  *config.Manager
	sessionManager *session.Manager
	toolRegistry   *ToolRegistry
	config         *types.ReactConfig
	llmConfig      *llm.Config
	currentSession *session.Session

	// 核心组件
	reactCore     ReactCoreInterface
	toolExecutor  *ToolExecutor
	promptBuilder *LightPromptBuilder

	// 简单的同步控制
	mu sync.RWMutex
}

// Response - 响应格式
type Response struct {
	Message     *session.Message        `json:"message"`
	ToolResults []types.ReactToolResult `json:"toolResults"`
	SessionID   string                  `json:"sessionId"`
	Complete    bool                    `json:"complete"`
}

// StreamChunk - 流式响应
type StreamChunk struct {
	Type             string                 `json:"type"`
	Content          string                 `json:"content"`
	Complete         bool                   `json:"complete,omitempty"`
	Metadata         map[string]interface{} `json:"metadata,omitempty"`
	TokensUsed       int                    `json:"tokens_used,omitempty"`
	TotalTokensUsed  int                    `json:"total_tokens_used,omitempty"`
	PromptTokens     int                    `json:"prompt_tokens,omitempty"`
	CompletionTokens int                    `json:"completion_tokens,omitempty"`
}

// StreamCallback - 流式回调函数
type StreamCallback func(StreamChunk)

// LightPromptBuilder - 轻量化prompt构建器
type LightPromptBuilder struct {
	promptLoader *prompts.PromptLoader
}

// NewReactAgent - 创建简化的ReactAgent
func NewReactAgent(configManager *config.Manager) (*ReactAgent, error) {
	// 设置LLM配置提供函数
	llm.SetConfigProvider(func() (*llm.Config, error) {
		return configManager.GetLLMConfig(), nil
	})

	// 获取LLM配置和客户端
	llmConfig := configManager.GetLLMConfig()
	llmClient, err := llm.GetLLMInstance(llm.BasicModel)
	if err != nil {
		log.Printf("[ERROR] ReactAgent: Failed to get LLM instance: %v", err)
		return nil, fmt.Errorf("failed to get LLM instance: %w", err)
	}

	// 创建session manager
	sessionManager, err := session.NewManager()
	if err != nil {
		log.Printf("[ERROR] ReactAgent: Failed to create session manager: %v", err)
		return nil, fmt.Errorf("failed to create session manager: %w", err)
	}

	// 创建统一的工具注册器
	toolRegistry := NewToolRegistry(configManager, sessionManager)

	agent := &ReactAgent{
		llm:            llmClient,
		configManager:  configManager,
		sessionManager: sessionManager,
		toolRegistry:   toolRegistry,
		config:         types.NewReactConfig(),
		llmConfig:      llmConfig,

		promptBuilder: NewLightPromptBuilder(),
	}

	// 初始化核心组件
	agent.reactCore = NewReactCore(agent, toolRegistry)
	agent.toolExecutor = NewToolExecutor(toolRegistry)

	// 注册sub-agent工具到工具注册器
	if reactCore, ok := agent.reactCore.(*ReactCore); ok {
		toolRegistry.RegisterSubAgentTool(reactCore)
	}

	// Memory tools removed

	return agent, nil
}

// ========== 会话管理 ==========

// StartSession - 开始会话
func (r *ReactAgent) StartSession(sessionID string) (*session.Session, error) {
	session, err := r.sessionManager.StartSession(sessionID)
	if err != nil {
		return nil, err
	}

	r.mu.Lock()
	r.currentSession = session
	r.mu.Unlock()

	return session, nil
}

// RestoreSession - 恢复会话
func (r *ReactAgent) RestoreSession(sessionID string) (*session.Session, error) {
	session, err := r.sessionManager.RestoreSession(sessionID)
	if err != nil {
		log.Printf("[ERROR] ReactAgent: Failed to restore session %s: %v", sessionID, err)
		return nil, err
	}

	r.mu.Lock()
	r.currentSession = session
	r.mu.Unlock()

	return session, nil
}

// ProcessMessageStream - 流式处理消息
func (r *ReactAgent) ProcessMessageStream(ctx context.Context, userMessage string, config *config.Config, callback StreamCallback) error {
	log.Printf("[DEBUG] ====== ProcessMessageStream called with message: %s", userMessage)

	r.mu.RLock()
	currentSession := r.currentSession
	r.mu.RUnlock()

	// If no active session, create one automatically
	if currentSession == nil {
		log.Printf("[DEBUG] No active session found, creating new session automatically")
		sessionID := fmt.Sprintf("auto_%d", time.Now().UnixNano())
		newSession, err := r.StartSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to create session automatically: %w", err)
		}
		currentSession = newSession
		log.Printf("[DEBUG] Auto-created session: %s", currentSession.ID)
	} else {
		if currentSession.ID == "" {
			log.Printf("[DEBUG] ⚠️ Session exists but has empty ID!")
		} else {
			log.Printf("[DEBUG] Using existing session: %s", currentSession.ID)
		}
	}

	// 将session ID通过其他方式传递给core，不使用context
	// 这里可以通过直接调用方法传递
	log.Printf("[DEBUG] 🔧 Context set with session ID: %s", currentSession.ID)

	// 执行流式ReAct循环
	result, err := r.reactCore.SolveTask(ctx, userMessage, callback)
	if err != nil {
		return fmt.Errorf("streaming task solving failed: %w", err)
	}
	// 发送完成信号
	if callback != nil {
		callback(StreamChunk{
			Type:             "complete",
			Content:          "Task completed",
			Complete:         true,
			TotalTokensUsed:  result.TokensUsed,
			PromptTokens:     result.PromptTokens,
			CompletionTokens: result.CompletionTokens,
		})
	}

	return nil
}

// ========== 公共接口 ==========

// GetAvailableTools - 获取可用工具列表
func (r *ReactAgent) GetAvailableTools(ctx context.Context) []string {
	return r.toolRegistry.ListTools(ctx)
}

// GetSessionHistory - 获取会话历史
func (r *ReactAgent) GetSessionHistory() []*session.Message {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentSession == nil {
		return nil
	}
	return r.currentSession.Messages
}

// GetReactCore - 获取ReactCore实例
func (r *ReactAgent) GetReactCore() ReactCoreInterface {
	return r.reactCore
}

// GetSessionManager - 获取SessionManager实例
func (r *ReactAgent) GetSessionManager() *session.Manager {
	return r.sessionManager
}

// parseToolCalls - 委托给ToolExecutor
func (r *ReactAgent) parseToolCalls(message *llm.Message) []*types.ReactToolCall {
	return r.toolExecutor.parseToolCalls(message)
}


// ========== 组件创建函数 ==========

// NewLightPromptBuilder - 创建轻量化提示构建器
func NewLightPromptBuilder() *LightPromptBuilder {
	promptLoader, err := prompts.NewPromptLoader()
	if err != nil {
		log.Printf("[ERROR] LightPromptBuilder: Failed to create prompt loader: %v", err)
		return &LightPromptBuilder{promptLoader: nil}
	}

	return &LightPromptBuilder{
		promptLoader: promptLoader,
	}
}
