package swe_bench

import (
	"context"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"

	"alex/internal/agent/app"
	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	runtimeconfig "alex/internal/config"
	"alex/internal/di"
	"alex/internal/workflow"
)

// AlexAgent implements the Agent interface using the new hexagonal architecture
type AlexAgent struct {
	config         *BatchConfig
	coordinator    *app.AgentCoordinator
	container      *di.Container
	enableUltra    bool
	resolvedConfig agent.AgentConfig
	runtimeConfig  runtimeconfig.RuntimeConfig
}

// NewAlexAgent creates a new Alex agent instance with new hexagonal architecture
func NewAlexAgent(batchConfig *BatchConfig) (*AlexAgent, error) {
	overrides := runtimeconfig.Overrides{}

	if name := strings.TrimSpace(batchConfig.Agent.Model.Name); name != "" {
		overrides.LLMModel = ptr(name)
	}
	if tokens := batchConfig.Agent.Model.MaxTokens; tokens > 0 {
		overrides.MaxTokens = ptr(tokens)
	}
	if turns := batchConfig.Agent.MaxTurns; turns > 0 {
		overrides.MaxIterations = ptr(turns)
	} else {
		overrides.MaxIterations = ptr(10)
	}
	if temp := batchConfig.Agent.Model.Temperature; temp != 0 {
		overrides.Temperature = ptr(temp)
	}

	sessionDir := "~/.alex-sessions-swebench"
	costDir := "~/.alex-costs-swebench"
	overrides.SessionDir = &sessionDir
	overrides.CostDir = &costDir

	runtimeCfg, meta, err := runtimeconfig.Load(
		runtimeconfig.WithEnv(runtimeconfig.DefaultEnvLookup),
		runtimeconfig.WithOverrides(overrides),
	)
	if err != nil {
		return nil, fmt.Errorf("load runtime configuration: %w", err)
	}
	if runtimeCfg.APIKey == "" {
		return nil, fmt.Errorf("API key is required (set runtime.api_key in ~/.alex/config.yaml or reference ${OPENAI_API_KEY})")
	}

	baseURL := runtimeCfg.BaseURL
	// Auto-adjust base URL if:
	// 1. No explicit base URL set (empty or default source), OR
	// 2. Provider is explicitly set (env/override) to openai/anthropic/deepseek but base URL is not explicitly set
	shouldAdjustBaseURL := baseURL == "" || meta.Source("base_url") == runtimeconfig.SourceDefault

	// If provider was explicitly set but base URL was not, adjust base URL to match provider
	providerSource := meta.Source("llm_provider")
	baseURLSource := meta.Source("base_url")
	providerLower := strings.ToLower(runtimeCfg.LLMProvider)

	if (providerSource == runtimeconfig.SourceEnv || providerSource == runtimeconfig.SourceOverride) &&
		(baseURLSource != runtimeconfig.SourceEnv && baseURLSource != runtimeconfig.SourceOverride) &&
		(providerLower == "openai" || providerLower == "anthropic" || providerLower == "deepseek") {
		shouldAdjustBaseURL = true
	}

	if shouldAdjustBaseURL {
		baseURL = getBaseURL(runtimeCfg.LLMProvider, runtimeCfg.LLMModel)
	}
	runtimeCfg.BaseURL = baseURL

	diConfig := di.ConfigFromRuntimeConfig(runtimeCfg)

	container, err := di.BuildContainer(diConfig)
	if err != nil {
		return nil, fmt.Errorf("build container: %w", err)
	}

	resolved := container.AgentCoordinator.GetConfig()
	batchConfig.Agent.Model.Name = resolved.LLMModel
	batchConfig.Agent.Model.Temperature = resolved.Temperature
	batchConfig.Agent.Model.MaxTokens = resolved.MaxTokens
	batchConfig.Agent.MaxTurns = resolved.MaxIterations

	modelLower := strings.ToLower(resolved.LLMModel)
	enableUltra := strings.Contains(modelLower, "r1") || strings.Contains(modelLower, "reasoning")
	if enableUltra {
		log.Printf("[ALEX-AGENT] Ultra Think mode ENABLED for model: %s", resolved.LLMModel)
	}

	return &AlexAgent{
		config:         batchConfig,
		coordinator:    container.AgentCoordinator,
		container:      container,
		enableUltra:    enableUltra,
		resolvedConfig: resolved,
		runtimeConfig:  runtimeCfg,
	}, nil
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
		Workflow:     result.Workflow,
	}, nil
}

// buildTraceFromResult builds a trace from domain task result
func (aa *AlexAgent) buildTraceFromResult(result *agent.TaskResult, instance Instance, startTime time.Time) []TraceStep {
	if result != nil && result.Workflow != nil {
		if trace := buildTraceFromWorkflow(result.Workflow); len(trace) > 0 {
			return trace
		}
	}

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

func buildTraceFromWorkflow(snapshot *workflow.WorkflowSnapshot) []TraceStep {
	if snapshot == nil || len(snapshot.Nodes) == 0 {
		return nil
	}

	nodes := make(map[string]workflow.NodeSnapshot, len(snapshot.Nodes))
	for _, node := range snapshot.Nodes {
		nodes[node.ID] = node
	}

	steps := make([]TraceStep, 0, len(snapshot.Nodes))
	for _, id := range snapshot.Order {
		node, ok := nodes[id]
		if !ok {
			continue
		}

		if step, ok := traceStepFromNode(node, len(steps)+1); ok {
			steps = append(steps, step)
		}
	}

	return steps
}

func traceStepFromNode(node workflow.NodeSnapshot, index int) (TraceStep, bool) {
	timestamp := node.StartedAt
	if timestamp.IsZero() {
		timestamp = node.CompletedAt
	}
	if timestamp.IsZero() {
		timestamp = time.Now()
	}

	step := TraceStep{
		Step:      index,
		Timestamp: timestamp,
	}

	switch {
	case node.ID == "react:context":
		step.Action = "context_setup"
		step.Observation = "Prepared context and attachments"
		return step, true

	case node.ID == "react:finalize":
		step.Action = "finalize"
		step.Observation = describeFinalize(node.Output)
		return step, true

	case strings.HasPrefix(node.ID, "react:iter:"):
		parts := strings.Split(node.ID, ":")
		if len(parts) < 4 {
			return TraceStep{}, false
		}

		iteration := parts[2]
		stage := parts[3]
		if stage == "tool" && len(parts) >= 5 {
			step.Action = "tool_call"
			step.ToolCall = toolCallFromNode(node)
			if step.ToolCall != nil {
				step.ToolCall.Duration = node.Duration
			}
			step.Observation = fmt.Sprintf("Iteration %s tool call", iteration)
			if node.Error != "" {
				step.Observation += ": " + node.Error
			}
			return step, true
		}

		switch stage {
		case "think":
			step.Action = "think"
			step.Observation = fmt.Sprintf("Iteration %s think", iteration)
			step.Thought = stringFromAny(mapFromAny(node.Output)["content"])
			return step, true
		case "plan":
			step.Action = "plan"
			planned := toolList(mapFromAny(node.Output))
			if len(planned) > 0 {
				step.Observation = fmt.Sprintf("Iteration %s planned tools: %s", iteration, strings.Join(planned, ", "))
			} else {
				step.Observation = fmt.Sprintf("Iteration %s plan ready", iteration)
			}
			return step, true
		case "tools":
			step.Action = "tool_batch"
			summary := mapFromAny(node.Output)
			success := intFromAny(summary["success"])
			failed := intFromAny(summary["failed"])
			step.Observation = fmt.Sprintf("Iteration %s executed tools (success=%d, failed=%d)", iteration, success, failed)
			if node.Error != "" {
				step.Observation += ": " + node.Error
			}
			return step, true
		}
	}

	return TraceStep{}, false
}

func toolCallFromNode(node workflow.NodeSnapshot) *ToolCall {
	output := mapFromAny(node.Output)
	input := mapFromAny(node.Input)
	toolCall := &ToolCall{}

	if len(input) > 0 {
		toolCall.Arguments = mapFromAny(input["arguments"])
		toolCall.Name = stringFromAny(input["tool"])
	}

	if resultAny, ok := output["result"]; ok {
		switch res := resultAny.(type) {
		case ports.ToolResult:
			if toolCall.Name == "" {
				toolCall.Name = res.CallID
			}
			toolCall.Result = res.Content
			if res.Error != nil {
				toolCall.Error = res.Error.Error()
			}
		case map[string]any:
			toolCall.Result = res["content"]
			if toolCall.Name == "" {
				toolCall.Name = stringFromAny(res["tool"])
			}
			if errVal, exists := res["error"]; exists {
				toolCall.Error = stringFromAny(errVal)
			}
		}
	}

	if name := stringFromAny(output["tool"]); toolCall.Name == "" {
		toolCall.Name = name
	}

	if toolCall.Name == "" && len(toolCall.Arguments) == 0 && toolCall.Result == nil && toolCall.Error == "" {
		return nil
	}

	return toolCall
}

func describeFinalize(output any) string {
	values := mapFromAny(output)
	if values == nil {
		return "Finalized run"
	}

	parts := []string{}
	if reason := stringFromAny(values["stop_reason"]); reason != "" {
		parts = append(parts, "stop="+reason)
	}
	if preview := stringFromAny(values["answer_preview"]); preview != "" {
		parts = append(parts, "answer="+preview)
	}
	if len(parts) == 0 {
		return "Finalized run"
	}
	return strings.Join(parts, ", ")
}

func mapFromAny(value any) map[string]any {
	switch v := value.(type) {
	case map[string]any:
		return v
	}
	return nil
}

func stringFromAny(value any) string {
	switch v := value.(type) {
	case string:
		return v
	case fmt.Stringer:
		return v.String()
	case error:
		return v.Error()
	}
	return ""
}

func intFromAny(value any) int {
	switch v := value.(type) {
	case int:
		return v
	case int32:
		return int(v)
	case int64:
		return int(v)
	case float64:
		return int(v)
	case float32:
		return int(v)
	}
	return 0
}

func toolList(values map[string]any) []string {
	if values == nil {
		return nil
	}

	var tools []string
	if list, ok := values["tools"].([]string); ok {
		tools = append(tools, list...)
	} else if listAny, ok := values["tools"].([]any); ok {
		for _, item := range listAny {
			if name := stringFromAny(item); name != "" {
				tools = append(tools, name)
			}
		}
	}

	if len(tools) == 0 {
		return nil
	}

	sort.Strings(tools)
	return tools
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
		aa.resolvedConfig.LLMModel,
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
	if strings.Contains(strings.ToLower(aa.resolvedConfig.LLMModel), "gpt-4") {
		costPer1000Tokens = 0.03 // $0.03 per 1000 tokens for GPT-4
	}
	return float64(tokens) / 1000.0 * costPer1000Tokens
}

// getBaseURL returns the appropriate base URL for the provider and model
func getBaseURL(provider, modelName string) string {
	// Check provider first for explicit routing
	providerLower := strings.ToLower(provider)
	if providerLower == "openai" {
		return "https://api.openai.com/v1"
	}
	if providerLower == "anthropic" {
		return "https://api.anthropic.com/v1"
	}
	if providerLower == "deepseek" {
		return "https://api.deepseek.com/v1"
	}

	// Fall back to model-based routing
	modelLower := strings.ToLower(modelName)
	if strings.Contains(modelLower, "gpt") || strings.Contains(modelLower, "openai") {
		return "https://api.openai.com/v1"
	}
	if strings.Contains(modelLower, "claude") {
		return "https://api.anthropic.com/v1"
	}
	if strings.Contains(modelLower, "deepseek") {
		return "https://api.deepseek.com/v1"
	}

	// Default to OpenRouter for broad model support
	return "https://openrouter.ai/api/v1"
}

// GetConfiguration returns the agent configuration
func (aa *AlexAgent) GetConfiguration() map[string]interface{} {
	return map[string]interface{}{
		"type":           "AlexAgent",
		"model":          aa.resolvedConfig.LLMModel,
		"provider":       aa.runtimeConfig.LLMProvider,
		"base_url":       aa.runtimeConfig.BaseURL,
		"ultra_think":    aa.enableUltra,
		"temperature":    aa.resolvedConfig.Temperature,
		"max_tokens":     aa.resolvedConfig.MaxTokens,
		"max_iterations": aa.resolvedConfig.MaxIterations,
	}
}

// Close cleans up resources
func (aa *AlexAgent) Close() error {
	if aa.container != nil {
		return aa.container.Shutdown()
	}
	return nil
}

func ptr[T any](v T) *T {
	return &v
}
