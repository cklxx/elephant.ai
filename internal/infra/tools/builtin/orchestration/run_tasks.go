package orchestration

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/taskfile"
	"alex/internal/infra/tools/builtin/shared"
	"gopkg.in/yaml.v3"
)

type runTasks struct {
	shared.BaseTool
}

// NewRunTasks creates the run_tasks tool for file-based orchestration.
func NewRunTasks() *runTasks {
	return &runTasks{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name: "run_tasks",
				Description: `Execute tasks defined in a YAML task file. The agent writes a task file using write_file, then calls run_tasks to dispatch all tasks to background agents. Status is written to a .status sidecar file readable via read_file.

Supports two modes:
- file: Read a TaskFile YAML and dispatch its tasks
- template: Use a pre-configured team template with a goal

Use wait=true for synchronous execution (blocks until all tasks complete).`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"file": {
							Type:        "string",
							Description: "Path to TaskFile YAML. Required unless template is provided.",
						},
						"template": {
							Type:        "string",
							Description: `Team template name. Pass "list" to see available templates.`,
						},
						"goal": {
							Type:        "string",
							Description: "Goal for the team template. Required when template is provided.",
						},
						"prompts": {
							Type:        "object",
							Description: "Per-role prompt overrides when using a template (role_name -> prompt).",
						},
						"wait": {
							Type:        "boolean",
							Description: "Block until all tasks complete. Default: false.",
						},
						"timeout_seconds": {
							Type:        "integer",
							Description: "Max wait time in seconds when wait=true. Default: 120.",
						},
						"task_ids": {
							Type:        "array",
							Description: "Only execute specific task IDs from the file. Omit to execute all.",
							Items:       &ports.Property{Type: "string"},
						},
					},
				},
			},
			ports.ToolMetadata{
				Name:     "run_tasks",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"orchestration", "background"},
			},
		),
	}
}

func (t *runTasks) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	dispatcher := agent.GetBackgroundDispatcher(ctx)
	if dispatcher == nil {
		return shared.ToolError(call.ID, "background task dispatch is not available in this context")
	}

	filePath, _ := call.Arguments["file"].(string)
	templateName, _ := call.Arguments["template"].(string)
	filePath = strings.TrimSpace(filePath)
	templateName = strings.TrimSpace(templateName)

	if filePath == "" && templateName == "" {
		return shared.ToolError(call.ID, "either file or template is required")
	}

	// Handle template listing.
	if strings.EqualFold(templateName, "list") {
		return t.listTemplates(ctx, call.ID)
	}

	var tf *taskfile.TaskFile
	var err error

	if templateName != "" {
		tf, err = t.resolveTemplate(ctx, call, templateName)
	} else {
		tf, err = t.loadTaskFile(filePath)
	}
	if err != nil {
		return shared.ToolError(call.ID, "%s", err)
	}

	// Filter to specific task IDs if requested.
	if raw, ok := call.Arguments["task_ids"]; ok {
		ids, parseErr := parseStringList(map[string]any{"task_ids": raw}, "task_ids")
		if parseErr != nil {
			return shared.ToolError(call.ID, "task_ids: %s", parseErr)
		}
		if len(ids) > 0 {
			tf = filterTasks(tf, ids)
			if len(tf.Tasks) == 0 {
				return shared.ToolError(call.ID, "no matching task IDs found in file")
			}
		}
	}

	wait := false
	if raw, ok := call.Arguments["wait"]; ok {
		if v, ok := raw.(bool); ok {
			wait = v
		}
	}
	timeout := 120 * time.Second
	if raw, ok := call.Arguments["timeout_seconds"]; ok {
		if v, _, err := parseOptionalInt(map[string]any{"timeout_seconds": raw}, "timeout_seconds"); err == nil && v > 0 {
			timeout = time.Duration(v) * time.Second
		}
	}

	// Determine status path.
	statusPath := statusPathForFile(filePath, tf.PlanID)

	executor := taskfile.NewExecutor(dispatcher)
	var result *taskfile.ExecuteResult

	if wait {
		result, err = executor.ExecuteAndWait(ctx, tf, call.ID, statusPath, timeout)
	} else {
		result, err = executor.Execute(ctx, tf, call.ID, statusPath)
	}
	if err != nil {
		return shared.ToolError(call.ID, "execution failed: %s", err)
	}

	return t.formatResult(call.ID, result, wait)
}

func (t *runTasks) loadTaskFile(path string) (*taskfile.TaskFile, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read task file: %w", err)
	}
	var tf taskfile.TaskFile
	if err := yaml.Unmarshal(data, &tf); err != nil {
		return nil, fmt.Errorf("parse task file: %w", err)
	}
	return &tf, nil
}

func (t *runTasks) resolveTemplate(ctx context.Context, call ports.ToolCall, templateName string) (*taskfile.TaskFile, error) {
	goal, _ := call.Arguments["goal"].(string)
	goal = strings.TrimSpace(goal)
	if goal == "" {
		return nil, fmt.Errorf("goal is required when using a template")
	}

	teams := agent.GetTeamDefinitions(ctx)
	var def *agent.TeamDefinition
	for i := range teams {
		if strings.EqualFold(teams[i].Name, templateName) {
			def = &teams[i]
			break
		}
	}
	if def == nil {
		return nil, fmt.Errorf("template %q not found. Use template=\"list\" to see available templates", templateName)
	}

	tmpl := taskfile.TeamTemplateFromDefinition(*def)

	var overrides map[string]string
	if raw, ok := call.Arguments["prompts"]; ok {
		parsed, err := parseStringMap(map[string]any{"prompts": raw}, "prompts")
		if err != nil {
			return nil, fmt.Errorf("prompts: %w", err)
		}
		overrides = parsed
	}

	return taskfile.RenderTaskFile(&tmpl, goal, overrides), nil
}

func (t *runTasks) listTemplates(ctx context.Context, callID string) (*ports.ToolResult, error) {
	teams := agent.GetTeamDefinitions(ctx)
	if len(teams) == 0 {
		return &ports.ToolResult{
			CallID:  callID,
			Content: "No team templates configured.",
		}, nil
	}

	var sb strings.Builder
	sb.WriteString("Available team templates:\n\n")
	for _, team := range teams {
		sb.WriteString(fmt.Sprintf("- **%s**: %s\n", team.Name, team.Description))
		sb.WriteString("  Roles: ")
		for j, r := range team.Roles {
			if j > 0 {
				sb.WriteString(", ")
			}
			sb.WriteString(fmt.Sprintf("%s (%s)", r.Name, r.AgentType))
		}
		sb.WriteString("\n  Stages: ")
		for j, s := range team.Stages {
			if j > 0 {
				sb.WriteString(" → ")
			}
			sb.WriteString(strings.Join(s.Roles, "+"))
		}
		sb.WriteString("\n\n")
	}
	return &ports.ToolResult{CallID: callID, Content: sb.String()}, nil
}

func (t *runTasks) formatResult(callID string, result *taskfile.ExecuteResult, waited bool) (*ports.ToolResult, error) {
	var sb strings.Builder
	if waited {
		sb.WriteString(fmt.Sprintf("All %d tasks completed.\n", len(result.TaskIDs)))
	} else {
		sb.WriteString(fmt.Sprintf("Dispatched %d tasks.\n", len(result.TaskIDs)))
	}
	sb.WriteString(fmt.Sprintf("Plan: %s\n", result.PlanID))
	sb.WriteString(fmt.Sprintf("Task IDs: %s\n", strings.Join(result.TaskIDs, ", ")))
	sb.WriteString(fmt.Sprintf("Status: %s\n", result.StatusPath))
	if !waited {
		sb.WriteString("\nUse read_file to check the status file for progress.")
	}
	return &ports.ToolResult{CallID: callID, Content: sb.String()}, nil
}

func filterTasks(tf *taskfile.TaskFile, ids []string) *taskfile.TaskFile {
	idSet := make(map[string]bool, len(ids))
	for _, id := range ids {
		idSet[id] = true
	}
	filtered := &taskfile.TaskFile{
		Version:  tf.Version,
		PlanID:   tf.PlanID,
		Defaults: tf.Defaults,
		Metadata: tf.Metadata,
	}
	for _, t := range tf.Tasks {
		if idSet[t.ID] {
			filtered.Tasks = append(filtered.Tasks, t)
		}
	}
	return filtered
}

func statusPathForFile(filePath, planID string) string {
	if filePath != "" {
		ext := filepath.Ext(filePath)
		return strings.TrimSuffix(filePath, ext) + ".status" + ext
	}
	return filepath.Join(".elephant", "tasks", planID+".status.yaml")
}
