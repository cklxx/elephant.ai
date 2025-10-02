package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

	"alex/internal/agent/contextkeys"
	"alex/internal/agent/ports"

	"golang.org/x/sync/errgroup"
)

// subagent implements parallel task delegation via tool calling
type subagent struct {
	coordinator ports.AgentCoordinator // Injected coordinator for recursion
	maxWorkers  int
}

// NewSubAgent creates a subagent tool with coordinator injection
func NewSubAgent(coordinator ports.AgentCoordinator, maxWorkers int) ports.ToolExecutor {
	if maxWorkers <= 0 {
		maxWorkers = 3 // Default to 3 parallel workers
	}
	return &subagent{
		coordinator: coordinator,
		maxWorkers:  maxWorkers,
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

	// Execute based on mode
	var results []SubtaskResult
	var err error

	if mode == "parallel" {
		results, err = t.executeParallel(ctx, subtasks, maxWorkers)
	} else {
		results, err = t.executeSerial(ctx, subtasks)
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

// Context key for nested subagent detection
type subagentCtxKey struct{}

func isNestedSubagent(ctx context.Context) bool {
	return ctx.Value(subagentCtxKey{}) != nil
}

func markSubagentContext(ctx context.Context) context.Context {
	// Mark as subagent and enable silent mode (no event output)
	ctx = context.WithValue(ctx, subagentCtxKey{}, true)
	ctx = contextkeys.WithSilentMode(ctx)
	return ctx
}

// executeParallel runs subtasks concurrently with dynamic progress tracking
func (t *subagent) executeParallel(ctx context.Context, subtasks []string, maxWorkers int) ([]SubtaskResult, error) {
	// Mark context to prevent nested subagent calls
	ctx = markSubagentContext(ctx)

	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxWorkers)

	results := make([]SubtaskResult, len(subtasks))
	completed := 0
	totalTokens := 0
	totalToolCalls := 0
	var mu sync.Mutex

	// Print compact initial status - no dynamic updates to avoid conflicts
	fmt.Printf("\n🤖 Subagent: Running %d tasks (max %d parallel)\n", len(subtasks), maxWorkers)

	for i, task := range subtasks {
		i, task := i, task // Capture loop variables

		g.Go(func() error {
			// Execute subtask
			result, err := t.coordinator.ExecuteTask(ctx, task, "")

			mu.Lock()
			defer mu.Unlock()

			completed++

			if err != nil {
				results[i] = SubtaskResult{
					Index: i,
					Task:  task,
					Error: err,
				}
				// Show error on new line (safe for concurrent output)
				taskPreview := task
				if len(taskPreview) > 50 {
					taskPreview = taskPreview[:47] + "..."
				}
				fmt.Printf("   ❌ [%d/%d] %s: %v\n", completed, len(subtasks), taskPreview, err)
				return nil // Don't fail the whole group
			}

			// Count tool calls from the task result
			toolCalls := result.Iterations // Conservative estimate

			results[i] = SubtaskResult{
				Index:      i,
				Task:       task,
				Answer:     result.Answer,
				Iterations: result.Iterations,
				TokensUsed: result.TokensUsed,
				ToolCalls:  toolCalls,
			}

			// Update totals
			totalTokens += result.TokensUsed
			totalToolCalls += toolCalls

			// Show completion on new line (safe for concurrent output)
			fmt.Printf("   ✓ [%d/%d] Task %d | %d tokens | %d tools\n",
				completed, len(subtasks), i+1, result.TokensUsed, toolCalls)

			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	// Show final summary
	fmt.Printf("   ━━━ Completed: %d/%d tasks | Total: %d tokens, %d tool calls\n\n",
		completed, len(subtasks), totalTokens, totalToolCalls)

	return results, nil
}

// executeSerial runs subtasks sequentially
func (t *subagent) executeSerial(ctx context.Context, subtasks []string) ([]SubtaskResult, error) {
	// Mark context to prevent nested subagent calls
	ctx = markSubagentContext(ctx)

	results := make([]SubtaskResult, len(subtasks))

	for i, task := range subtasks {
		result, err := t.coordinator.ExecuteTask(ctx, task, "")

		if err != nil {
			results[i] = SubtaskResult{
				Index: i,
				Task:  task,
				Error: err,
			}
			continue
		}

		// Count tool calls from the task result
		toolCalls := result.Iterations // Conservative estimate

		results[i] = SubtaskResult{
			Index:      i,
			Task:       task,
			Answer:     result.Answer,
			Iterations: result.Iterations,
			TokensUsed: result.TokensUsed,
			ToolCalls:  toolCalls,
		}
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
