package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"sync"

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
		Description: `Delegate complex tasks to parallel sub-agents for concurrent execution.

Use this when a task can be broken down into independent subtasks that can run in parallel.

Parameters:
- subtasks: Array of task descriptions
- mode: "parallel" (default) or "serial" execution
- max_workers: Maximum concurrent workers (default 3)

Example:
{
  "subtasks": [
    "Analyze the authentication module",
    "Review the database schema",
    "Check API endpoints"
  ],
  "mode": "parallel"
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
	Error      error
}

// executeParallel runs subtasks concurrently
func (t *subagent) executeParallel(ctx context.Context, subtasks []string, maxWorkers int) ([]SubtaskResult, error) {
	g, ctx := errgroup.WithContext(ctx)
	g.SetLimit(maxWorkers)

	results := make([]SubtaskResult, len(subtasks))
	var mu sync.Mutex

	for i, task := range subtasks {
		i, task := i, task // Capture loop variables

		g.Go(func() error {
			result, err := t.coordinator.ExecuteTask(ctx, task, "")

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				results[i] = SubtaskResult{
					Index: i,
					Task:  task,
					Error: err,
				}
				return nil // Don't fail the whole group
			}

			results[i] = SubtaskResult{
				Index:      i,
				Task:       task,
				Answer:     result.Answer,
				Iterations: result.Iterations,
				TokensUsed: result.TokensUsed,
			}
			return nil
		})
	}

	if err := g.Wait(); err != nil {
		return nil, err
	}

	return results, nil
}

// executeSerial runs subtasks sequentially
func (t *subagent) executeSerial(ctx context.Context, subtasks []string) ([]SubtaskResult, error) {
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

		results[i] = SubtaskResult{
			Index:      i,
			Task:       task,
			Answer:     result.Answer,
			Iterations: result.Iterations,
			TokensUsed: result.TokensUsed,
		}
	}

	return results, nil
}

// formatResults formats subtask results into readable output
func (t *subagent) formatResults(callID string, subtasks []string, results []SubtaskResult, mode string) (*ports.ToolResult, error) {
	var output strings.Builder

	// Summary header
	successCount := 0
	failureCount := 0
	totalTokens := 0
	totalIterations := 0

	for _, r := range results {
		if r.Error == nil {
			successCount++
			totalTokens += r.TokensUsed
			totalIterations += r.Iterations
		} else {
			failureCount++
		}
	}

	output.WriteString(fmt.Sprintf("Subagent Execution Summary (%s mode)\n", mode))
	output.WriteString(strings.Repeat("=", 50) + "\n\n")
	output.WriteString(fmt.Sprintf("Total tasks: %d | Success: %d | Failed: %d\n", len(subtasks), successCount, failureCount))
	output.WriteString(fmt.Sprintf("Total iterations: %d | Total tokens: %d\n\n", totalIterations, totalTokens))

	// Individual results
	for _, r := range results {
		output.WriteString(fmt.Sprintf("[Task %d] %s\n", r.Index+1, r.Task))
		output.WriteString(strings.Repeat("-", 50) + "\n")

		if r.Error != nil {
			output.WriteString(fmt.Sprintf("❌ Failed: %v\n\n", r.Error))
		} else {
			output.WriteString(fmt.Sprintf("✓ Success (iterations: %d, tokens: %d)\n", r.Iterations, r.TokensUsed))
			output.WriteString(fmt.Sprintf("Answer:\n%s\n\n", r.Answer))
		}
	}

	// Metadata for programmatic access
	metadata := map[string]any{
		"mode":             mode,
		"total_tasks":      len(subtasks),
		"success_count":    successCount,
		"failure_count":    failureCount,
		"total_tokens":     totalTokens,
		"total_iterations": totalIterations,
	}

	// Add individual results to metadata
	resultsJSON, _ := json.Marshal(results)
	metadata["results"] = string(resultsJSON)

	return &ports.ToolResult{
		CallID:   callID,
		Content:  output.String(),
		Metadata: metadata,
	}, nil
}