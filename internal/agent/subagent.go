package agent

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/llm"
	"alex/internal/session"
	"alex/internal/utils"
	"alex/pkg/types"
)

// ========== Sub-Agent Logging ==========

// Using the new unified logging system
var subAgentLogger = utils.SubAgentLogger

// subAgentLog - 为向后兼容保留的包装函数
func subAgentLog(level, format string, args ...interface{}) {
	switch level {
	case "DEBUG":
		subAgentLogger.Debug(format, args...)
	case "INFO":
		subAgentLogger.Info(format, args...)
	case "WARN":
		subAgentLogger.Warn(format, args...)
	case "ERROR":
		subAgentLogger.Error(format, args...)
	default:
		subAgentLogger.Info(format, args...)
	}
}

// ========== Core Task Execution Abstraction ==========

// TaskExecutionContext - 独立任务执行上下文，支持session隔离
type TaskExecutionContext struct {
	TaskID         string
	Task           string
	Messages       []llm.Message
	TaskCtx        *types.ReactTaskContext
	Tools          []llm.Tool
	Config         *llm.Config
	MaxIter        int
	Session        *session.Session // 支持独立的session上下文
	SessionManager *session.Manager // 支持独立的session manager
}

// TaskExecutionResult - 任务执行结果
type TaskExecutionResult struct {
	Answer               string
	Success              bool
	Confidence           float64
	TokensUsed           int
	PromptTokens         int
	CompletionTokens     int
	CurrentMessageTokens int // 当前消息token数，压缩后会清零
	History              []types.ReactExecutionStep
	Messages             []llm.Message // 返回更新后的消息列表
}

// ExecuteTaskCore - 核心任务执行逻辑，不依赖session和message管理
// 为sub-agent架构准备的独立执行函数
func (rc *ReactCore) ExecuteTaskCore(ctx context.Context, execCtx *TaskExecutionContext, streamCallback StreamCallback) (*TaskExecutionResult, error) {
	if execCtx == nil {
		return nil, fmt.Errorf("execution context cannot be nil")
	}

	// 初始化执行结果
	result := &TaskExecutionResult{
		Success:    false,
		Confidence: 0.0,
		Messages:   make([]llm.Message, len(execCtx.Messages)),
	}
	copy(result.Messages, execCtx.Messages)

	// 设置默认最大迭代数
	maxIterations := execCtx.MaxIter
	if maxIterations <= 0 {
		maxIterations = 100
	}

	// 决定是否使用流式处理
	isStreaming := streamCallback != nil
	// 注意：不在这里显示初始状态消息，避免与SolveTask中的消息重复

	// 执行核心ReAct循环
	for iteration := 1; iteration <= maxIterations; iteration++ {
		step := types.ReactExecutionStep{
			Number:    iteration,
			Timestamp: time.Now(),
		}

		subAgentLog("INFO", "🔄 Starting iteration %d/%d", iteration, maxIterations)

		// Optimized message queue handling for subagents:
		// SubAgents should focus on their assigned task without interruption from new user messages
		// Only check for pending messages in main agent context (not subagents)
		// This prevents performance issues and race conditions in parallel execution
		if rc.agent != nil && rc.agent.HasPendingMessages() && !isSubAgentContext(execCtx) {
			subAgentLog("DEBUG", "📬 Main agent detected pending messages, integrating into current task")

			// 收集所有待处理的消息 (仅在主 agent 中)
			var newMessages []string
			for rc.agent.HasPendingMessages() && len(newMessages) < 5 { // Limit message integration to prevent overflow
				if pendingItem, hasItem := rc.agent.CheckPendingMessages(); hasItem {
					newMessages = append(newMessages, pendingItem.Message)
					subAgentLog("DEBUG", "📬 Integrating message: %s", pendingItem.Message)
				}
			}

			if len(newMessages) > 0 {
				if isStreaming {
					streamCallback(StreamChunk{
						Type:     "message_integration",
						Content:  fmt.Sprintf("📬 Integrating %d new messages into current task", len(newMessages)),
						Metadata: map[string]any{"iteration": iteration, "phase": "message_integration", "new_messages_count": len(newMessages)},
					})
				}

				// 将新消息融入到当前对话中，更新任务目标
				combinedMessage := "Additional user messages received:\n"
				for i, msg := range newMessages {
					combinedMessage += fmt.Sprintf("%d. %s\n", i+1, msg)
				}
				combinedMessage += "\nPlease integrate these new requirements with the current task and continue working."

				// 添加融合消息到LLM对话历史
				userMsg := llm.Message{
					Role:    "user",
					Content: combinedMessage,
				}
				result.Messages = append(result.Messages, userMsg)

				subAgentLog("DEBUG", "📬 Successfully integrated %d messages into current task", len(newMessages))
			}
		} else if isSubAgentContext(execCtx) {
			// SubAgent should focus on assigned task without message queue interruption
			subAgentLog("DEBUG", "SubAgent context: skipping message queue check for focused execution")
		}

		if isStreaming {
			streamCallback(StreamChunk{
				Type:     "iteration",
				Content:  fmt.Sprintf("🔄 Core Iteration %d: Processing...", iteration),
				Metadata: map[string]any{"iteration": iteration, "phase": "core_processing"},
			})
		}

		// SubAgent 优化的消息压缩系统 - 使用更低的阈值和更好的错误处理
		if rc.messageProcessor != nil {
			// SubAgent 使用更严格的压缩策略：较低的 token 阈值和消息数量阈值
			totalTokensUsed := result.PromptTokens + result.CompletionTokens
			currentTokens := result.CurrentMessageTokens
			messageCount := len(result.Messages)

			// SubAgent 特定的压缩阈值（比主 agent 更低）
			const (
				subAgentTokenThreshold   = 50000 // 50K token 限制（主 agent 是 100K）
				subAgentMessageThreshold = 10    // 10 条消息（主 agent 是 15）
				subAgentForceThreshold   = 30000 // 30K token 强制压缩阈值
			)

			shouldCompress := false
			compressReason := ""

			// 判断是否需要压缩
			if iteration > 1 && (messageCount > subAgentMessageThreshold && currentTokens > subAgentTokenThreshold) {
				shouldCompress = true
				compressReason = "normal_threshold"
			} else if currentTokens > subAgentForceThreshold {
				// 强制压缩：当 token 数过高时，即使在第一次迭代也要压缩
				shouldCompress = true
				compressReason = "force_threshold"
			} else if isSubAgentContext(execCtx) && messageCount > 8 {
				// SubAgent 专用逻辑：即使 token 不多，也要控制消息数量
				shouldCompress = true
				compressReason = "subagent_message_limit"
			}

			if shouldCompress {
				subAgentLog("INFO", "💾 SubAgent compressing messages at iteration %d (reason: %s): %d messages, %d tokens",
					iteration, compressReason, messageCount, currentTokens)

				// 执行压缩，带有完善的错误处理
				compressedMessages, newConsumedTokens, newCurrentTokens := rc.compressMessagesWithFallback(ctx, result.Messages, totalTokensUsed, currentTokens, iteration)

				if compressedMessages != nil {
					// 压缩成功
					result.CurrentMessageTokens = newCurrentTokens
					result.Messages = compressedMessages
					result.PromptTokens = newConsumedTokens // 更新累积 token 数
					subAgentLog("INFO", "💾 SubAgent compression successful: %d->%d messages, tokens: %d->%d",
						messageCount, len(compressedMessages), currentTokens, newCurrentTokens)

					// 发送压缩成功的流式通知
					if isStreaming {
						streamCallback(StreamChunk{
							Type:     "subagent_compression",
							Content:  fmt.Sprintf("💾 SubAgent compressed %d messages to %d (saved %d tokens)",
								messageCount, len(compressedMessages), currentTokens-newCurrentTokens),
							Metadata: map[string]any{
								"iteration":        iteration,
								"compress_reason":  compressReason,
								"original_messages": messageCount,
								"compressed_messages": len(compressedMessages),
								"tokens_saved":     currentTokens - newCurrentTokens,
							},
						})
					}
				} else {
					// 压缩失败，但继续执行
					subAgentLog("WARN", "⚠️ SubAgent compression failed at iteration %d, continuing with original messages", iteration)
				}
			} else {
				subAgentLog("DEBUG", "💾 SubAgent skipping compression at iteration %d: %d messages, %d tokens (below thresholds)",
					iteration, messageCount, currentTokens)
			}
		}

		// SubAgent 专用：在 LLM 调用前检查是否需要紧急压缩
		if rc.messageProcessor != nil {
			// 估算当前消息的 token 数
			estimatedTokens := rc.estimateMessageTokens(result.Messages)
			maxTokensLimit := execCtx.Config.MaxTokens
			if maxTokensLimit <= 0 {
				maxTokensLimit = 8000 // 默认限制
			}

			// 如果预计 token 数超过 80% 的限制，进行紧急压缩
			if estimatedTokens > int(float64(maxTokensLimit)*0.8) {
				subAgentLog("WARN", "⚠️ SubAgent emergency compression triggered: %d tokens (80%% of %d limit)", estimatedTokens, maxTokensLimit)

				// 执行紧急压缩
				totalTokensUsed := result.PromptTokens + result.CompletionTokens
				compressedMessages, newConsumedTokens, newCurrentTokens := rc.compressMessagesWithFallback(ctx, result.Messages, totalTokensUsed, result.CurrentMessageTokens, iteration)

				if compressedMessages != nil && len(compressedMessages) < len(result.Messages) {
					result.Messages = compressedMessages
					result.PromptTokens = newConsumedTokens
					result.CurrentMessageTokens = newCurrentTokens
					subAgentLog("INFO", "✅ SubAgent emergency compression successful: %d->%d messages", len(result.Messages), len(compressedMessages))

					if isStreaming {
						streamCallback(StreamChunk{
							Type:     "emergency_compression",
							Content:  fmt.Sprintf("⚡ Emergency compression: %d->%d messages to avoid context limit", len(result.Messages), len(compressedMessages)),
							Metadata: map[string]any{"iteration": iteration, "reason": "approaching_context_limit"},
						})
					}
				}
			}
		}

		// 构建LLM请求
		request := &llm.ChatRequest{
			Messages:   result.Messages,
			ModelType:  llm.BasicModel,
			Tools:      execCtx.Tools,
			ToolChoice: "auto",
			Config:     execCtx.Config,
			MaxTokens:  execCtx.Config.MaxTokens,
		}

		// 获取LLM实例
		subAgentLog("DEBUG", "🤖 Getting LLM instance for iteration %d", iteration)
		client, err := llm.GetLLMInstance(llm.BasicModel)
		if err != nil {
			subAgentLog("ERROR", "Failed to get LLM instance at iteration %d: %v", iteration, err)
			if isStreaming {
				streamCallback(StreamChunk{Type: "error", Content: fmt.Sprintf("❌ LLM initialization failed: %v", err)})
			}
			return nil, fmt.Errorf("LLM initialization failed at iteration %d: %w", iteration, err)
		}

		// 验证请求
		if err := rc.llmHandler.validateLLMRequest(request); err != nil {
			subAgentLog("ERROR", "Invalid LLM request at iteration %d: %v", iteration, err)
			if isStreaming {
				streamCallback(StreamChunk{Type: "error", Content: fmt.Sprintf("❌ Invalid request: %v", err)})
			}
			return nil, fmt.Errorf("invalid LLM request at iteration %d: %w", iteration, err)
		}

		// 执行LLM调用
		subAgentLog("INFO", "🤖 Calling LLM for iteration %d...", iteration)
		response, err := rc.llmHandler.callLLMWithRetry(ctx, client, request, 6)
		if err != nil {
			subAgentLog("ERROR", "LLM call failed at iteration %d: %v", iteration, err)
			if isStreaming {
				streamCallback(StreamChunk{Type: "error", Content: fmt.Sprintf("❌ LLM call failed: %v", err)})
			}
			return nil, fmt.Errorf("LLM call failed at iteration %d: %w", iteration, err)
		}
		subAgentLog("DEBUG", "🤖 LLM call completed for iteration %d", iteration)

		// 验证响应
		if response == nil || len(response.Choices) == 0 {
			subAgentLog("ERROR", "Invalid response at iteration %d", iteration)
			if isStreaming {
				streamCallback(StreamChunk{Type: "error", Content: "❌ Invalid response from LLM"})
			}
			return nil, fmt.Errorf("invalid response at iteration %d", iteration)
		}

		choice := response.Choices[0]
		step.Thought = strings.TrimSpace(choice.Message.Content)

		// 处理token使用情况
		usage := response.GetUsage()
		tokensUsed := usage.GetTotalTokens()
		promptTokens := usage.GetPromptTokens()
		completionTokens := usage.GetCompletionTokens()

		result.TokensUsed += tokensUsed
		result.PromptTokens += promptTokens
		result.CompletionTokens += completionTokens
		result.CurrentMessageTokens += promptTokens // 当前消息token数累加
		step.TokensUsed = tokensUsed

		// 发送token使用情况
		if isStreaming && tokensUsed > 0 {
			streamCallback(StreamChunk{
				Type:             "token_usage",
				Content:          fmt.Sprintf("Tokens used: %d (prompt: %d, completion: %d)", tokensUsed, promptTokens, completionTokens),
				TokensUsed:       tokensUsed,
				TotalTokensUsed:  result.TokensUsed,
				PromptTokens:     promptTokens,
				CompletionTokens: completionTokens,
				Metadata:         map[string]any{"iteration": iteration, "phase": "core_token_accounting"},
			})
		}

		// 发送思考结果
		if len(choice.Message.Content) > 0 && len(choice.Message.ToolCalls) > 0 {
			if isStreaming {
				streamCallback(StreamChunk{
					Type:     "thinking_result",
					Content:  choice.Message.Content,
					Metadata: map[string]any{"iteration": iteration, "phase": "core_thinking"},
				})
			}
		}

		// 添加assistant消息到对话历史
		if len(choice.Message.Content) > 0 || len(choice.Message.ToolCalls) > 0 {
			result.Messages = append(result.Messages, choice.Message)
		}

		// 解析工具调用
		subAgentLog("DEBUG", "🔍 Parsing tool calls from LLM response at iteration %d", iteration)
		toolCalls := rc.agent.parseToolCalls(&choice.Message)
		subAgentLog("INFO", "🔍 Parsed %d tool calls", len(toolCalls))

		if len(toolCalls) > 0 {
			step.Action = "tool_execution"
			step.ToolCall = toolCalls

			subAgentLogger.Info("🔧 Executing %d tool calls at iteration %d", len(toolCalls), iteration)

			// Check for multiple subagent calls - use parallel execution if detected
			subagentCalls := rc.filterSubAgentCalls(toolCalls)
			
			var toolResult []*types.ReactToolResult
			
			if len(subagentCalls) > 1 {
				subAgentLog("INFO", "🚀 Detected %d subagent calls - executing in parallel", len(subagentCalls))
				toolResult = rc.executeSubAgentsInParallel(ctx, toolCalls, subagentCalls, streamCallback)
			} else {
				// 使用统一的工具执行系统进行串行执行
				toolExecutor := utils.NewToolExecutor("SUB-AGENT")
				displayFormatter := utils.NewToolDisplayFormatter() // Default green color

				// 转换回调函数类型
				var utilsCallback utils.StreamCallback
				if streamCallback != nil {
					utilsCallback = func(chunk utils.StreamChunk) {
						// 转换 utils.StreamChunk 到 agent.StreamChunk
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

				toolResult = toolExecutor.ExecuteSerialToolsWithRecovery(
					ctx,
					toolCalls,
					rc.executeToolDirect,
					utilsCallback,
					displayFormatter.Format,
				)
			}

			step.Result = toolResult
			subAgentLogger.Info("🔧 Tool execution completed with %d results", len(toolResult))
			subAgentLogger.Debug("🔧 Tool execution completed with %+v", toolResult)

			// 构建工具消息
			if toolResult != nil {
				isGemini := strings.Contains(request.Config.BaseURL, "googleapis")
				toolMessages := rc.toolHandler.buildToolMessages(toolResult, isGemini)

				result.Messages = append(result.Messages, toolMessages...)

				step.Observation = rc.toolHandler.generateObservation(toolResult)
			}
			// 添加任务输入，重复提醒目标
			result.Messages = append(result.Messages, llm.Message{
				Role:    "user",
				Content: execCtx.Task,
			})
		} else {
			// 没有工具调用，直接返回最终答案
			finalAnswer := choice.Message.Content
			step.Action = "direct_answer"
			step.Observation = finalAnswer
			step.Duration = time.Since(step.Timestamp)

			subAgentLog("INFO", "✅ Task completed at iteration %d with direct answer", iteration)
			result.Answer = finalAnswer
			result.Success = true
			result.Confidence = 0.8
			result.History = append(result.History, step)

			if isStreaming {
				streamCallback(StreamChunk{
					Type:     "final_answer",
					Content:  finalAnswer,
					Metadata: map[string]any{"iteration": iteration, "phase": "core_final_answer"},
				})
			}
			return result, nil
		}

		step.Duration = time.Since(step.Timestamp)
		result.History = append(result.History, step)
	}

	// 达到最大迭代次数
	subAgentLog("WARN", "Maximum iterations (%d) reached", maxIterations)
	if isStreaming {
		streamCallback(StreamChunk{
			Type:     "max_iterations",
			Content:  fmt.Sprintf("⚠️ Core execution reached maximum iterations (%d)", maxIterations),
			Metadata: map[string]any{"max_iterations": maxIterations},
		})
	}

	result.Answer = "Maximum iterations reached without completion"
	result.Success = false
	result.Confidence = 0.5
	return result, nil
}

// NewTaskExecutionContext - 创建任务执行上下文的便捷函数
func (rc *ReactCore) NewTaskExecutionContext(ctx context.Context, task string, systemPrompt string, maxIter int) *TaskExecutionContext {
	taskID := generateTaskID()
	taskCtx := types.NewReactTaskContext(taskID, task)

	// 构建初始消息列表
	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
		{
			Role:    "user",
			Content: task,
		},
	}

	// 构建工具定义
	tools := rc.toolHandler.buildToolDefinitions(ctx)

	return &TaskExecutionContext{
		TaskID:         taskID,
		Task:           task,
		Messages:       messages,
		TaskCtx:        taskCtx,
		Tools:          tools,
		Config:         rc.agent.llmConfig,
		MaxIter:        maxIter,
		Session:        nil,                     // 由调用者在需要时设置
		SessionManager: rc.agent.sessionManager, // 使用ReactCore所属的session manager
	}
}

// ========== Sub-Agent Architecture Support ==========

// SubAgentInterface - Sub-agent接口，定义sub-agent的核心能力
type SubAgentInterface interface {
	// ExecuteTask - 执行独立任务，返回完成状态和结果
	ExecuteTask(ctx context.Context, task string) (*SubAgentResult, error)

	// ExecuteTaskWithStream - 执行独立任务，支持流式回调以显示工具调用过程
	ExecuteTaskWithStream(ctx context.Context, task string, streamCallback StreamCallback) (*SubAgentResult, error)

	// GetSessionID - 获取sub-agent的session ID
	GetSessionID() string

	// GetConfig - 获取sub-agent配置
	GetConfig() *SubAgentConfig
}

// SubAgentConfig - Sub-agent配置
type SubAgentConfig struct {
	SessionID     string   // 子会话ID
	MaxIterations int      // 最大迭代次数
	Tools         []string // 允许使用的工具列表
	ContextCache  bool     // 是否启用上下文缓存
}

// SubAgentResult - Sub-agent执行结果
type SubAgentResult struct {
	Success       bool   `json:"success"`
	TaskCompleted bool   `json:"task_completed"`
	Result        string `json:"result"`                  // 任务结果内容
	MaterialPath  string `json:"material_path"`           // 物料地址（如文件路径）
	SessionID     string `json:"session_id"`              // 子会话ID
	TokensUsed    int    `json:"tokens_used"`             // 使用的token数
	Duration      int64  `json:"duration_ms"`             // 执行时长（毫秒）
	ErrorMessage  string `json:"error_message,omitempty"` // 错误信息
}

// SubAgent - Sub-agent的具体实现
type SubAgent struct {
	config         *SubAgentConfig
	reactCore      *ReactCore
	sessionManager *session.Manager // 独立的session manager
	sessionID      string
}

// NewSubAgent - 创建新的sub-agent实例
func NewSubAgent(parentCore *ReactCore, config *SubAgentConfig) (*SubAgent, error) {
	// 添加全面的nil检查
	if parentCore == nil {
		return nil, fmt.Errorf("parentCore cannot be nil")
	}
	if parentCore.agent == nil {
		return nil, fmt.Errorf("parentCore.agent cannot be nil")
	}
	if parentCore.agent.llm == nil {
		return nil, fmt.Errorf("parentCore.agent.llm cannot be nil")
	}
	if parentCore.agent.configManager == nil {
		return nil, fmt.Errorf("parentCore.agent.configManager cannot be nil")
	}
	if parentCore.agent.llmConfig == nil {
		return nil, fmt.Errorf("parentCore.agent.llmConfig cannot be nil")
	}
	if config == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}

	if config.SessionID == "" {
		config.SessionID = fmt.Sprintf("sub_%s", generateTaskID())
	}

	subAgentLog("INFO", "Creating new sub-agent with session ID: %s", config.SessionID)

	// 创建独立的session manager，避免与主agent冲突
	subSessionManager, err := session.NewManager()
	if err != nil {
		subAgentLog("ERROR", "Failed to create session manager: %v", err)
		return nil, fmt.Errorf("failed to create sub-agent session manager: %w", err)
	}

	// 创建独立的工具注册器，使用sub-agent模式防止递归
	subToolRegistry := NewToolRegistryWithSubAgentMode(parentCore.agent.configManager, subSessionManager, true)

	agent := &ReactAgent{
		llm:            parentCore.agent.llm,
		configManager:  parentCore.agent.configManager,
		sessionManager: subSessionManager,
		toolRegistry:   subToolRegistry,
		config:         types.NewReactConfig(),
		llmConfig:      parentCore.agent.llmConfig,
		promptBuilder:  NewLightPromptBuilder(), // 添加缺失的promptBuilder
		messageQueue:   NewMessageQueue(),       // 初始化MessageQueue防止nil pointer
	}
	// 创建独立的ReactCore实例，避免session状态污染
	subReactCore := NewReactCore(agent, subToolRegistry)
	if subReactCore == nil {
		subAgentLog("ERROR", "Failed to create ReactCore for subagent")
		return nil, fmt.Errorf("failed to create ReactCore for subagent")
	}

	subAgentLog("INFO", "Sub-agent initialized successfully with %d tools", len(subToolRegistry.ListTools(context.Background())))

	return &SubAgent{
		config:         config,
		reactCore:      subReactCore,
		sessionManager: subSessionManager,
		sessionID:      config.SessionID,
	}, nil
}

// ExecuteTask - 实现SubAgentInterface.ExecuteTask，支持流式回调
func (sa *SubAgent) ExecuteTask(ctx context.Context, task string, streamCallback StreamCallback) (result *SubAgentResult, err error) {
	startTime := time.Now()

	// Comprehensive panic recovery for subagent execution
	defer func() {
		if r := recover(); r != nil {
			subAgentLog("ERROR", "Sub-agent task execution panicked: %v", r)
			// Create safe result on panic
			result = &SubAgentResult{
				Success:       false,
				TaskCompleted: false,
				Result:        "Task execution was interrupted by system panic but recovered safely",
				SessionID:     sa.sessionID,
				Duration:      time.Since(startTime).Milliseconds(),
				ErrorMessage:  fmt.Sprintf("panic during execution: %v", r),
			}
			err = fmt.Errorf("subagent execution panicked: %v", r)

			if streamCallback != nil {
				streamCallback(StreamChunk{
					Type:     "sub_agent_panic_recovery",
					Content:  fmt.Sprintf("⚠️ Sub-agent recovered from panic: %v", r),
					Metadata: map[string]any{"sub_agent_id": sa.sessionID, "panic_value": fmt.Sprintf("%v", r)},
				})
			}
		}
	}()

	subAgentLog("INFO", "🚀 Starting sub-agent task execution with stream callback")
	subAgentLog("INFO", "📋 Task: %s", task)
	subAgentLog("INFO", "🆔 Session ID: %s", sa.sessionID)

	// 为sub-agent创建独立的session，避免与主agent混淆
	subAgentLog("DEBUG", "📝 Creating independent session for sub-agent")
	subSession, err := sa.sessionManager.StartSession(sa.sessionID)
	if err != nil {
		subAgentLog("ERROR", "Failed to start session: %v", err)
		return &SubAgentResult{
			Success:       false,
			TaskCompleted: false,
			Result:        "",
			SessionID:     sa.sessionID,
			Duration:      time.Since(startTime).Milliseconds(),
			ErrorMessage:  fmt.Sprintf("failed to start sub-agent session: %v", err),
		}, err
	}
	subAgentLog("DEBUG", "📝 Session created successfully")

	// 准备系统提示
	systemPrompt := sa.buildDefaultSystemPrompt()

	// 创建独立的任务执行上下文
	execCtx := sa.reactCore.NewTaskExecutionContext(ctx, task, systemPrompt, sa.config.MaxIterations)

	// 设置sub-agent专用的session和session manager
	execCtx.Session = subSession
	execCtx.SessionManager = sa.sessionManager

	// 如果有工具限制，过滤工具列表
	if len(sa.config.Tools) > 0 {
		execCtx.Tools = sa.filterTools(execCtx.Tools)
	}

	// 发送子代理开始信号
	if streamCallback != nil {
		streamCallback(StreamChunk{
			Type:     "sub_agent_start",
			Content:  fmt.Sprintf("🤖 Sub-agent starting task: %s", task),
			Metadata: map[string]any{"sub_agent_id": sa.sessionID, "task": task},
		})
	}

	// 执行核心任务，传入流式回调 with robust error handling
	subAgentLog("INFO", "⚡ Executing core task with max %d iterations and stream callback", sa.config.MaxIterations)
	coreResult, err := sa.reactCore.ExecuteTaskCore(ctx, execCtx, streamCallback)
	if err != nil {
		// Enhanced error handling for context limit and other API errors
		if isContextLimitError(err) {
			subAgentLog("WARN", "⚠️ Core task hit context limits, attempting graceful degradation: %v", err)
			if streamCallback != nil {
				streamCallback(StreamChunk{
					Type:     "context_limit_warning",
					Content:  "⚠️ Task hit context limits but recovered with partial results",
					Metadata: map[string]any{"sub_agent_id": sa.sessionID, "error_type": "context_limit"},
				})
			}
			// Return partial results instead of complete failure
			return &SubAgentResult{
				Success:       false,
				TaskCompleted: false,
				Result:        "Task partially completed before hitting context limits",
				SessionID:     sa.sessionID,
				Duration:      time.Since(startTime).Milliseconds(),
				ErrorMessage:  fmt.Sprintf("context limit reached: %v", err),
			}, nil // Don't return error, let caller handle gracefully
		}

		subAgentLog("ERROR", "❌ Core task execution failed: %v", err)
		if streamCallback != nil {
			streamCallback(StreamChunk{
				Type:     "sub_agent_error",
				Content:  fmt.Sprintf("❌ Sub-agent execution failed: %v", err),
				Metadata: map[string]any{"sub_agent_id": sa.sessionID, "error": err.Error()},
			})
		}
		return &SubAgentResult{
			Success:       false,
			TaskCompleted: false,
			Result:        "",
			SessionID:     sa.sessionID,
			Duration:      time.Since(startTime).Milliseconds(),
			ErrorMessage:  err.Error(),
		}, err
	}
	subAgentLog("DEBUG", "⚡ Core task execution completed")

	// 构建sub-agent结果
	subResult := &SubAgentResult{
		Success:       coreResult.Success,
		TaskCompleted: coreResult.Success,
		Result:        coreResult.Answer,
		SessionID:     sa.sessionID,
		TokensUsed:    coreResult.TokensUsed,
		Duration:      time.Since(startTime).Milliseconds(),
	}

	// 发送子代理完成信号
	if streamCallback != nil {
		streamCallback(StreamChunk{
			Type:    "sub_agent_complete",
			Content: fmt.Sprintf("✅ Sub-agent completed: %s", subResult.Result),
			Metadata: map[string]any{
				"sub_agent_id":   sa.sessionID,
				"success":        subResult.Success,
				"task_completed": subResult.TaskCompleted,
				"tokens_used":    subResult.TokensUsed,
				"duration":       subResult.Duration,
			},
		})
	}

	// 如果任务失败，设置错误信息
	if !coreResult.Success {
		subResult.ErrorMessage = "Task execution did not complete successfully"
		subAgentLog("WARN", "Task execution unsuccessful after %dms", subResult.Duration)
	} else {
		subAgentLog("INFO", "Task completed successfully in %dms, tokens used: %d",
			subResult.Duration, subResult.TokensUsed)
	}

	return subResult, nil
}

// GetSessionID - 实现SubAgentInterface.GetSessionID
func (sa *SubAgent) GetSessionID() string {
	return sa.sessionID
}

// GetConfig - 实现SubAgentInterface.GetConfig
func (sa *SubAgent) GetConfig() *SubAgentConfig {
	return sa.config
}

// buildDefaultSystemPrompt - 构建默认的sub-agent系统提示（基于主agent但移除递归子代理能力）
func (sa *SubAgent) buildDefaultSystemPrompt() string {
	return `You are a specialized sub-agent with coding assistant capabilities and product thinking mindset. You investigate problems before writing code and create practical, testable solutions.

## Context
- **Working as Sub-Agent**: You are executing a specific task assigned by the main agent
- **Task Focus**: Complete the assigned task efficiently and report results clearly
- **No Sub-Agents**: You cannot create or launch other sub-agents (this capability is disabled)

# Core Principles
- **Act Immediately**: Start working without asking questions
- **Test Everything**: Every task must have verifiable completion criteria
- **Investigate First**: Research user needs and available tools
- **Use Tools Together**: Run multiple tools at once when possible
- **Keep Answers Short**: 1-4 lines unless user wants more detail
- **Write Good Code**: Focus on security, speed, and easy maintenance
- **Large Files**: Split files >10000 chars into segments (multiple file_edit calls)

# Research Strategy

**INVESTIGATE FIRST** (before any coding):
- **User Workflow**: How will people actually use this?
- **Industry Patterns**: What do successful projects do?
- **Available Tools**: What libraries and frameworks exist?
- **Competition**: How do other products solve this?
- **Testing Requirements**: How will we verify this works?

**DESIGN CRITERIA** (every feature must meet):
- **User Value**: Solves a real problem
- **Business Goals**: Helps achieve objectives  
- **Testability**: Can be verified/measured
- **Scalability**: Works with more users
- **Maintainability**: Easy to maintain and extend

# Tool Usage & File Handling

**PARALLEL EXECUTION**: Run multiple tools together:
` + "`" + `
// Research: file_read(docs/) + web_search("patterns") + grep_search("examples")
// Verify: file_read(src/) + file_list() + bash("test command")
` + "`" + `

**LARGE FILES (>10000 chars)**: Use segmented writing:
` + "`" + `
1. Plan: Break into logical 2000-5000 char segments
2. Write: file_edit(path, "", segment1)           // Create with first part
3. Append: file_edit(path, marker1, segment2)     // Add second part  
4. Continue: file_edit(path, marker2, segment3)   // Add remaining parts
5. Test: file_read(path) + validation commands
` + "`" + `

**SEGMENT BOUNDARIES** (for appending):
- Functions: ` + "`" + `}\n\n` + "`" + ` | Classes: ` + "`" + `}\n\n` + "`" + ` | Sections: unique closing tags

# WORKFLOW

## Sub-Agent Process (focused task execution):

1. **UNDERSTAND**: Analyze the assigned task and its requirements
2. **RESEARCH**: Investigate domain + technical + business context if needed
3. **PLAN**: Design solution with testing criteria + user value
4. **TODO**: Break into specific, testable tasks (if complex)
5. **EXECUTE**: Build + test each task immediately
6. **VERIFY**: Confirm complete solution works
7. **REPORT**: Provide clear results to the main agent

## Task Testing Requirements:

**EVERY TASK** must include verification step:
- **Code**: Run/compile + check functionality
- **Files**: Read result + verify content/structure
- **Config**: Test settings work correctly
- **Docs**: Check readability + accuracy
- **Large Files (>10000 chars)**: Use segmented writing + final verification

## TODO Standards:
- **Specific**: Clear, actionable with test criteria
- **Testable**: Each task has verification method
- **Sequential**: Complete + test before next task
- **Complete**: Mark done only after successful verification

# Communication & Examples

**STYLE**: Direct answers, 1-4 lines max. Avoid "Here is...", "Let me...", "I'll help..."

**SIMPLE TASKS**:
` + "`" + `
User: 2 + 2
Assistant: 4

User: Hello  
Assistant: Hi! What specific task should I complete?
` + "`" + `

**COMPLEX TASKS** (with testing):
` + "`" + `
User: Build authentication system
Assistant: [web_search("auth best practices") + file_read(existing_auth) + grep_search("security")]
[todo_update: 1.Research patterns+test requirements 2.Design flow+security tests 3.Implement JWT 4.Add OAuth 5.Test auth flow 6.Test security 7.Deploy+verify]
JWT + OAuth2 recommended. Testing plan included...
` + "`" + `

# Sub-Agent Specific Guidelines

- **Focus on assigned task**: Complete the specific task given by the main agent
- **Report results clearly**: Provide concrete, actionable results
- **No recursive sub-agents**: You cannot create additional sub-agents
- **Work autonomously**: Use all available tools except sub-agent creation
- **Be efficient**: Complete tasks quickly and thoroughly
- **Test thoroughly**: Verify all work before reporting completion

You should work autonomously within your task scope and provide concrete results that the main agent can use.`
}

// estimateMessageTokens - 快速估算消息的 token 数量
func (rc *ReactCore) estimateMessageTokens(messages []llm.Message) int {
	totalTokens := 0
	for _, msg := range messages {
		// 简单的字符数估算：通常 1 token ≈ 4 characters
		contentLength := len(msg.Content)
		estimatedTokens := contentLength / 4

		// 为工具调用添加额外的 token 估算
		if len(msg.ToolCalls) > 0 {
			for _, tc := range msg.ToolCalls {
				estimatedTokens += len(tc.Function.Name)/4 + len(tc.Function.Arguments)/4 + 10 // 额外开销
			}
		}

		totalTokens += estimatedTokens
	}

	// 添加消息结构开销
	totalTokens += len(messages) * 10 // 每条消息大约 10 token 的结构开销

	return totalTokens
}

// compressMessagesWithFallback - SubAgent 专用的消息压缩，带有降级策略
func (rc *ReactCore) compressMessagesWithFallback(ctx context.Context, messages []llm.Message, consumedTokens, currentTokens, iteration int) ([]llm.Message, int, int) {
	// 首先尝试正常压缩
	compressedMessages, newConsumedTokens, newCurrentTokens := rc.messageProcessor.CompressMessages(ctx, messages, consumedTokens, currentTokens)

	// 检查压缩是否成功（压缩后消息数量应该减少）
	if len(compressedMessages) > 0 && len(compressedMessages) < len(messages) {
		subAgentLog("DEBUG", "✅ SubAgent compression successful via normal method")
		return compressedMessages, newConsumedTokens, newCurrentTokens
	}

	// 如果正常压缩失败，尝试简单的历史裁剪策略
	subAgentLog("WARN", "⚠️ Normal compression failed, trying fallback message truncation")

	if len(messages) <= 3 {
		// 消息太少，无法裁剪
		subAgentLog("WARN", "Too few messages for fallback compression, keeping original")
		return messages, consumedTokens, currentTokens
	}

	// 保留系统消息和最近的消息，删除中间的消息
	fallbackMessages := make([]llm.Message, 0, len(messages))

	// 保留前2条系统消息
	systemMsgCount := 0
	for i, msg := range messages {
		if msg.Role == "system" && systemMsgCount < 2 {
			fallbackMessages = append(fallbackMessages, msg)
			systemMsgCount++
		} else if msg.Role != "system" {
			// 只保留最后3条非系统消息
			if i >= len(messages)-3 {
				fallbackMessages = append(fallbackMessages, msg)
			}
		}
	}

	// 添加一条说明消息
	fallbackMessages = append(fallbackMessages, llm.Message{
		Role:    "assistant",
		Content: fmt.Sprintf("📝 [SubAgent Note: Message history was truncated at iteration %d to manage context length. Previous conversation context may be lost.]", iteration),
	})

	subAgentLog("INFO", "📝 SubAgent fallback compression: %d->%d messages", len(messages), len(fallbackMessages))

	// 重新计算 token 数（简单估算）
	newCurrentTokens = currentTokens / 3 // 粗略估算压缩后的 token 数

	return fallbackMessages, consumedTokens + currentTokens/2, newCurrentTokens
}

// isSubAgentContext - Check if execution context is for a SubAgent
func isSubAgentContext(execCtx *TaskExecutionContext) bool {
	if execCtx == nil || execCtx.TaskID == "" {
		return false
	}
	// SubAgent TaskIDs typically start with "sub_" prefix
	return strings.HasPrefix(execCtx.TaskID, "sub_") ||
		(execCtx.Session != nil && strings.Contains(execCtx.Session.ID, "sub"))
}


// filterTools - 根据配置过滤可用工具
func (sa *SubAgent) filterTools(allTools []llm.Tool) []llm.Tool {
	var filteredTools []llm.Tool

	// 始终过滤掉sub_agent工具，防止无限递归
	for _, tool := range allTools {
		if tool.Function.Name == "sub_agent" {
			subAgentLog("DEBUG", "Filtered out sub_agent tool to prevent recursion")
			continue
		}
		filteredTools = append(filteredTools, tool)
	}

	// 如果指定了允许的工具列表，进一步过滤
	if len(sa.config.Tools) > 0 {
		allowedTools := make(map[string]bool)
		for _, toolName := range sa.config.Tools {
			// 确保sub_agent不在允许列表中
			if toolName != "sub_agent" {
				allowedTools[toolName] = true
			}
		}

		var finalTools []llm.Tool
		for _, tool := range filteredTools {
			if allowedTools[tool.Function.Name] {
				finalTools = append(finalTools, tool)
			}
		}
		return finalTools
	}

	return filteredTools
}

// ========== Tool Integration for Sub-Agent ==========

// ExecuteSubAgentTask - 作为工具调用的sub-agent包装器
// 这个函数可以被注册为一个工具，供主agent调用
func (rc *ReactCore) ExecuteSubAgentTask(ctx context.Context, args map[string]interface{}) (interface{}, error) {
	// 解析参数
	task, ok := args["task"].(string)
	if !ok {
		return nil, fmt.Errorf("task parameter is required and must be a string")
	}

	// 可选参数
	maxIter := 50 // 默认值
	if iter, exists := args["max_iterations"]; exists {
		if iterInt, ok := iter.(int); ok {
			maxIter = iterInt
		}
	}

	var allowedTools []string
	if tools, exists := args["allowed_tools"]; exists {
		if toolsSlice, ok := tools.([]interface{}); ok {
			for _, tool := range toolsSlice {
				if toolStr, ok := tool.(string); ok {
					allowedTools = append(allowedTools, toolStr)
				}
			}
		}
	}

	// 创建sub-agent配置
	config := &SubAgentConfig{
		MaxIterations: maxIter,
		Tools:         allowedTools,
		ContextCache:  true, // 默认启用上下文缓存
	}

	// 创建并执行sub-agent
	subAgent, err := NewSubAgent(rc, config)
	if err != nil {
		return nil, fmt.Errorf("failed to create sub-agent: %w", err)
	}

	// 使用安全的回调函数 - 如果ReactCore的streamCallback为nil，使用nil
	// ExecuteTask内部会处理nil streamCallback的情况
	var safeCallback StreamCallback
	if rc.streamCallback != nil {
		safeCallback = rc.streamCallback
	}

	return subAgent.ExecuteTask(ctx, task, safeCallback)
}
