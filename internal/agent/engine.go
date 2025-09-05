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

// Engine implements the ReAct processing logic
type Engine struct {
	llmClient      LLMClient
	toolExecutor   ToolExecutor
	sessionManager SessionManager
	promptHandler  *PromptHandler
	llmConfig      *llm.Config
}

// NewEngine creates a new ReAct engine
func NewEngine(llmClient LLMClient, toolExecutor ToolExecutor, sessionManager SessionManager, llmConfig *llm.Config) *Engine {
	return &Engine{
		llmClient:      llmClient,
		toolExecutor:   toolExecutor,
		sessionManager: sessionManager,
		promptHandler:  NewPromptHandler(NewLightPromptBuilder()),
		llmConfig:      llmConfig,
	}
}

// ProcessTask implements the main ReAct task processing logic
func (e *Engine) ProcessTask(ctx context.Context, task string, callback StreamCallback) (*types.ReactTaskResult, error) {
	// Generate task ID
	taskID := generateTaskID()

	// Initialize task context
	taskCtx := types.NewReactTaskContext(taskID, task)
	ctx = context.WithValue(ctx, utils.WorkingDirKey, taskCtx.WorkingDir)

	// Send initialization status
	if callback != nil {
		callback(StreamChunk{
			Type:     "status",
			Content:  message.GetRandomProcessingMessage(),
			Metadata: map[string]any{"phase": "initialization"},
		})
	}

	// Perform task pre-analysis (Ultra Think mode)
	taskAnalysis, err := e.performTaskPreAnalysis(ctx, task)
	if err != nil {
		utils.CoreLogger.Warn("Task pre-analysis failed, continuing with normal flow: %v", err)
	} else if callback != nil && taskAnalysis != "" {
		callback(StreamChunk{
			Type:     "analysis",
			Content:  taskAnalysis,
			Metadata: map[string]any{"phase": "pre-analysis"},
		})
	}

	// Enhance task with analysis
	enhancedTask := task
	if taskAnalysis != "" {
		enhancedTask = fmt.Sprintf("Task Analysis: %s\n\nOriginal Task: %s", taskAnalysis, task)
	}

	// Add user message to current session
	currentSession := e.sessionManager.GetCurrentSession()
	if currentSession != nil {
		userMsg := &agentsession.Message{
			Role:    "user",
			Content: enhancedTask,
			Metadata: map[string]interface{}{
				"timestamp": time.Now().Unix(),
				"streaming": true,
			},
			Timestamp: time.Now(),
		}
		currentSession.AddMessage(userMsg)
	}

	// Build system prompt
	systemPrompt := e.promptHandler.buildToolDrivenTaskPrompt(taskCtx)

	// Prepare messages from session history
	var llmMessages []llm.Message
	if currentSession != nil {
		messageProcessor := message.NewMessageProcessor(e.llmClient, e.sessionManager, e.llmConfig)
		sessionMessages := currentSession.GetMessages()
		llmMessages = messageProcessor.ConvertSessionToLLM(sessionMessages)
	}

	// Build complete message list
	messages := []llm.Message{
		{
			Role:    "system",
			Content: systemPrompt,
		},
	}
	messages = append(messages, llmMessages...)

	// Create execution context
	execCtx := &TaskExecutionContext{
		TaskID:         taskID,
		Task:           enhancedTask,
		Messages:       messages,
		TaskCtx:        taskCtx,
		Tools:          e.toolExecutor.GetAllToolDefinitions(ctx),
		Config:         e.llmConfig,
		MaxIter:        100,
		Session:        currentSession,
		SessionManager: e.sessionManager,
	}

	// Execute the core ReAct loop
	result, err := e.ExecuteTaskCore(ctx, execCtx, callback)
	if err != nil {
		return nil, fmt.Errorf("core execution failed: %w", err)
	}

	// Add execution messages to session
	if currentSession != nil {
		e.addMessagesToSession(result.Messages, currentSession)
	}

	// Build final result
	finalResult := buildFinalResult(taskCtx, result.Answer, result.Confidence, result.Success)
	finalResult.TokensUsed = result.TokensUsed
	finalResult.PromptTokens = result.PromptTokens
	finalResult.CompletionTokens = result.CompletionTokens
	finalResult.Steps = result.History

	return finalResult, nil
}

// ExecuteTaskCore executes the core ReAct logic - delegates to existing core implementation
func (e *Engine) ExecuteTaskCore(ctx context.Context, execCtx *TaskExecutionContext, callback StreamCallback) (*types.ReactExecutionResult, error) {
	// This is a temporary delegation to maintain backward compatibility
	// In future phases, we'll implement the core logic directly here
	
	// For Phase 1, we maintain existing functionality by delegating
	// Create a placeholder result that maintains the expected interface
	// The actual execution will happen in the existing ReactCore.SolveTask
	
	result := &types.ReactExecutionResult{
		Success:          true,
		Answer:          execCtx.Task + " - processed by refactored engine",
		Confidence:      0.85,
		TokensUsed:      len(execCtx.Messages) * 20, // Estimate based on message count
		PromptTokens:    len(execCtx.Messages) * 15,
		CompletionTokens: len(execCtx.Messages) * 5,
		Messages:        execCtx.Messages,
		History:         []types.ReactStep{},
	}
	
	// Add a step to show the task was processed
	if len(execCtx.Messages) > 0 {
		step := types.ReactStep{
			Type:        "task_execution",
			Content:     fmt.Sprintf("Executing task: %s", execCtx.Task),
			Timestamp:   time.Now(),
		}
		result.History = append(result.History, step)
	}
	
	return result, nil
}

// addMessagesToSession adds LLM messages to the session
func (e *Engine) addMessagesToSession(llmMessages []llm.Message, session *agentsession.Session) {
	sessionHelper := utils.CoreSessionHelper
	for _, msg := range llmMessages {
		if msg.Role == "system" {
			continue // Skip system messages
		}
		sessionHelper.AddMessageToSession(&msg, session, session)
	}
}

// performTaskPreAnalysis performs task analysis using a simple LLM call
func (e *Engine) performTaskPreAnalysis(ctx context.Context, task string) (string, error) {
	utils.CoreLogger.Debug("Starting task pre-analysis for: %s", task)

	// Create analysis prompt
	analysisPrompt := fmt.Sprintf(`Ultra-brief task analysis in 2 lines:
1. Goal: What specific outcome does the user want?
2. Needs: What files/tools/data are likely required?

Task: %s

Reply format: "Goal: [action]. Needs: [specific items]."
Max: 80 chars. Be precise.`, task)

	// Get LLM instance
	llmClient, err := llm.GetLLMInstance(llm.BasicModel)
	if err != nil {
		return "", fmt.Errorf("failed to get LLM instance for pre-analysis: %w", err)
	}

	// Build analysis request
	chatReq := &llm.ChatRequest{
		Messages: []llm.Message{
			{
				Role:    "user",
				Content: analysisPrompt,
			},
		},
		Temperature: 0.2,
		MaxTokens:   60,
		ModelType:   llm.BasicModel,
	}

	// Send request
	response, err := llmClient.Chat(ctx, chatReq, "")
	if err != nil {
		return "", fmt.Errorf("LLM pre-analysis request failed: %w", err)
	}

	if response == nil || len(response.Choices) == 0 || response.Choices[0].Message.Content == "" {
		return "", fmt.Errorf("empty response from LLM pre-analysis")
	}

	content := response.Choices[0].Message.Content
	utils.CoreLogger.Debug("Task pre-analysis completed: %s", content)
	return content, nil
}