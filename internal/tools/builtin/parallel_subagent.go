package builtin

import (
	"context"
	"fmt"
	"strings"
	"time"
)

// ParallelSubAgentExecutor - Interface to avoid circular dependencies
type ParallelSubAgentExecutor interface {
	ExecuteTasksParallel(ctx context.Context, args map[string]interface{}) (interface{}, error)
}

// ParallelSubAgentTool - Tool for executing multiple tasks in parallel
type ParallelSubAgentTool struct {
	executor ParallelSubAgentExecutor
}

// CreateParallelSubAgentTool - Factory function for the parallel subagent tool
func CreateParallelSubAgentTool(executor ParallelSubAgentExecutor) Tool {
	return &ParallelSubAgentTool{
		executor: executor,
	}
}

// Name returns the tool name
func (psat *ParallelSubAgentTool) Name() string {
	return "parallel_subagent"
}

// Description returns the tool description
func (psat *ParallelSubAgentTool) Description() string {
	return "Execute multiple tasks in parallel using subagents with automatic result ordering and efficient resource management. Ideal for independent tasks that can benefit from concurrent execution."
}

// Parameters returns the tool parameters schema
func (psat *ParallelSubAgentTool) Parameters() map[string]interface{} {
	return map[string]interface{}{
		"type": "object",
		"properties": map[string]interface{}{
			"tasks": map[string]interface{}{
				"type": "array",
				"items": map[string]interface{}{
					"type": "string",
				},
				"description": "Array of task strings to execute in parallel",
				"minItems":    1,
				"maxItems":    50,
			},
			"max_workers": map[string]interface{}{
				"type":        "integer",
				"description": "Maximum number of parallel workers (1-10, default: 3)",
				"minimum":     1,
				"maximum":     10,
				"default":     3,
			},
			"task_timeout": map[string]interface{}{
				"type":        "string",
				"description": "Timeout for individual tasks (e.g., '2m', '30s', default: '2m')",
				"default":     "2m",
			},
			"enable_streaming": map[string]interface{}{
				"type":        "boolean",
				"description": "Enable real-time streaming of task execution (default: true)",
				"default":     true,
			},
		},
		"required": []string{"tasks"},
	}
}

// Execute runs the parallel subagent tool
func (psat *ParallelSubAgentTool) Execute(ctx context.Context, args map[string]interface{}) (*ToolResult, error) {
	// Validate that we have an executor
	if psat.executor == nil {
		return &ToolResult{
			Content: "❌ Parallel subagent executor not available",
		}, fmt.Errorf("parallel subagent executor not configured")
	}

	// Parse and validate arguments
	parsedArgs, err := psat.parseArguments(args)
	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("❌ Invalid arguments: %v", err),
		}, err
	}

	// Execute via the executor interface
	result, err := psat.executor.ExecuteTasksParallel(ctx, map[string]interface{}{
		"tasks":            parsedArgs.Tasks,
		"max_workers":      parsedArgs.MaxWorkers,
		"task_timeout":     parsedArgs.TaskTimeout.String(),
		"enable_streaming": parsedArgs.EnableStreaming,
	})

	if err != nil {
		return &ToolResult{
			Content: fmt.Sprintf("❌ Parallel execution failed: %v", err),
		}, err
	}

	// Format the result
	return psat.formatExecutionResult(parsedArgs.Tasks, result)
}

// Validate checks if the provided arguments are valid for this tool
func (psat *ParallelSubAgentTool) Validate(args map[string]interface{}) error {
	_, err := psat.parseArguments(args)
	return err
}

// ParsedArguments - Structured arguments for the tool
type ParsedArguments struct {
	Tasks           []string      `json:"tasks"`
	MaxWorkers      int           `json:"max_workers"`
	TaskTimeout     time.Duration `json:"task_timeout"`
	EnableStreaming bool          `json:"enable_streaming"`
}

// parseArguments - Parse and validate tool arguments
func (psat *ParallelSubAgentTool) parseArguments(args map[string]interface{}) (*ParsedArguments, error) {
	parsed := &ParsedArguments{
		MaxWorkers:      3,                // Default
		TaskTimeout:     2 * time.Minute, // Default
		EnableStreaming: true,             // Default
	}

	// Parse tasks (required)
	tasksRaw, ok := args["tasks"]
	if !ok {
		return nil, fmt.Errorf("tasks parameter is required")
	}

	tasksSlice, ok := tasksRaw.([]interface{})
	if !ok {
		return nil, fmt.Errorf("tasks must be an array")
	}

	if len(tasksSlice) == 0 {
		return nil, fmt.Errorf("tasks array cannot be empty")
	}

	if len(tasksSlice) > 50 {
		return nil, fmt.Errorf("tasks array cannot exceed 50 items, got %d", len(tasksSlice))
	}

	parsed.Tasks = make([]string, len(tasksSlice))
	for i, task := range tasksSlice {
		taskStr, ok := task.(string)
		if !ok {
			return nil, fmt.Errorf("task %d must be a string", i)
		}
		if strings.TrimSpace(taskStr) == "" {
			return nil, fmt.Errorf("task %d cannot be empty", i)
		}
		parsed.Tasks[i] = strings.TrimSpace(taskStr)
	}

	// Parse max_workers (optional)
	if maxWorkersRaw, exists := args["max_workers"]; exists {
		if maxWorkersFloat, ok := maxWorkersRaw.(float64); ok {
			maxWorkers := int(maxWorkersFloat)
			if maxWorkers < 1 || maxWorkers > 10 {
				return nil, fmt.Errorf("max_workers must be between 1 and 10, got %d", maxWorkers)
			}
			parsed.MaxWorkers = maxWorkers
		} else {
			return nil, fmt.Errorf("max_workers must be a number")
		}
	}

	// Parse task_timeout (optional)
	if taskTimeoutRaw, exists := args["task_timeout"]; exists {
		if timeoutStr, ok := taskTimeoutRaw.(string); ok {
			timeout, err := time.ParseDuration(timeoutStr)
			if err != nil {
				return nil, fmt.Errorf("invalid task_timeout format: %v", err)
			}
			if timeout <= 0 {
				return nil, fmt.Errorf("task_timeout must be positive")
			}
			parsed.TaskTimeout = timeout
		} else {
			return nil, fmt.Errorf("task_timeout must be a duration string (e.g., '2m', '30s')")
		}
	}

	// Parse enable_streaming (optional)
	if enableStreamingRaw, exists := args["enable_streaming"]; exists {
		if enableStreaming, ok := enableStreamingRaw.(bool); ok {
			parsed.EnableStreaming = enableStreaming
		} else {
			return nil, fmt.Errorf("enable_streaming must be a boolean")
		}
	}

	return parsed, nil
}

// formatExecutionResult - Format execution results for display
func (psat *ParallelSubAgentTool) formatExecutionResult(tasks []string, result interface{}) (*ToolResult, error) {
	// The result format will depend on what the executor returns
	// For now, create a basic successful result
	var output strings.Builder
	
	output.WriteString("## Parallel Task Execution Results\n\n")
	output.WriteString(fmt.Sprintf("✅ Successfully executed %d tasks in parallel\n\n", len(tasks)))
	
	// List the tasks that were executed
	output.WriteString("### Executed Tasks:\n")
	for i, task := range tasks {
		output.WriteString(fmt.Sprintf("%d. %s\n", i+1, task))
	}

	return &ToolResult{
		Content: output.String(),
		Data: map[string]interface{}{
			"task_count": len(tasks),
			"raw_result": result,
		},
		Metadata: map[string]interface{}{
			"execution_type": "parallel",
			"tool_name":      "parallel_subagent",
		},
	}, nil
}