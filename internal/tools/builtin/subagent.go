package builtin

import (
	"alex/internal/agent/types"
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"
	"time"

	"alex/internal/agent/app"
	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/output"

	"golang.org/x/sync/errgroup"
)

// subagent implements parallel task delegation via tool calling
type subagent struct {
	coordinator ports.AgentCoordinator // Injected coordinator for recursion
	maxWorkers  int
	renderer    *output.CLIRenderer // Unified renderer for output
}

// NewSubAgent creates a subagent tool with coordinator injection
func NewSubAgent(coordinator ports.AgentCoordinator, maxWorkers int) ports.ToolExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 3 // Default to 3 parallel workers
	}
	return &subagent{
		coordinator: coordinator,
		maxWorkers:  maxWorkers,
		renderer:    output.NewCLIRenderer(false), // Always use concise mode for subagent
	}
}

func (t *subagent) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "subagent",
		Version:  "1.0.0",
		Category: "agent",
		Tags:     []string{"delegation", "parallel", "orchestration"},
	}
}

func (t *subagent) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "subagent",
		Description: `Delegate ONLY COMPLEX, TIME-CONSUMING tasks to parallel sub-agents for concurrent execution.

⚠️ IMPORTANT USAGE GUIDELINES:
- ❌ DO NOT use for simple, quick tasks (file operations, single searches, basic analysis)
- ❌ DO NOT use when main agent can complete task in 1-2 iterations
- ✅ ONLY use for truly complex research requiring multiple independent investigations
- ✅ ONLY use when each subtask is substantial (>5 steps) and parallel execution saves significant time
- ✅ Each subtask should be completely independent and take >30 seconds

WHEN TO USE:
- Comprehensive research requiring multiple deep investigations (e.g., "research 5 different ML frameworks")
- Large-scale code analysis across multiple modules
- Parallel data gathering from different sources
- Complex comparative analysis requiring separate detailed studies

WHEN NOT TO USE (use direct tools instead):
- Simple file operations
- Single web searches or file reads
- Quick analysis or summaries
- Tasks completable in <5 tool calls
- Sequential tasks with dependencies

Parameters:
- subtasks: Array of COMPLEX, INDEPENDENT task descriptions
- mode: "parallel" (default) or "serial" execution
- max_workers: Maximum concurrent workers (default 3)

Example (GOOD - truly complex parallel research):
{
  "subtasks": [
    "Comprehensive analysis of React 18 features, best practices, and migration guide",
    "Complete Vue 3 Composition API research with real-world examples",
    "In-depth Svelte framework study including compiler and reactivity model"
  ],
  "mode": "parallel"
}

Example (BAD - use direct tools instead):
{
  "subtasks": [
    "Read README.md",        # ❌ Use file_read directly
    "List project files",    # ❌ Use list_files directly
    "Search for 'main'"      # ❌ Use grep directly
  ]
}`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"subtasks": {
					Type:        "array",
					Description: "Array of independent task descriptions to execute",
				},
				"mode": {
					Type:        "string",
					Description: "Execution mode: 'parallel' or 'serial'",
					Enum:        []any{"parallel", "serial"},
				},
				"max_workers": {
					Type:        "integer",
					Description: "Maximum concurrent workers (only for parallel mode)",
				},
			},
			Required: []string{"subtasks"},
		},
	}
}

func (t *subagent) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Check for nested subagent calls (prevent recursion)
	if isNestedSubagent(ctx) {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: Subagent cannot call subagent recursively. Use direct tools instead.",
			Error:   fmt.Errorf("recursive subagent call not allowed"),
		}, nil
	}

	// Parse subtasks
	subtasksArg, ok := call.Arguments["subtasks"]
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: subtasks parameter required",
			Error:   fmt.Errorf("missing subtasks"),
		}, nil
	}

	subtasksArray, ok := subtasksArg.([]any)
	if !ok {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: subtasks must be an array",
			Error:   fmt.Errorf("invalid subtasks type"),
		}, nil
	}

	if len(subtasksArray) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "Error: at least one subtask required",
			Error:   fmt.Errorf("empty subtasks"),
		}, nil
	}

	// Parse mode
	mode := "parallel"
	if modeArg, ok := call.Arguments["mode"].(string); ok {
		mode = modeArg
	}

	// Parse max_workers
	maxWorkers := t.maxWorkers
	if mwArg, ok := call.Arguments["max_workers"].(float64); ok {
		maxWorkers = int(mwArg)
		if maxWorkers < 1 {
			maxWorkers = 1
		}
		if maxWorkers > 10 {
			maxWorkers = 10 // Cap at 10
		}
	}

	// Convert to string array
	subtasks := make([]string, len(subtasksArray))
	for i, st := range subtasksArray {
		if s, ok := st.(string); ok {
			subtasks[i] = s
		} else {
			subtasks[i] = fmt.Sprintf("%v", st)
		}
	}

	// Get parent listener from context (if any)
	var parentListener domain.EventListener
	if listenerVal := ctx.Value(parentListenerKey{}); listenerVal != nil {
		parentListener = listenerVal.(domain.EventListener)
	}

	// Execute based on mode
	var results []SubtaskResult
	var err error

	if mode == "parallel" {
		results, err = t.executeParallel(ctx, subtasks, maxWorkers, parentListener)
	} else {
		results, err = t.executeSerial(ctx, subtasks, parentListener)
	}

	if err != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Subagent execution failed: %v", err),
			Error:   err,
		}, nil
	}

	// Format results
	return t.formatResults(call.ID, subtasks, results, mode)
}

// SubtaskResult holds the result of a single subtask
type SubtaskResult struct {
	Index      int
	Task       string
	Answer     string
	Iterations int
	TokensUsed int
	ToolCalls  int // Number of tool calls made
	Error      error
}

// executionEnv holds all dependencies needed to execute a subtask
type executionEnv struct {
	llmClient    ports.LLMClient
	toolRegistry ports.ToolRegistry
	parser       ports.FunctionCallParser
	contextMgr   ports.ContextManager
	systemPrompt string
	maxIters     int
}

// prepareExecutionEnv gets all dependencies from coordinator
func (t *subagent) prepareExecutionEnv() (*executionEnv, error) {
	llmClient, err := t.coordinator.GetLLMClient()
	if err != nil {
		return nil, fmt.Errorf("failed to get LLM client: %w", err)
	}

	config := t.coordinator.GetConfig()
	maxIters := config.(app.Config).MaxIterations

	return &executionEnv{
		llmClient:    llmClient.(ports.LLMClient),
		toolRegistry: t.coordinator.GetToolRegistryWithoutSubagent(),
		parser:       t.coordinator.GetParser().(ports.FunctionCallParser),
		contextMgr:   t.coordinator.GetContextManager().(ports.ContextManager),
		systemPrompt: t.coordinator.GetSystemPrompt(),
		maxIters:     maxIters,
	}, nil
}

// executeSubtask executes a single subtask and returns the result
func (t *subagent) executeSubtask(ctx context.Context, task string, index int, totalTasks int, parentListener domain.EventListener) SubtaskResult {
	// Create task preview (max 60 chars)
	taskPreview := task
	if len(taskPreview) > 60 {
		taskPreview = taskPreview[:57] + "..."
	}

	// Create listener for this subtask that forwards to parent
	listener := &subagentListener{
		taskIndex:      index,
		totalTasks:     totalTasks,
		taskPreview:    taskPreview,
		parentListener: parentListener,
	}

	// Get execution environment
	env, err := t.prepareExecutionEnv()
	if err != nil {
		return SubtaskResult{Index: index, Task: task, Error: err}
	}

	// Create state
	state := &domain.TaskState{
		Messages:     []domain.Message{{Role: "system", Content: env.systemPrompt}},
		SystemPrompt: env.systemPrompt,
	}

	// Create services
	services := domain.Services{
		LLM:          env.llmClient,
		ToolExecutor: env.toolRegistry,
		Parser:       env.parser,
		Context:      env.contextMgr,
	}

	// Create ReactEngine with listener and execute
	reactEngine := domain.NewReactEngine(env.maxIters)
	reactEngine.SetEventListener(listener)
	result, err := reactEngine.SolveTask(ctx, task, state, services)

	if err != nil {
		return SubtaskResult{
			Index: index,
			Task:  task,
			Error: err,
		}
	}

	return SubtaskResult{
		Index:      index,
		Task:       task,
		Answer:     result.Answer,
		Iterations: result.Iterations,
		TokensUsed: result.TokensUsed,
		ToolCalls:  listener.getToolCallCount(),
	}
}

// Context key for nested subagent detection
type subagentCtxKey struct{}

// Context key for parent listener
type parentListenerKey struct{}

// WithParentListener adds a parent listener to context for subagent event forwarding
func WithParentListener(ctx context.Context, listener domain.EventListener) context.Context {
	return context.WithValue(ctx, parentListenerKey{}, listener)
}

// SubtaskEvent wraps domain events with subtask context
type SubtaskEvent struct {
	OriginalEvent  domain.AgentEvent
	SubtaskIndex   int    // 0-based subtask index
	TotalSubtasks  int    // Total number of subtasks
	SubtaskPreview string // Short preview of the subtask (for display)
}

// Implement domain.AgentEvent interface
func (e *SubtaskEvent) EventType() string {
	return "subtask_" + e.OriginalEvent.EventType()
}

func (e *SubtaskEvent) Timestamp() time.Time {
	return e.OriginalEvent.Timestamp()
}

func (e *SubtaskEvent) GetAgentLevel() types.AgentLevel {
	return e.OriginalEvent.GetAgentLevel()
}

func (e *SubtaskEvent) GetSessionID() string {
	return e.OriginalEvent.GetSessionID()
}

// subagentListener tracks progress and forwards events to parent
type subagentListener struct {
	taskIndex      int                  // Which subtask this is (0-based)
	totalTasks     int                  // Total number of subtasks
	taskPreview    string               // Short preview of the task
	toolCallCount  int                  // Number of tools executed
	parentListener domain.EventListener // Parent listener to forward events to
	mu             sync.Mutex
}

func (l *subagentListener) OnEvent(event domain.AgentEvent) {
	// Count tool calls
	if _, ok := event.(*domain.ToolCallCompleteEvent); ok {
		l.mu.Lock()
		l.toolCallCount++
		l.mu.Unlock()
	}

	// Forward event to parent listener wrapped with subtask context
	if l.parentListener != nil {
		// Wrap the event with subtask information
		wrappedEvent := &SubtaskEvent{
			OriginalEvent:  event,
			SubtaskIndex:   l.taskIndex,
			TotalSubtasks:  l.totalTasks,
			SubtaskPreview: l.taskPreview,
		}
		l.parentListener.OnEvent(wrappedEvent)
	}
}

func (l *subagentListener) getToolCallCount() int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.toolCallCount
}

func isNestedSubagent(ctx context.Context) bool {
	return ctx.Value(subagentCtxKey{}) != nil
}

func markSubagentContext(ctx context.Context) context.Context {
	// Mark as subagent - output layer will decide what to show
	ctx = context.WithValue(ctx, subagentCtxKey{}, true)

	// Add subagent output context - events will carry this level
	outCtx := &types.OutputContext{
		Level:   types.LevelSubagent,
		AgentID: "subagent",
		Verbose: false,
	}
	ctx = types.WithOutputContext(ctx, outCtx)

	return ctx
}

// executeParallel runs subtasks concurrently with dynamic progress tracking
func (t *subagent) executeParallel(ctx context.Context, subtasks []string, maxWorkers int, parentListener domain.EventListener) ([]SubtaskResult, error) {
	// Mark context to prevent nested subagent calls
	ctx = markSubagentContext(ctx)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxWorkers)

	results := make([]SubtaskResult, len(subtasks))
	completed := 0
	totalTokens := 0
	totalToolCalls := 0
	var mu sync.Mutex

	// Display header is now handled by event listener in stream_output.go

	for i, task := range subtasks {
		i, task := i, task // Capture loop variables

		g.Go(func() error {
			// Execute subtask
			result := t.executeSubtask(ctx, task, i, len(subtasks), parentListener)

			mu.Lock()
			defer mu.Unlock()

			completed++
			results[i] = result

			// Handle errors
			if result.Error != nil {
				// Error events already forwarded via listener
				return nil // Don't fail the whole group
			}

			// Update totals
			totalTokens += result.TokensUsed
			totalToolCalls += result.ToolCalls

			// Task completion is now shown via event system in stream_output.go

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Count successes and failures
	success, failed := 0, 0
	for _, r := range results {
		if r.Error != nil {
			failed++
		} else {
			success++
		}
	}

	// Show final summary
	outCtx := types.GetOutputContext(ctx)
	if outCtx == nil {
		outCtx = &types.OutputContext{
			Level:   types.LevelSubagent,
			AgentID: "subagent",
			Verbose: false,
		}
	}
	rendered := t.renderer.RenderSubagentComplete(outCtx, len(subtasks), success, failed, totalTokens, totalToolCalls)
	fmt.Print(rendered)

	return results, nil
}

// executeSerial runs subtasks sequentially
func (t *subagent) executeSerial(ctx context.Context, subtasks []string, parentListener domain.EventListener) ([]SubtaskResult, error) {
	// Mark context to prevent nested subagent calls
	ctx = markSubagentContext(ctx)

	results := make([]SubtaskResult, len(subtasks))

	for i, task := range subtasks {
		results[i] = t.executeSubtask(ctx, task, i, len(subtasks), parentListener)
	}

	return results, nil
}

// formatResults formats subtask results for LLM (concise) and metadata (detailed)
func (t *subagent) formatResults(callID string, subtasks []string, results []SubtaskResult, mode string) (*ports.ToolResult, error) {
	var output strings.Builder

	// Calculate summary statistics
	successCount := 0
	failureCount := 0
	totalTokens := 0
	totalIterations := 0
	totalToolCalls := 0

	for _, r := range results {
		if r.Error == nil {
			successCount++
			totalTokens += r.TokensUsed
			totalIterations += r.Iterations
			totalToolCalls += r.ToolCalls
		} else {
			failureCount++
		}
	}

	// Concise output for LLM - just the answers
	output.WriteString(fmt.Sprintf("Subagent completed %d/%d tasks (%s mode)\n\n", successCount, len(subtasks), mode))

	for _, r := range results {
		if r.Error != nil {
			output.WriteString(fmt.Sprintf("Task %d failed: %v\n\n", r.Index+1, r.Error))
		} else {
			output.WriteString(fmt.Sprintf("Task %d result:\n%s\n\n", r.Index+1, strings.TrimSpace(r.Answer)))
		}
	}

	// Metadata for programmatic access (full details for user display)
	metadata := map[string]any{
		"mode":             mode,
		"total_tasks":      len(subtasks),
		"success_count":    successCount,
		"failure_count":    failureCount,
		"total_tokens":     totalTokens,
		"total_iterations": totalIterations,
		"total_tool_calls": totalToolCalls,
	}

	// Add individual results to metadata (full answers included)
	resultsJSON, _ := json.Marshal(results)
	metadata["results"] = string(resultsJSON)

	return &ports.ToolResult{
		CallID:   callID,
		Content:  output.String(),
		Metadata: metadata,
	}, nil
}
