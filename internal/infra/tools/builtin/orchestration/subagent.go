package orchestration

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	appcontext "alex/internal/app/agent/context"
	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	tools "alex/internal/domain/agent/ports/tools"
	"alex/internal/domain/workflow"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/async"
	id "alex/internal/shared/utils/id"
)

// TaskExecutor is the consumer-side interface for delegating task execution.
// Defined here rather than in a central ports package following Go's
// "accept interfaces, return structs" idiom — the consumer owns the contract.
type TaskExecutor interface {
	ExecuteTask(ctx context.Context, task string, sessionID string, listener agent.EventListener) (*agent.TaskResult, error)
}

// subagent implements parallel task delegation via the coordinator interface
// ARCHITECTURE: This tool properly follows hexagonal architecture by:
// - NOT importing domain layer (no internal/agent/domain)
// - NOT importing output layer (no internal/output)
// - Delegating all execution to the TaskExecutor interface (consumer-defined)
// - Using appcontext.MarkSubagentContext to trigger registry filtering (RECURSION PREVENTION)
type subagent struct {
	shared.BaseTool
	coordinator  TaskExecutor
	maxWorkers   int
	startStagger time.Duration
}

// NewSubAgent creates a subagent tool with coordinator injection.
// The coordinator must implement ExecuteTask for task delegation.
func NewSubAgent(coordinator TaskExecutor, maxWorkers int) tools.ToolExecutor {
	return &subagent{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "subagent",
				Description: `Delegate complex work to dedicated subagents. Provide a single prompt or a list of tasks to run in parallel/serial. The subagent inherits the current tool access mode/preset and cannot spawn other subagents.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"prompt": {
							Type:        "string",
							Description: "Single task to delegate when tasks are not provided.",
						},
						"tasks": {
							Type:        "array",
							Description: "Optional list of subtasks to delegate to subagents.",
							Items:       &ports.Property{Type: "string"},
						},
						"mode": {
							Type:        "string",
							Description: "Execution mode for multiple tasks (parallel or serial).",
							Enum:        []any{"parallel", "serial"},
						},
						"max_parallel": {
							Type:        "integer",
							Description: "Maximum number of subtasks to run concurrently when mode=parallel.",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "subagent",
				Version:  "2.0.0",
				Category: "agent",
				Tags:     []string{"delegation", "orchestration"},
			},
		),
		coordinator:  coordinator,
		maxWorkers:   maxWorkers,
		startStagger: 25 * time.Millisecond,
	}
}

func (t *subagent) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "prompt", "tasks", "mode", "max_parallel":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	prompt := ""
	if raw, ok := call.Arguments["prompt"]; ok {
		promptStr, ok := raw.(string)
		if !ok {
			return shared.ToolError(call.ID, "prompt must be a string")
		}
		prompt = strings.TrimSpace(promptStr)
	}

	tasks, err := parseStringList(call.Arguments, "tasks")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	mode, err := parseSubagentMode(call.Arguments, len(tasks))
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	maxParallel, err := parseOptionalPositiveInt(call.Arguments, "max_parallel")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if len(tasks) == 0 {
		if prompt == "" {
			return shared.ToolError(call.ID, "missing required parameter: prompt")
		}
		tasks = []string{prompt}
	}

	if mode == "" {
		if len(tasks) > 1 {
			mode = "parallel"
		} else {
			mode = "single"
		}
	}
	if len(tasks) <= 1 && mode == "parallel" {
		mode = "single"
	}

	sharedAttachments, sharedIterations := tools.GetAttachmentContext(ctx)

	// CRITICAL: Mark context as subagent execution
	// This triggers ExecutionPreparationService to use filtered registry (without subagent tool)
	// Prevents infinite recursion
	ctx = appcontext.MarkSubagentContext(ctx)

	// Propagate causal chain: set causation_id to the subagent tool call ID
	// so every subtask knows which call spawned it.
	ctx = id.WithCausationID(ctx, call.ID)

	// Extract parent listener from context if available.
	parentListener := shared.GetParentListenerFromContext(ctx)

	results := make([]SubtaskResult, len(tasks))
	collector := newAttachmentCollector(sharedAttachments)
	effectiveMaxParallel := maxParallel
	if effectiveMaxParallel <= 0 && t.maxWorkers > 0 {
		effectiveMaxParallel = t.maxWorkers
	}
	parallelism := resolveParallelism(mode, len(tasks), effectiveMaxParallel)
	if parallelism <= 1 {
		for i, task := range tasks {
			if err := ctx.Err(); err != nil {
				for pending := i; pending < len(tasks); pending++ {
					results[pending] = SubtaskResult{
						Index: pending,
						Task:  tasks[pending],
						Error: err,
					}
				}
				break
			}
			results[i] = t.executeSubtask(ctx, task, i, len(tasks), parentListener, parallelism, sharedAttachments, sharedIterations, collector)
		}
	} else {
		jobs := make(chan int)
		processed := make([]bool, len(tasks))
		var wg sync.WaitGroup
		wg.Add(parallelism)

		for i := 0; i < parallelism; i++ {
			async.Go(nil, "subagent.parallel", func() {
				defer wg.Done()
				for {
					select {
					case <-ctx.Done():
						return
					case idx, ok := <-jobs:
						if !ok {
							return
						}
						results[idx] = t.executeSubtask(ctx, tasks[idx], idx, len(tasks), parentListener, parallelism, sharedAttachments, sharedIterations, collector)
						processed[idx] = true
					}
				}
			})
		}

		stopQueue := false
		for i := range tasks {
			select {
			case <-ctx.Done():
				stopQueue = true
			case jobs <- i:
			}
			if stopQueue {
				break
			}
			if t.startStagger > 0 && i < len(tasks)-1 {
				timer := time.NewTimer(t.startStagger)
				select {
				case <-ctx.Done():
					timer.Stop()
					stopQueue = true
				case <-timer.C:
				}
				if stopQueue {
					break
				}
			}
		}
		close(jobs)
		wg.Wait()
		if err := ctx.Err(); err != nil {
			for i := range tasks {
				if processed[i] {
					continue
				}
				results[i] = SubtaskResult{
					Index: i,
					Task:  tasks[i],
					Error: err,
				}
			}
		}
	}

	if len(tasks) == 1 && results[0].Error != nil {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Subagent execution failed: %v", results[0].Error),
			Error:   results[0].Error,
		}, nil
	}

	// Format results
	formatted, err := t.formatResults(call, tasks, results, mode)
	if err != nil {
		return nil, err
	}
	if attachments := collector.Snapshot(); len(attachments) > 0 {
		formatted.Attachments = attachments
	}
	return formatted, nil
}

func parseSubagentMode(args map[string]any, taskCount int) (string, error) {
	raw, exists := args["mode"]
	if !exists || raw == nil {
		if taskCount > 1 {
			return "parallel", nil
		}
		return "", nil
	}
	mode, ok := raw.(string)
	if !ok {
		return "", fmt.Errorf("mode must be a string")
	}
	normalized := strings.ToLower(strings.TrimSpace(mode))
	switch normalized {
	case "parallel", "serial":
		return normalized, nil
	case "":
		if taskCount > 1 {
			return "parallel", nil
		}
		return "", nil
	default:
		return "", fmt.Errorf("unsupported mode: %s", mode)
	}
}

func parseOptionalPositiveInt(args map[string]any, key string) (int, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return 0, nil
	}
	switch v := raw.(type) {
	case int:
		if v < 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return v, nil
	case float64:
		if v < 0 {
			return 0, fmt.Errorf("%s must be a positive integer", key)
		}
		return int(v), nil
	default:
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
}

func resolveParallelism(mode string, taskCount int, maxParallel int) int {
	if mode == "serial" || taskCount <= 1 {
		return 1
	}
	if mode == "" || mode == "parallel" {
		parallelism := taskCount
		if maxParallel > 0 && maxParallel < parallelism {
			parallelism = maxParallel
		}
		if parallelism < 1 {
			return 1
		}
		return parallelism
	}
	return 1
}

// SubtaskResult holds the result of a single subtask execution
type SubtaskResult struct {
	Index      int                        `json:"index"`
	Task       string                     `json:"task"`
	Answer     string                     `json:"answer,omitempty"`
	Iterations int                        `json:"iterations,omitempty"`
	TokensUsed int                        `json:"tokens_used,omitempty"`
	Workflow   *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	LogID      string                     `json:"log_id,omitempty"`
	Error      error                      `json:"error,omitempty"`
}
