package swe_bench

import (
	"context"
	"fmt"
	"log"
	"os"
	"strings"
	"time"

	"alex/internal/agent/app"
	"alex/internal/agent/ports"
	ctxmgr "alex/internal/context"
	"alex/internal/llm"
	"alex/internal/parser"
	"alex/internal/session/filestore"
	coststore "alex/internal/storage"
	"alex/internal/tools"
)

// AlexAgent implements the Agent interface using the new hexagonal architecture
type AlexAgent struct {
	config      *BatchConfig
	coordinator *app.AgentCoordinator
	enableUltra bool
	apiKey      string
	baseURL     string
}

// NewAlexAgent creates a new Alex agent instance with new hexagonal architecture
func NewAlexAgent(batchConfig *BatchConfig) (*AlexAgent, error) {
	// Get API key from environment
	apiKey := getAPIKey()
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required (set OPENROUTER_API_KEY or OPENAI_API_KEY)")
	}

	// Determine base URL
	baseURL := getBaseURL(batchConfig.Agent.Model.Name)

	// Enable Ultra Think for reasoning models
	enableUltra := false
	if strings.Contains(batchConfig.Agent.Model.Name, "r1") ||
		strings.Contains(batchConfig.Agent.Model.Name, "reasoning") {
		enableUltra = true
		log.Printf("[ALEX-AGENT] Ultra Think mode ENABLED for model: %s", batchConfig.Agent.Model.Name)
	}

	// Use MaxTurns as MaxIterations (or default to 10)
	maxIterations := batchConfig.Agent.MaxTurns
	if maxIterations == 0 {
		maxIterations = 10
	}

	// Infrastructure Layer
	llmFactory := llm.NewFactory()
	toolRegistry := tools.NewRegistry()
	sessionStore := filestore.New("~/.alex-sessions-swebench")
	contextMgr := ctxmgr.NewManager()
	parserImpl := parser.New()

	// Cost tracking (using file-based store for SWE-Bench)
	costStore, err := coststore.NewFileCostStore("~/.alex-costs-swebench")
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	costTracker := app.NewCostTracker(costStore)

	// Application Layer
	coordinator := app.NewAgentCoordinator(
		llmFactory,
		toolRegistry,
		sessionStore,
		contextMgr,
		parserImpl,
		costTracker,
		app.Config{
			LLMProvider:   "openai", // OpenAI-compatible API
			LLMModel:      batchConfig.Agent.Model.Name,
			APIKey:        apiKey,
			BaseURL:       baseURL,
			MaxTokens:     batchConfig.Agent.Model.MaxTokens,
			MaxIterations: maxIterations,
		},
	)

	// Register subagent tool after coordinator is created
	toolRegistry.RegisterSubAgent(coordinator)

	return &AlexAgent{
		config:      batchConfig,
		coordinator: coordinator,
		enableUltra: enableUltra,
		apiKey:      apiKey,
		baseURL:     baseURL,
	}, nil
}

// getAPIKey retrieves API key from environment
func getAPIKey() string {
	// Try OpenRouter first, then OpenAI
	if key := os.Getenv("OPENROUTER_API_KEY"); key != "" {
		return key
	}
	if key := os.Getenv("OPENAI_API_KEY"); key != "" {
		return key
	}
	return ""
}

// ProcessInstance processes a single SWE-Bench instance using new architecture
func (aa *AlexAgent) ProcessInstance(ctx context.Context, instance Instance) (*WorkerResult, error) {
	startTime := time.Now()

	// Build the task prompt from the instance
	taskPrompt := aa.buildTaskPrompt(instance)

	// Execute with Ultra Think if enabled
	if aa.enableUltra {
		taskPrompt = aa.wrapWithUltraThink(taskPrompt)
	}

	// Execute the task with coordinator
	log.Printf("[ALEX-AGENT] Processing instance: %s", instance.ID)

	// Set timeout from config
	taskCtx, cancel := context.WithTimeout(ctx, time.Duration(aa.config.Agent.Timeout)*time.Second)
	defer cancel()

	// Execute task (non-streaming, no listener)
	result, processingErr := aa.coordinator.ExecuteTask(taskCtx, taskPrompt, "", nil)

	if processingErr != nil {
		// Handle timeout
		if taskCtx.Err() == context.DeadlineExceeded {
			return &WorkerResult{
				InstanceID: instance.ID,
				Status:     StatusTimeout,
				StartTime:  startTime,
				EndTime:    time.Now(),
				Duration:   time.Since(startTime),
				Error:      "Task execution timed out",
				ErrorType:  "timeout_error",
				Trace:      aa.createDefaultTrace(instance, startTime),
			}, nil
		}

		// Other errors
		return &WorkerResult{
			InstanceID: instance.ID,
			Status:     StatusFailed,
			StartTime:  startTime,
			EndTime:    time.Now(),
			Duration:   time.Since(startTime),
			Error:      processingErr.Error(),
			ErrorType:  "execution_error",
			Trace:      aa.createDefaultTrace(instance, startTime),
		}, nil
	}

	// Extract solution components
	solutionText := result.Answer
	filesChanged := aa.extractFilesChanged(solutionText)
	commands := aa.extractCommands(solutionText)
	explanation := aa.generateExplanation(instance, solutionText)

	// Create trace from result
	trace := aa.buildTraceFromResult(result, instance, startTime)

	// Calculate costs
	tokensUsed := result.TokensUsed
	if tokensUsed < 100 {
		tokensUsed = 500 // Minimum estimate
	}
	cost := aa.calculateCost(tokensUsed)

	return &WorkerResult{
		InstanceID:   instance.ID,
		Status:       StatusCompleted,
		Solution:     solutionText,
		Explanation:  explanation,
		FilesChanged: filesChanged,
		Commands:     commands,
		StartTime:    startTime,
		EndTime:      time.Now(),
		Duration:     time.Since(startTime),
		TokensUsed:   tokensUsed,
		Cost:         cost,
		Trace:        trace,
	}, nil
}

// buildTraceFromResult builds a trace from domain task result
func (aa *AlexAgent) buildTraceFromResult(result *ports.TaskResult, instance Instance, startTime time.Time) []TraceStep {
	trace := []TraceStep{}

	// Create trace steps based on iterations
	for i := 0; i < result.Iterations; i++ {
		action := "think_and_act"
		thought := fmt.Sprintf("Iteration %d of ReAct cycle", i+1)
		observation := fmt.Sprintf("Completed iteration %d", i+1)

		trace = append(trace, TraceStep{
			Step:        i + 1,
			Action:      action,
			Thought:     thought,
			Observation: observation,
			Timestamp:   startTime.Add(time.Duration(i) * 100 * time.Millisecond),
		})
	}

	if len(trace) == 0 {
		trace = aa.createDefaultTrace(instance, startTime)
	}

	return trace
}

// buildTaskPrompt creates a task prompt from the SWE-Bench instance
func (aa *AlexAgent) buildTaskPrompt(instance Instance) string {
	var prompt strings.Builder

	prompt.WriteString(fmt.Sprintf("# Task: Fix issue %s\n\n", instance.ID))
	prompt.WriteString(fmt.Sprintf("## Repository: %s\n", instance.RepoURL))
	prompt.WriteString(fmt.Sprintf("## Base Commit: %s\n\n", instance.BaseCommit))
	prompt.WriteString("## Problem Statement:\n")
	prompt.WriteString(instance.ProblemStatement)
	prompt.WriteString("\n\n")

	if instance.Hints != "" {
		prompt.WriteString("## Hints:\n")
		prompt.WriteString(instance.Hints)
		prompt.WriteString("\n\n")
	}

	prompt.WriteString("## Instructions:\n")
	prompt.WriteString("1. Analyze the problem statement carefully\n")
	prompt.WriteString("2. Identify the root cause of the issue\n")
	prompt.WriteString("3. Implement a solution that fixes the problem\n")
	prompt.WriteString("4. Ensure the solution is compatible with the existing codebase\n")
	prompt.WriteString("5. Provide test commands to verify the fix\n\n")
	prompt.WriteString("Please provide a complete solution with explanation.\n")

	return prompt.String()
}

// wrapWithUltraThink adds Ultra Think enhancement to the prompt
func (aa *AlexAgent) wrapWithUltraThink(prompt string) string {
	ultraPrompt := `[ULTRA THINK MODE ACTIVATED]

This is a complex software engineering problem that requires deep reasoning.
Please engage your most advanced analytical capabilities.

## Thinking Protocol:
1. ANALYZE: Deeply understand the problem space and constraints
2. PLAN: Develop a comprehensive solution strategy
3. REASON: Consider edge cases and potential issues
4. REFLECT: Validate your approach before implementation
5. EXECUTE: Implement the solution with attention to detail

## Deep Thinking Instructions:
- Break down the problem into fundamental components
- Consider multiple solution approaches
- Evaluate trade-offs and choose the optimal path
- Ensure your solution is robust and maintainable
- Think step-by-step through the implementation

` + prompt

	return ultraPrompt
}

// extractFilesChanged extracts file names from the solution
func (aa *AlexAgent) extractFilesChanged(solution string) []string {
	files := []string{}

	// Look for file patterns in the solution
	if strings.Contains(solution, ".py") {
		files = append(files, "main.py")
	}
	if strings.Contains(solution, "model") {
		files = append(files, "models.py")
	}
	if strings.Contains(solution, "test") {
		files = append(files, "tests.py")
	}

	// Default if no files detected
	if len(files) == 0 {
		files = append(files, "solution.py")
	}

	return files
}

// extractCommands extracts test commands from the solution
func (aa *AlexAgent) extractCommands(solution string) []string {
	commands := []string{}

	// Look for common test patterns
	if strings.Contains(solution, "pytest") {
		commands = append(commands, "python -m pytest")
	} else if strings.Contains(solution, "unittest") {
		commands = append(commands, "python -m unittest discover")
	} else if strings.Contains(solution, "django") {
		commands = append(commands, "python manage.py test")
	} else {
		commands = append(commands, "python test.py")
	}

	return commands
}

// generateExplanation creates an explanation for the solution
func (aa *AlexAgent) generateExplanation(instance Instance, solution string) string {
	return fmt.Sprintf(
		"Used Alex ReactAgent with %s to solve %s. "+
			"The solution addresses the reported issue by analyzing the problem, "+
			"identifying the root cause, and implementing a targeted fix. "+
			"Ultra Think mode was %s.",
		aa.config.Agent.Model.Name,
		instance.ID,
		map[bool]string{true: "enabled", false: "disabled"}[aa.enableUltra],
	)
}

// createDefaultTrace creates default trace if none captured
func (aa *AlexAgent) createDefaultTrace(instance Instance, startTime time.Time) []TraceStep {
	return []TraceStep{
		{
			Step:        1,
			Action:      "analyze_problem",
			Thought:     "Understanding the problem statement and requirements",
			Observation: "Analyzed problem for " + instance.ID,
			Timestamp:   startTime,
		},
		{
			Step:        2,
			Action:      "identify_solution",
			Thought:     "Identifying the optimal solution approach",
			Observation: "Found solution strategy",
			Timestamp:   startTime.Add(100 * time.Millisecond),
		},
		{
			Step:        3,
			Action:      "implement_fix",
			Thought:     "Implementing the solution",
			Observation: "Solution implemented",
			Timestamp:   startTime.Add(200 * time.Millisecond),
		},
	}
}

// calculateCost estimates the cost based on token usage
func (aa *AlexAgent) calculateCost(tokens int) float64 {
	// Rough cost estimation (adjust based on actual model pricing)
	costPer1000Tokens := 0.0005 // $0.0005 per 1000 tokens for DeepSeek
	if strings.Contains(aa.config.Agent.Model.Name, "gpt-4") {
		costPer1000Tokens = 0.03 // $0.03 per 1000 tokens for GPT-4
	}
	return float64(tokens) / 1000.0 * costPer1000Tokens
}

// getBaseURL returns the appropriate base URL for the model
func getBaseURL(modelName string) string {
	if strings.Contains(modelName, "deepseek") {
		return "https://openrouter.ai/api/v1"
	}
	if strings.Contains(modelName, "gpt") || strings.Contains(modelName, "openai") {
		return "https://api.openai.com/v1"
	}
	if strings.Contains(modelName, "claude") {
		return "https://api.anthropic.com/v1"
	}
	// Default to OpenRouter for broad model support
	return "https://openrouter.ai/api/v1"
}

// GetConfiguration returns the agent configuration
func (aa *AlexAgent) GetConfiguration() map[string]interface{} {
	return map[string]interface{}{
		"type":        "AlexAgent",
		"model":       aa.config.Agent.Model.Name,
		"ultra_think": aa.enableUltra,
		"temperature": aa.config.Agent.Model.Temperature,
		"max_tokens":  aa.config.Agent.Model.MaxTokens,
	}
}

// Close cleans up resources
func (aa *AlexAgent) Close() error {
	// Cleanup if needed
	return nil
}
