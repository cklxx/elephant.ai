package agent

import (
	"context"
	"fmt"
	"log"
	"time"

	"alex/internal/context/message"
	"alex/internal/llm"
	agentsession "alex/internal/session"
	"alex/internal/utils"
	"alex/pkg/types"
)

// ReactCore - 使用工具调用流程的ReactCore核心实现
type ReactCore struct {
	agent            *ReactAgent
	streamCallback   StreamCallback
	messageProcessor *message.MessageProcessor
	llmHandler       *LLMHandler
	toolHandler      *ToolHandler
	promptHandler    *PromptHandler
}

// NewReactCore - 创建ReAct核心实例
func NewReactCore(agent *ReactAgent, toolRegistry *ToolRegistry) *ReactCore {
	llmClient, err := llm.GetLLMInstance(llm.BasicModel)
	if err != nil {
		log.Printf("[ERROR] NewReactCore: Failed to get LLM instance: %v", err)
		llmClient = nil
	}

	return &ReactCore{
		agent:            agent,
		messageProcessor: message.NewMessageProcessor(llmClient, agent.sessionManager),
		llmHandler:       NewLLMHandler(agent.sessionManager, nil), // Will be set per request
		toolHandler:      NewToolHandler(toolRegistry),
		promptHandler:    NewPromptHandler(agent.promptBuilder),
	}
}

// SolveTask - 使用抽象化核心逻辑的简化任务解决方法
func (rc *ReactCore) SolveTask(ctx context.Context, task string, streamCallback StreamCallback) (*types.ReactTaskResult, error) {
	// 设置流回调
	rc.streamCallback = streamCallback
	rc.llmHandler.streamCallback = streamCallback

	// 生成任务ID
	taskID := generateTaskID()

	// 初始化任务上下文
	taskCtx := types.NewReactTaskContext(taskID, task)
	ctx = context.WithValue(ctx, utils.WorkingDirKey, taskCtx.WorkingDir)

	// 决定是否使用流式处理
	isStreaming := streamCallback != nil
	if isStreaming {
		streamCallback(StreamChunk{Type: "status", Content: message.GetRandomProcessingMessage(), Metadata: map[string]any{"phase": "initialization"}})
	}

	// 添加用户消息到session
	userMsg := &agentsession.Message{
		Role:    "user",
		Content: task,
		Metadata: map[string]interface{}{
			"timestamp": time.Now().Unix(),
			"streaming": true,
		},
		Timestamp: time.Now(),
	}
	rc.agent.currentSession.AddMessage(userMsg)

	// 构建系统提示
	systemPrompt := rc.promptHandler.buildToolDrivenTaskPrompt(taskCtx)

	// 准备消息列表（包含session历史）
	sess := rc.agent.currentSession
	sessionMessages := sess.GetMessages()

	unifiedMessages := rc.messageProcessor.ConvertSessionToUnified(sessionMessages)
	llmMessages := rc.messageProcessor.ConvertUnifiedToLLM(unifiedMessages)

	// 构建完整的消息列表
	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}
	messages = append(messages, llmMessages...)

	// 创建任务执行上下文
	execCtx := &TaskExecutionContext{
		TaskID:         taskID,
		Task:           task,
		Messages:       messages,
		TaskCtx:        taskCtx,
		Tools:          rc.toolHandler.buildToolDefinitions(ctx),
		Config:         rc.agent.llmConfig,
		MaxIter:        100,
		Session:        rc.agent.currentSession, // 主agent使用currentSession
		SessionManager: rc.agent.sessionManager,
	}

	// 使用抽象化的核心执行逻辑
	result, err := rc.ExecuteTaskCore(ctx, execCtx, streamCallback)
	if err != nil {
		log.Printf("[ERROR] ReactCore: Core execution failed: %v", err)
		return nil, fmt.Errorf("core execution failed: %w", err)
	}

	// 将执行过程中的消息添加到session
	for _, msg := range result.Messages {
		// 跳过系统消息和已经添加的消息
		if msg.Role == "system" {
			continue
		}

		switch msg.Role {
		case "assistant":
			rc.addMessageToSession(&msg, execCtx.Session)
		case "tool":
			// 工具消息需要特殊处理
			sessionMsg := &agentsession.Message{
				Role:      msg.Role,
				Content:   msg.Content,
				Timestamp: time.Now(),
				Metadata: map[string]interface{}{
					"source":    "tool_result",
					"timestamp": time.Now().Unix(),
				},
			}
			if msg.ToolCallId != "" {
				sessionMsg.Metadata["tool_call_id"] = msg.ToolCallId
			}
			if msg.Name != "" {
				sessionMsg.Metadata["tool_name"] = msg.Name
			}
			// 使用execCtx的session而不是sess变量
			targetSession := execCtx.Session
			if targetSession == nil {
				targetSession = rc.agent.currentSession
			}
			if targetSession != nil {
				targetSession.AddMessage(sessionMsg)
			}
		}
	}

	// 构建最终结果
	finalResult := buildFinalResult(taskCtx, result.Answer, result.Confidence, result.Success)
	finalResult.TokensUsed = result.TokensUsed
	finalResult.PromptTokens = result.PromptTokens
	finalResult.CompletionTokens = result.CompletionTokens
	finalResult.Steps = result.History

	return finalResult, nil
}

// addMessageToSession - 将LLM消息添加到session中供memory系统学习
func (rc *ReactCore) addMessageToSession(llmMsg *llm.Message, session *agentsession.Session) {
	// 使用传入的session，如果为nil则使用当前会话
	sess := session
	if sess == nil {
		sess = rc.agent.currentSession
	}
	if sess == nil {
		return // 没有会话则跳过
	}

	// 转换LLM消息为session消息格式
	sessionMsg := &agentsession.Message{
		Role:      llmMsg.Role,
		Content:   llmMsg.Content,
		Timestamp: time.Now(),
		Metadata: map[string]interface{}{
			"source":    "llm_response",
			"timestamp": time.Now().Unix(),
		},
	}

	// 转换工具调用信息
	if len(llmMsg.ToolCalls) > 0 {
		for _, tc := range llmMsg.ToolCalls {
			// 将Arguments字符串解析为map[string]interface{}
			var args map[string]interface{}
			if tc.Function.Arguments != "" {
				// 简单处理：如果是JSON字符串尝试解析，否则存为字符串
				args = map[string]interface{}{"raw": tc.Function.Arguments}
			}

			sessionMsg.ToolCalls = append(sessionMsg.ToolCalls, agentsession.ToolCall{
				ID:   tc.ID,
				Name: tc.Function.Name,
				Args: args,
			})
		}
		sessionMsg.Metadata["has_tool_calls"] = true
		sessionMsg.Metadata["tool_count"] = len(llmMsg.ToolCalls)
	}

	// 添加到session
	sess.AddMessage(sessionMsg)
}

// readCurrentTodos - 读取当前会话的TODO列表
func (rc *ReactCore) readCurrentTodos(ctx context.Context, session *agentsession.Session) string {
	// 使用传入的session，如果为nil则使用当前会话
	sess := session
	if sess == nil {
		sess = rc.agent.currentSession
	}
	if sess == nil {
		log.Printf("[DEBUG] ReactCore: No session available, cannot read todos")
		return ""
	}

	sessionID := sess.ID
	if sessionID == "" {
		log.Printf("[DEBUG] ReactCore: Session has empty ID, cannot read todos")
		return ""
	}

	// 直接调用todo工具，传递session ID作为参数
	if todoTool, err := rc.toolHandler.registry.GetTool(ctx, "todo_read"); err == nil {
		args := map[string]interface{}{
			"session_id": sessionID,
		}
		result, err := todoTool.Execute(ctx, args)
		if err != nil {
			log.Printf("[DEBUG] ReactCore: Failed to read todos: %v", err)
			return ""
		}
		if result != nil && result.Content != "" {
			return result.Content
		}
	}
	return ""
}
