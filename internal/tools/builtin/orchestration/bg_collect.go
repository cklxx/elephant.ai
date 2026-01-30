package orchestration

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/tools/builtin/shared"
)

type bgCollect struct {
	shared.BaseTool
}

// NewBGCollect creates the bg_collect tool for retrieving background task results.
func NewBGCollect() *bgCollect {
	return &bgCollect{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "bg_collect",
				Description: `Collect full results from background tasks. By default returns immediately with whatever status tasks are in. Set wait=true to block until tasks complete (with optional timeout).`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"task_ids": {
							Type:        "array",
							Description: "Optional list of task IDs to collect. Omit to collect all.",
							Items:       &ports.Property{Type: "string"},
						},
						"wait": {
							Type:        "boolean",
							Description: "When true, block until the requested tasks complete or timeout elapses. Default: false.",
						},
						"timeout_seconds": {
							Type:        "integer",
							Description: "Maximum seconds to wait when wait=true. Default: 30.",
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "bg_collect",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "async"},
			},
		),
	}
}

func (t *bgCollect) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "task_ids", "wait", "timeout_seconds":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	ids, err := parseStringList(call.Arguments, "task_ids")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	wait := false
	if raw, ok := call.Arguments["wait"]; ok {
		if b, ok := raw.(bool); ok {
			wait = b
		}
	}

	timeout := 30 * time.Second
	if raw, ok := call.Arguments["timeout_seconds"]; ok {
		switch v := raw.(type) {
		case float64:
			if v > 0 {
				timeout = time.Duration(v) * time.Second
			}
		case int:
			if v > 0 {
				timeout = time.Duration(v) * time.Second
			}
		}
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	results := dispatcher.Collect(ids, wait, timeout)
	if len(results) == 0 {
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: "No background tasks found.",
		}, nil
	}

	var sb strings.Builder
	completed := 0
	failed := 0
	for _, r := range results {
		switch r.Status {
		case agent.BackgroundTaskStatusCompleted:
			completed++
		case agent.BackgroundTaskStatusFailed, agent.BackgroundTaskStatusCancelled:
			failed++
		}
	}

	sb.WriteString(fmt.Sprintf("Background Task Results (%d total, %d completed, %d failed/cancelled):\n\n",
		len(results), completed, failed))

	for _, r := range results {
		sb.WriteString(fmt.Sprintf("--- Task: %s (%s) ---\n", r.ID, string(r.Status)))
		sb.WriteString(fmt.Sprintf("Description: %s\n", r.Description))
		if r.Duration > 0 {
			sb.WriteString(fmt.Sprintf("Duration: %v\n", r.Duration.Round(time.Millisecond)))
		}
		if r.Iterations > 0 {
			sb.WriteString(fmt.Sprintf("Iterations: %d, Tokens: %d\n", r.Iterations, r.TokensUsed))
		}
		if r.Error != "" {
			sb.WriteString(fmt.Sprintf("Error: %s\n", r.Error))
		}
		if r.Answer != "" {
			sb.WriteString(fmt.Sprintf("Result:\n%s\n", strings.TrimSpace(r.Answer)))
		}
		sb.WriteString("\n")
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: sb.String(),
	}, nil
}
