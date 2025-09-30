package agent

import (
	"context"
	"fmt"
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

	// Parallel execution support
	parallelAgent *SimpleParallelSubAgent
}

// NewReactCore - 创建ReAct核心实例
func NewReactCore(agent *ReactAgent, toolRegistry *ToolRegistry) *ReactCore {
	// Validate inputs
	if agent == nil {
		utils.CoreLogger.Error("Cannot create ReactCore with nil agent")
		return nil
	}
	if toolRegistry == nil {
		utils.CoreLogger.Error("Cannot create ReactCore with nil toolRegistry")
		return nil
	}

	// Create components with additional validation
	messageProcessor := message.NewMessageProcessor(agent.llm, agent.sessionManager, agent.llmConfig)
	if messageProcessor == nil {
		utils.CoreLogger.Error("Failed to create MessageProcessor")
		return nil
	}

	llmHandler := NewLLMHandler(agent.sessionManager, nil)
	if llmHandler == nil {
		utils.CoreLogger.Error("Failed to create LLMHandler")
		return nil
	}

	toolHandler := NewToolHandler(toolRegistry)
	if toolHandler == nil {
		utils.CoreLogger.Error("Failed to create ToolHandler")
		return nil
	}

	promptHandler := NewPromptHandler(agent.promptBuilder)
	if promptHandler == nil {
		utils.CoreLogger.Error("Failed to create PromptHandler")
		return nil
	}

	core := &ReactCore{
		agent:            agent,
		messageProcessor: messageProcessor,
		llmHandler:       llmHandler,
		toolHandler:      toolHandler,
		promptHandler:    promptHandler,
	}

	// Initialize parallel agent with default configuration
	parallelAgent, err := NewSimpleParallelSubAgent(core, DefaultParallelConfig())
	if err != nil {
		// Log error but don't fail core creation
		utils.CoreLogger.Error("Failed to initialize parallel subagent: %v", err)
	} else {
		core.parallelAgent = parallelAgent
	}

	return core
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

	// Ultra Think: 预分析用户任务
	taskAnalysis, err := rc.performTaskPreAnalysis(ctx, task)
	if err != nil {
		utils.CoreLogger.Warn("Task pre-analysis failed, continuing with normal flow: %v", err)
	} else if isStreaming && taskAnalysis != "" {
		streamCallback(StreamChunk{Type: "analysis", Content: taskAnalysis, Metadata: map[string]any{"phase": "pre-analysis"}})
	}

	// 将分析结果融入任务输入
	enhancedTask := task
	if taskAnalysis != "" {
		enhancedTask = fmt.Sprintf("Task Analysis: %s\n\nOriginal Task: %s", taskAnalysis, task)
	}

	// 添加用户消息到session（使用增强后的任务）
	userMsg := &agentsession.Message{
		Role:    "user",
		Content: enhancedTask,
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

	llmMessages := rc.messageProcessor.ConvertSessionToLLM(sessionMessages)

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
		Task:           enhancedTask,
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
		utils.CoreLogger.Error("Core execution failed: %v", err)
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
				Role:       msg.Role,
				Content:    msg.Content,
				ToolCallId: msg.ToolCallId,
				Name:       msg.Name,
				Timestamp:  time.Now(),
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
	sessionHelper := utils.CoreSessionHelper
	sessionHelper.AddMessageToSession(llmMsg, session, rc.agent.currentSession)
}

// executeToolDirect - 直接使用registry执行工具
func (rc *ReactCore) executeToolDirect(ctx context.Context, toolName string, args map[string]interface{}, callId string) (*types.ReactToolResult, error) {
	coreLogger := utils.CoreLogger
	coreLogger.Debug("Starting execution - Tool: '%s', CallID: '%s'", toolName, callId)

	// 添加nil检查防止panic
	if rc == nil {
		coreLogger.Error("ReactCore is nil")
		return &types.ReactToolResult{
			Success:  false,
			Error:    "ReactCore is nil",
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, fmt.Errorf("ReactCore is nil")
	}

	if rc.toolHandler == nil {
		coreLogger.Error("toolHandler is nil")
		return &types.ReactToolResult{
			Success:  false,
			Error:    "toolHandler is nil",
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, fmt.Errorf("toolHandler is nil")
	}

	if rc.toolHandler.registry == nil {
		coreLogger.Error("toolHandler.registry is nil")
		return &types.ReactToolResult{
			Success:  false,
			Error:    "toolHandler.registry is nil",
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, fmt.Errorf("toolHandler.registry is nil")
	}

	// 使用ReactCore的工具注册器获取工具
	tool, err := rc.toolHandler.registry.GetTool(ctx, toolName)
	if err != nil {
		coreLogger.Error("Failed to get tool '%s': %v", toolName, err)
		return &types.ReactToolResult{
			Success:  false,
			Error:    fmt.Sprintf("Failed to get tool '%s': %v", toolName, err),
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, err
	}

	if tool == nil {
		coreLogger.Error("Tool '%s' is nil", toolName)
		return &types.ReactToolResult{
			Success:  false,
			Error:    fmt.Sprintf("Tool '%s' is nil", toolName),
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, fmt.Errorf("tool '%s' is nil", toolName)
	}

	// 确保args不为nil
	if args == nil {
		args = make(map[string]interface{})
	}

	// 执行工具
	result, err := tool.Execute(ctx, args)
	if err != nil {
		coreLogger.Error("Tool '%s' execution failed: %v", toolName, err)
		return &types.ReactToolResult{
			Success:  false,
			Error:    err.Error(),
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, nil
	}

	if result == nil {
		coreLogger.Error("Tool '%s' returned nil result", toolName)
		return &types.ReactToolResult{
			Success:  false,
			Error:    "tool returned nil result",
			ToolName: toolName,
			ToolArgs: args,
			CallID:   callId,
		}, nil
	}

	// 转换为ReactToolResult格式
	// builtin.ToolResult没有Success和Error字段，成功执行就认为是成功的
	reactResult := &types.ReactToolResult{
		Success:  true, // 能够执行到这里说明没有错误
		Content:  result.Content,
		Error:    "", // builtin.ToolResult没有Error字段
		ToolName: toolName,
		ToolArgs: args,
		CallID:   callId,
		Data:     result.Data,
	}

	coreLogger.Debug("Tool '%s' executed successfully", toolName)
	return reactResult, nil
}

// performTaskPreAnalysis - 执行任务预分析，理解用户需求并分析所需信息
func (rc *ReactCore) performTaskPreAnalysis(ctx context.Context, task string) (string, error) {
	coreLogger := utils.CoreLogger
	coreLogger.Debug("Starting task pre-analysis for: %s", task)

	// 创建优化的英文分析提示
	analysisPrompt := fmt.Sprintf(`Ultra-brief task analysis in 2 lines:
1. Goal: What specific outcome does the user want?
2. Needs: What files/tools/data are likely required?

Task: %s

Reply format: "Goal: [action]. Needs: [specific items]."
Max: 80 chars. Be precise.`, task)

	// 获取LLM实例
	llmClient, err := llm.GetLLMInstance(llm.BasicModel)
	if err != nil {
		return "", fmt.Errorf("failed to get LLM instance for pre-analysis: %w", err)
	}

	// 构建分析消息
	messages := []llm.Message{
		{
			Role:    "user",
			Content: analysisPrompt,
		},
	}

	// 设置简单的配置，快速响应
	config := &llm.Config{
		Temperature: 0.2, // 降低随机性，更聚焦
		MaxTokens:   60,  // 严格限制输出长度
	}

	// 构建ChatRequest
	chatReq := &llm.ChatRequest{
		Messages:    messages,
		Temperature: config.Temperature,
		MaxTokens:   config.MaxTokens,
		ModelType:   llm.BasicModel,
	}

	// 发送LLM请求
	response, err := llmClient.Chat(ctx, chatReq, "")
	if err != nil {
		return "", fmt.Errorf("LLM pre-analysis request failed: %w", err)
	}

	if response == nil || len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from LLM pre-analysis")
	}

	content := response.Choices[0].Message.Content
	coreLogger.Debug("Task pre-analysis completed: %s", content)
	return content, nil
}

// ExecuteTasksParallel - Implementation of ParallelSubAgentExecutor interface
// This method allows the parallel subagent tool to use ReactCore for execution
func (rc *ReactCore) ExecuteTasksParallel(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	if rc.parallelAgent == nil {
		return nil, fmt.Errorf("parallel subagent not initialized")
	}

	return rc.parallelAgent.ExecuteTasksParallelFromTool(ctx, args)
}

// filterSubAgentCalls - Filter out subagent tool calls from the tool calls list
func (rc *ReactCore) filterSubAgentCalls(toolCalls []*types.ReactToolCall) []*types.ReactToolCall {
	var subagentCalls []*types.ReactToolCall
	for _, call := range toolCalls {
		if call.Name == "subagent" {
			subagentCalls = append(subagentCalls, call)
		}
	}
	return subagentCalls
}

// executeSubAgentsInParallel - Execute multiple subagent calls in parallel while preserving order
func (rc *ReactCore) executeSubAgentsInParallel(
	ctx context.Context,
	allToolCalls []*types.ReactToolCall,
	subagentCalls []*types.ReactToolCall,
	streamCallback StreamCallback,
) []*types.ReactToolResult {
	if rc.parallelAgent == nil {
		utils.CoreLogger.Error("Parallel agent not initialized, falling back to serial execution")
		return rc.executeSerialFallback(ctx, allToolCalls, streamCallback)
	}

	// Extract tasks from subagent calls
	var tasks []string
	var callIDMap = make(map[int]string) // Map task index to original call ID

	subagentIndex := 0
	for _, call := range subagentCalls {
		if task, ok := call.Arguments["task"].(string); ok {
			tasks = append(tasks, task)
			callIDMap[subagentIndex] = call.CallID
			subagentIndex++
		}
	}

	if len(tasks) == 0 {
		utils.CoreLogger.Error("No valid tasks found in subagent calls")
		return rc.executeSerialFallback(ctx, allToolCalls, streamCallback)
	}

	// Execute tasks in parallel using SimpleParallelSubAgent
	subAgentResults, err := rc.parallelAgent.ExecuteTasksParallel(ctx, tasks, streamCallback)
	if err != nil {
		utils.CoreLogger.Error("Parallel execution failed: %v", err)
		return rc.executeSerialFallback(ctx, allToolCalls, streamCallback)
	}

	// Convert SubAgentResult to ReactToolResult and maintain order
	var results []*types.ReactToolResult
	resultMap := make(map[string]*types.ReactToolResult) // Map CallID to result

	for i, subResult := range subAgentResults {
		callID := callIDMap[i]
		result := &types.ReactToolResult{
			Success:  subResult.Success,
			Content:  subResult.Result,
			ToolName: "subagent",
			CallID:   callID,
			Data: map[string]interface{}{
				"success":        subResult.Success,
				"task_completed": subResult.TaskCompleted,
				"session_id":     subResult.SessionID,
				"tokens_used":    subResult.TokensUsed,
				"duration_ms":    subResult.Duration,
			},
		}

		if !subResult.Success {
			result.Error = subResult.ErrorMessage
		}

		resultMap[callID] = result
	}

	// Build final results in original order, mixing parallel and serial results
	for _, call := range allToolCalls {
		if call.Name == "subagent" {
			if result, exists := resultMap[call.CallID]; exists {
				results = append(results, result)
			}
		} else {
			// Execute non-subagent tools serially
			result, err := rc.executeToolDirect(ctx, call.Name, call.Arguments, call.CallID)
			if err != nil {
				result = &types.ReactToolResult{
					Success:  false,
					Error:    err.Error(),
					ToolName: call.Name,
					CallID:   call.CallID,
				}
			}
			results = append(results, result)
		}
	}

	return results
}

// executeSerialFallback - Fallback to serial execution when parallel fails
func (rc *ReactCore) executeSerialFallback(
	ctx context.Context,
	toolCalls []*types.ReactToolCall,
	streamCallback StreamCallback,
) []*types.ReactToolResult {
	utils.CoreLogger.Info("Using serial execution fallback")

	toolExecutor := utils.NewToolExecutor("SUB-AGENT-FALLBACK")
	displayFormatter := utils.NewToolDisplayFormatter()

	// Convert streamCallback if needed
	var utilsCallback utils.StreamCallback
	if streamCallback != nil {
		utilsCallback = func(chunk utils.StreamChunk) {
			agentChunk := StreamChunk{
				Type:             chunk.Type,
				Content:          chunk.Content,
				Complete:         chunk.Complete,
				Metadata:         chunk.Metadata,
				TokensUsed:       chunk.TokensUsed,
				TotalTokensUsed:  chunk.TotalTokensUsed,
				PromptTokens:     chunk.PromptTokens,
				CompletionTokens: chunk.CompletionTokens,
			}
			streamCallback(agentChunk)
		}
	}

	return toolExecutor.ExecuteSerialToolsWithRecovery(
		ctx,
		toolCalls,
		rc.executeToolDirect,
		utilsCallback,
		displayFormatter.Format,
	)
}
