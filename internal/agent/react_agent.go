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
	toolExecutor  ToolExecutor      // Interface for tool execution
	toolParser    *ToolExecutorImpl // Implementation for parsing
	promptBuilder *LightPromptBuilder

	// 消息队列机制
	messageQueue *MessageQueue

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
	Type             string         `json:"type"`
	Content          string         `json:"content"`
	Complete         bool           `json:"complete,omitempty"`
	Metadata         map[string]any `json:"metadata,omitempty"`
	TokensUsed       int            `json:"tokens_used,omitempty"`
	TotalTokensUsed  int            `json:"total_tokens_used,omitempty"`
	PromptTokens     int            `json:"prompt_tokens,omitempty"`
	CompletionTokens int            `json:"completion_tokens,omitempty"`
}

// StreamCallback - 流式回调函数
type StreamCallback func(StreamChunk)

// MessageQueueItem - 消息队列项
type MessageQueueItem struct {
	Message   string          `json:"message"`
	Timestamp time.Time       `json:"timestamp"`
	Callback  StreamCallback  `json:"-"` // 不序列化回调函数
	Context   context.Context `json:"-"` // 不序列化context
	Config    *config.Config  `json:"-"` // 不序列化config
	Metadata  map[string]any  `json:"metadata,omitempty"`
}

// MessageQueue - 消息队列
type MessageQueue struct {
	items []MessageQueueItem
	mutex sync.RWMutex
}

// LightPromptBuilder - 轻量化prompt构建器
type LightPromptBuilder struct {
	promptLoader *prompts.PromptLoader
}

// NewSimplifiedAgent - creates a new simplified Agent (recommended for new code)
func NewSimplifiedAgent(configManager *config.Manager) (*Agent, error) {
	return LegacyAgentFactory(configManager)
}

// NewReactAgent - 创建简化的ReactAgent (legacy, maintained for backward compatibility)
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

	// Tool registry and LLM config initialized

	agent := &ReactAgent{
		llm:            llmClient,
		configManager:  configManager,
		sessionManager: sessionManager,
		toolRegistry:   toolRegistry,
		config:         types.NewReactConfig(),
		llmConfig:      llmConfig,

		promptBuilder: NewLightPromptBuilder(),
		messageQueue:  NewMessageQueue(),
	}

	// 初始化核心组件
	agent.reactCore = NewReactCore(agent, toolRegistry)
	agent.toolExecutor = NewToolExecutorAdapter(toolRegistry) // Interface implementation
	agent.toolParser = NewToolExecutorImpl(toolRegistry)       // Parsing implementation

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
	// Processing stream message

	r.mu.RLock()
	currentSession := r.currentSession
	r.mu.RUnlock()

	// If no active session, create one automatically
	if currentSession == nil {
		// Creating new session automatically
		sessionID := fmt.Sprintf("auto_%d", time.Now().UnixNano())
		newSession, err := r.StartSession(sessionID)
		if err != nil {
			return fmt.Errorf("failed to create session automatically: %w", err)
		}
		// Update instance variable
		r.mu.Lock()
		r.currentSession = newSession
		r.mu.Unlock()
		log.Printf("Auto-created session: %s", sessionID)
	}

	// Context prepared with session ID

	// 执行流式ReAct循环
	result, err := r.reactCore.SolveTask(ctx, userMessage, callback)
	if err != nil {
		return fmt.Errorf("streaming task solving failed: %w", err)
	}

	// 发送当前任务完成信号
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

	// Check for queued messages after task completion
	if r.HasPendingMessages() {
		// Processing next queued message

		// Get next pending message
		if pendingItem, hasItem := r.CheckPendingMessages(); hasItem {
			// Processing next message from queue

			// 发送开始处理下一个消息的信号
			if callback != nil {
				callback(StreamChunk{
					Type:     "next_message_start",
					Content:  fmt.Sprintf("📬 Starting next message: %s", pendingItem.Message),
					Metadata: map[string]any{"phase": "queue_processing"},
				})
			}

			// 递归调用ProcessMessageStream处理下一个消息
			// 这样保持了正常的消息处理流程
			return r.ProcessMessageStream(pendingItem.Context, pendingItem.Message, pendingItem.Config, pendingItem.Callback)
		}
	}

	return nil
}

// ========== 公共接口 ==========

// GetAvailableTools - 获取可用工具列表
func (r *ReactAgent) GetAvailableTools(ctx context.Context) []string {
	return r.toolRegistry.ListTools(ctx)
}

// AddMessage - 公共接口：添加消息到队列
func (r *ReactAgent) AddMessage(ctx context.Context, message string, config *config.Config, callback StreamCallback) {
	// Adding message to queue
	r.EnqueueMessage(ctx, message, config, callback)
}

// GetQueueSize - 获取消息队列大小
func (r *ReactAgent) GetQueueSize() int {
	return r.messageQueue.Size()
}

// ClearMessageQueue - 清空消息队列
func (r *ReactAgent) ClearMessageQueue() {
	r.messageQueue.Clear()
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

// GetSessionID - 获取当前会话ID
func (r *ReactAgent) GetSessionID() (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	if r.currentSession == nil {
		return "", fmt.Errorf("no active session")
	}
	return r.currentSession.ID, nil
}

// parseToolCalls - 委托给ToolExecutorImpl
func (r *ReactAgent) parseToolCalls(message *llm.Message) []*types.ReactToolCall {
	return r.toolParser.parseToolCalls(message)
}

// ========== 消息队列管理 ==========

// NewMessageQueue - 创建新的消息队列
func NewMessageQueue() *MessageQueue {
	return &MessageQueue{
		items: make([]MessageQueueItem, 0),
	}
}

// Enqueue - 添加消息到队列
func (mq *MessageQueue) Enqueue(item MessageQueueItem) {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()
	mq.items = append(mq.items, item)
	// Item added to queue
}

// Dequeue - 从队列取出消息
func (mq *MessageQueue) Dequeue() (MessageQueueItem, bool) {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()

	if len(mq.items) == 0 {
		// Queue is empty
		return MessageQueueItem{}, false
	}

	item := mq.items[0]
	mq.items = mq.items[1:]
	// Message dequeued
	return item, true
}

// HasPendingMessages - 检查是否有待处理的消息
func (mq *MessageQueue) HasPendingMessages() bool {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()
	return len(mq.items) > 0
}

// Size - 获取队列大小
func (mq *MessageQueue) Size() int {
	mq.mutex.RLock()
	defer mq.mutex.RUnlock()
	return len(mq.items)
}

// Clear - 清空队列
func (mq *MessageQueue) Clear() {
	mq.mutex.Lock()
	defer mq.mutex.Unlock()
	mq.items = mq.items[:0]
}

// EnqueueMessage - ReactAgent的消息入队方法
func (r *ReactAgent) EnqueueMessage(ctx context.Context, message string, config *config.Config, callback StreamCallback) {
	if r.messageQueue == nil {
		log.Printf("[ERROR] ReactAgent: messageQueue is nil! Cannot enqueue message.")
		return
	}

	item := MessageQueueItem{
		Message:   message,
		Timestamp: time.Now(),
		Callback:  callback,
		Context:   ctx,
		Config:    config,
		Metadata: map[string]any{
			"queued_at": time.Now().Unix(),
		},
	}

	r.messageQueue.Enqueue(item)
	// Message enqueued successfully
}

// CheckPendingMessages - 检查并处理待处理的消息
func (r *ReactAgent) CheckPendingMessages() (MessageQueueItem, bool) {
	item, found := r.messageQueue.Dequeue()
	// Message dequeue operation completed
	return item, found
}

// HasPendingMessages - 检查是否有待处理的消息
func (r *ReactAgent) HasPendingMessages() bool {
	return r.messageQueue.HasPendingMessages()
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
