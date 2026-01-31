package orchestration

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"alex/internal/agent/ports"
	agent "alex/internal/agent/ports/agent"
	"alex/internal/tools/builtin/shared"
)

var slugRe = regexp.MustCompile(`[^a-z0-9]+`)

type bgDispatch struct {
	shared.BaseTool
}

// NewBGDispatch creates the bg_dispatch tool for launching background tasks.
func NewBGDispatch() *bgDispatch {
	return &bgDispatch{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "bg_dispatch",
				Description: `Dispatch a task to run in the background. The task executes asynchronously while you continue working. Use bg_status to check progress and bg_collect to retrieve results.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"task_id": {
							Type:        "string",
							Description: "Optional unique identifier for this background task. Auto-generated from description if omitted.",
						},
						"description": {
							Type:        "string",
							Description: "A short human-readable description of the task.",
						},
						"prompt": {
							Type:        "string",
							Description: "The full task prompt to execute in the background.",
						},
						"agent_type": {
							Type:        "string",
							Description: `Agent type to use. "internal" (default) uses the built-in subagent. External types include "claude_code" and "codex".`,
						},
						"config": {
							Type:        "object",
							Description: "Optional per-task config overrides (string map) passed to the external agent executor.",
						},
						"depends_on": {
							Type:        "array",
							Description: "Task IDs that must complete successfully before this task starts.",
							Items:       &ports.Property{Type: "string"},
						},
						"workspace_mode": {
							Type:        "string",
							Description: `Workspace isolation mode: "shared" (default), "branch", or "worktree".`,
						},
						"file_scope": {
							Type:        "array",
							Description: "Advisory file scope for this task (paths or directories).",
							Items:       &ports.Property{Type: "string"},
						},
						"inherit_context": {
							Type:        "boolean",
							Description: "Whether to prepend completed dependency results to the task prompt.",
						},
					},
					Required: []string{"description", "prompt"},
				},
			},
			ports.ToolMetadata{
				Name:     "bg_dispatch",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "async"},
			},
		),
	}
}

func (t *bgDispatch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	// Validate parameters.
	for key := range call.Arguments {
		switch key {
		case "task_id", "description", "prompt", "agent_type", "config", "depends_on", "workspace_mode", "file_scope", "inherit_context":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	description, errResult := shared.RequireStringArg(call.Arguments, call.ID, "description")
	if errResult != nil {
		return errResult, nil
	}

	// task_id is optional; auto-generate from description slug or call.ID.
	taskID := strings.TrimSpace(shared.StringArg(call.Arguments, "task_id"))
	if taskID == "" {
		taskID = slugFromDescription(description)
	}
	if taskID == "" {
		taskID = "bg-" + call.ID
	}

	prompt, errResult := shared.RequireStringArg(call.Arguments, call.ID, "prompt")
	if errResult != nil {
		return errResult, nil
	}

	agentType := "internal"
	if raw, ok := call.Arguments["agent_type"]; ok {
		if str, ok := raw.(string); ok && strings.TrimSpace(str) != "" {
			agentType = strings.TrimSpace(str)
		}
	}
	configOverrides, err := parseStringMap(call.Arguments, "config")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	dependsOn, err := parseStringList(call.Arguments, "depends_on")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	fileScope, err := parseStringList(call.Arguments, "file_scope")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	inheritContext, _, err := parseOptionalBool(call.Arguments, "inherit_context")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	workspaceMode := ""
	if raw, ok := call.Arguments["workspace_mode"]; ok {
		if str, ok := raw.(string); ok {
			workspaceMode = strings.TrimSpace(str)
		}
	}

	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	req := agent.BackgroundDispatchRequest{
		TaskID:         taskID,
		Description:    description,
		Prompt:         prompt,
		AgentType:      agentType,
		CausationID:    call.ID,
		Config:         configOverrides,
		DependsOn:      dependsOn,
		WorkspaceMode:  agent.WorkspaceMode(workspaceMode),
		FileScope:      fileScope,
		InheritContext: inheritContext,
	}
	if err := dispatcher.Dispatch(ctx, req); err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	content := fmt.Sprintf("Background task %q dispatched successfully. Use bg_status(task_ids=[\"%s\"]) to check progress.", taskID, taskID)
	if len(fileScope) > 0 {
		conflicts := detectScopeConflicts(fileScope, dispatcher.Status(nil), taskID)
		if len(conflicts) > 0 {
			var sb strings.Builder
			sb.WriteString(content)
			sb.WriteString("\n\n⚠ Scope overlap detected:\n")
			for _, conflict := range conflicts {
				sb.WriteString(fmt.Sprintf("  Task %q overlaps on: %s\n", conflict.TaskID, strings.Join(conflict.OverlapPaths, ", ")))
			}
			content = sb.String()
		}
	}

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"task_id": taskID,
		},
	}, nil
}

type scopeConflict struct {
	TaskID       string
	OverlapPaths []string
}

func detectScopeConflicts(scope []string, summaries []agent.BackgroundTaskSummary, newTaskID string) []scopeConflict {
	var conflicts []scopeConflict
	for _, summary := range summaries {
		if summary.ID == newTaskID {
			continue
		}
		if summary.Status != agent.BackgroundTaskStatusRunning && summary.Status != agent.BackgroundTaskStatusBlocked {
			continue
		}
		if len(summary.FileScope) == 0 {
			continue
		}
		overlap := overlapPaths(scope, summary.FileScope)
		if len(overlap) > 0 {
			conflicts = append(conflicts, scopeConflict{
				TaskID:       summary.ID,
				OverlapPaths: overlap,
			})
		}
	}
	return conflicts
}

func overlapPaths(a, b []string) []string {
	var overlap []string
	for _, pathA := range a {
		for _, pathB := range b {
			if strings.HasPrefix(pathA, pathB) || strings.HasPrefix(pathB, pathA) {
				overlap = append(overlap, pathA)
				break
			}
		}
	}
	return dedupe(overlap)
}

func dedupe(items []string) []string {
	seen := map[string]struct{}{}
	var out []string
	for _, item := range items {
		if _, ok := seen[item]; ok {
			continue
		}
		seen[item] = struct{}{}
		out = append(out, item)
	}
	return out
}

// slugFromDescription generates a slug from a description string.
// Lowercase, non-alphanumeric sequences replaced with hyphens, truncated to 32 chars.
func slugFromDescription(desc string) string {
	slug := strings.ToLower(strings.TrimSpace(desc))
	slug = slugRe.ReplaceAllString(slug, "-")
	slug = strings.Trim(slug, "-")
	if len(slug) > 32 {
		slug = slug[:32]
		slug = strings.TrimRight(slug, "-")
	}
	return slug
}

