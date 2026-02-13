package orchestration

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/infra/tools/builtin/shared"
	"alex/internal/shared/utils/id"
)

type bgPlan struct {
	shared.BaseTool
}

type bgPlanDefaults struct {
	AgentType     string
	ExecutionMode string
	AutonomyLevel string
	WorkspaceMode string
	Config        map[string]string
}

type bgPlanTask struct {
	ID             string
	Description    string
	Prompt         string
	AgentType      string
	ExecutionMode  string
	AutonomyLevel  string
	Config         map[string]string
	DependsOn      []string
	WorkspaceMode  agent.WorkspaceMode
	FileScope      []string
	InheritContext bool
}

// NewBGPlan creates the bg_plan tool for multi-task DAG planning and optional dispatch.
func NewBGPlan() *bgPlan {
	return &bgPlan{
		BaseTool: shared.NewBaseTool(
			ports.ToolDefinition{
				Name:        "bg_plan",
				Description: `Build a background-task DAG plan for multiple coding tasks and optionally dispatch all nodes in dependency-safe order.`,
				Parameters: ports.ParameterSchema{
					Type: "object",
					Properties: map[string]ports.Property{
						"tasks": {
							Type:        "array",
							Description: "Array of task specs. Each item accepts: task_id, description, prompt, agent_type, execution_mode, autonomy_level, depends_on, config, workspace_mode, file_scope, inherit_context.",
							Items:       &ports.Property{Type: "object"},
						},
						"defaults": {
							Type:        "object",
							Description: "Optional defaults applied to each task: agent_type, execution_mode, autonomy_level, workspace_mode, config.",
						},
						"dispatch": {
							Type:        "boolean",
							Description: "When true, dispatch all planned tasks immediately.",
						},
					},
					Required: []string{"tasks"},
				},
			},
			ports.ToolMetadata{
				Name:     "bg_plan",
				Version:  "1.0.0",
				Category: "agent",
				Tags:     []string{"background", "orchestration", "dag", "planning"},
			},
		),
	}
}

func (t *bgPlan) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	for key := range call.Arguments {
		switch key {
		case "tasks", "defaults", "dispatch":
		default:
			return shared.ToolError(call.ID, "unsupported parameter: %s", key)
		}
	}

	tasksRaw, ok := call.Arguments["tasks"]
	if !ok {
		return shared.ToolError(call.ID, "tasks is required")
	}
	taskList, ok := tasksRaw.([]any)
	if !ok || len(taskList) == 0 {
		return shared.ToolError(call.ID, "tasks must be a non-empty array")
	}

	defaults, err := parseBGPlanDefaults(call.Arguments["defaults"])
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	dispatch, _, err := parseOptionalBool(call.Arguments, "dispatch")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	tasks := make([]bgPlanTask, 0, len(taskList))
	seen := make(map[string]struct{}, len(taskList))
	for i, item := range taskList {
		taskObj, ok := item.(map[string]any)
		if !ok {
			return shared.ToolError(call.ID, "tasks[%d] must be an object", i)
		}
		task, err := parseBGPlanTask(taskObj, defaults)
		if err != nil {
			return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("tasks[%d]: %v", i, err), Error: err}, nil
		}
		if _, exists := seen[task.ID]; exists {
			return shared.ToolError(call.ID, "duplicate task_id %q", task.ID)
		}
		seen[task.ID] = struct{}{}
		tasks = append(tasks, task)
	}

	taskIndex := make(map[string]int, len(tasks))
	for idx, task := range tasks {
		taskIndex[task.ID] = idx
	}
	for _, task := range tasks {
		for _, dep := range task.DependsOn {
			if _, exists := taskIndex[dep]; !exists {
				return shared.ToolError(call.ID, "task %q depends on unknown task_id %q", task.ID, dep)
			}
			if dep == task.ID {
				return shared.ToolError(call.ID, "task %q cannot depend on itself", task.ID)
			}
		}
	}

	order, err := topologicalOrder(tasks)
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	var dispatchedIDs []string
	if dispatch {
		dispatcher := agent.GetBackgroundDispatcher(ctx)
		if dispatcher == nil {
			return shared.ToolError(call.ID, "background task dispatch is not available in this context")
		}

		taskByID := make(map[string]bgPlanTask, len(tasks))
		for _, task := range tasks {
			taskByID[task.ID] = task
		}

		for _, taskID := range order {
			task := taskByID[taskID]
			req := agent.BackgroundDispatchRequest{
				TaskID:         task.ID,
				Description:    task.Description,
				Prompt:         task.Prompt,
				AgentType:      task.AgentType,
				ExecutionMode:  task.ExecutionMode,
				AutonomyLevel:  task.AutonomyLevel,
				CausationID:    call.ID,
				Config:         cloneTaskConfig(task.Config),
				DependsOn:      append([]string(nil), task.DependsOn...),
				WorkspaceMode:  task.WorkspaceMode,
				FileScope:      append([]string(nil), task.FileScope...),
				InheritContext: task.InheritContext,
			}
			if err := dispatcher.Dispatch(ctx, req); err != nil {
				return &ports.ToolResult{
					CallID:  call.ID,
					Content: fmt.Sprintf("dispatch failed for task %q: %v", task.ID, err),
					Error:   err,
				}, nil
			}
			dispatchedIDs = append(dispatchedIDs, task.ID)
		}
	}

	planID := "plan-" + id.NewKSUID()
	content := formatPlanSummary(planID, tasks, order, dispatch, dispatchedIDs)
	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"plan_id":        planID,
			"task_ids":       collectTaskIDs(tasks),
			"topo_order":     order,
			"dispatch":       dispatch,
			"dispatched_ids": dispatchedIDs,
		},
	}, nil
}

func parseBGPlanDefaults(raw any) (bgPlanDefaults, error) {
	defaults := bgPlanDefaults{
		AgentType:     "internal",
		ExecutionMode: "execute",
		AutonomyLevel: "controlled",
		WorkspaceMode: "",
	}
	if raw == nil {
		return defaults, nil
	}
	obj, ok := raw.(map[string]any)
	if !ok {
		return defaults, fmt.Errorf("defaults must be an object")
	}
	if v, ok := obj["agent_type"]; ok {
		str, ok := v.(string)
		if !ok {
			return defaults, fmt.Errorf("defaults.agent_type must be a string")
		}
		if strings.TrimSpace(str) != "" {
			defaults.AgentType = canonicalAgentType(str)
		}
	}
	if v, ok := obj["execution_mode"]; ok {
		str, ok := v.(string)
		if !ok {
			return defaults, fmt.Errorf("defaults.execution_mode must be a string")
		}
		defaults.ExecutionMode = normalizeModeString(str)
	}
	if v, ok := obj["autonomy_level"]; ok {
		str, ok := v.(string)
		if !ok {
			return defaults, fmt.Errorf("defaults.autonomy_level must be a string")
		}
		defaults.AutonomyLevel = normalizeAutonomyString(str)
	}
	if v, ok := obj["workspace_mode"]; ok {
		str, ok := v.(string)
		if !ok {
			return defaults, fmt.Errorf("defaults.workspace_mode must be a string")
		}
		defaults.WorkspaceMode = strings.TrimSpace(str)
	}
	config, err := parseStringMap(obj, "config")
	if err != nil {
		return defaults, err
	}
	defaults.Config = config
	return defaults, nil
}

func parseBGPlanTask(taskObj map[string]any, defaults bgPlanDefaults) (bgPlanTask, error) {
	var task bgPlanTask

	if raw, ok := taskObj["task_id"]; ok {
		str, ok := raw.(string)
		if !ok {
			return task, fmt.Errorf("task_id must be a string")
		}
		task.ID = strings.TrimSpace(str)
	}
	if task.ID == "" {
		task.ID = "bgp-" + id.NewKSUID()
	}

	description, ok := taskObj["description"].(string)
	if !ok || strings.TrimSpace(description) == "" {
		return task, fmt.Errorf("description is required and must be a string")
	}
	task.Description = strings.TrimSpace(description)

	prompt, ok := taskObj["prompt"].(string)
	if !ok || strings.TrimSpace(prompt) == "" {
		return task, fmt.Errorf("prompt is required and must be a string")
	}
	task.Prompt = strings.TrimSpace(prompt)

	task.AgentType = defaults.AgentType
	if raw, ok := taskObj["agent_type"]; ok {
		str, ok := raw.(string)
		if !ok {
			return task, fmt.Errorf("agent_type must be a string")
		}
		if strings.TrimSpace(str) != "" {
			task.AgentType = canonicalAgentType(str)
		}
	}

	task.ExecutionMode = defaults.ExecutionMode
	if raw, ok := taskObj["execution_mode"]; ok {
		str, ok := raw.(string)
		if !ok {
			return task, fmt.Errorf("execution_mode must be a string")
		}
		task.ExecutionMode = normalizeModeString(str)
	}

	task.AutonomyLevel = defaults.AutonomyLevel
	if raw, ok := taskObj["autonomy_level"]; ok {
		str, ok := raw.(string)
		if !ok {
			return task, fmt.Errorf("autonomy_level must be a string")
		}
		task.AutonomyLevel = normalizeAutonomyString(str)
	}

	config := cloneTaskConfig(defaults.Config)
	taskConfig, err := parseStringMap(taskObj, "config")
	if err != nil {
		return task, err
	}
	if config == nil {
		config = make(map[string]string)
	}
	for key, value := range taskConfig {
		config[key] = value
	}
	task.Config = config

	task.DependsOn, err = parseStringList(taskObj, "depends_on")
	if err != nil {
		return task, err
	}
	task.FileScope, err = parseStringList(taskObj, "file_scope")
	if err != nil {
		return task, err
	}
	task.InheritContext, _, err = parseOptionalBool(taskObj, "inherit_context")
	if err != nil {
		return task, err
	}

	workspaceMode := defaults.WorkspaceMode
	if raw, ok := taskObj["workspace_mode"]; ok {
		str, ok := raw.(string)
		if !ok {
			return task, fmt.Errorf("workspace_mode must be a string")
		}
		workspaceMode = strings.TrimSpace(str)
	}

	applyPlanCodingDefaults(&task, workspaceMode)
	return task, nil
}

func applyPlanCodingDefaults(task *bgPlanTask, workspaceMode string) {
	if task == nil {
		return
	}
	if task.Config == nil {
		task.Config = make(map[string]string)
	}
	task.Config["execution_mode"] = task.ExecutionMode
	task.Config["autonomy_level"] = task.AutonomyLevel

	if isCodingExternalAgent(task.AgentType) {
		if strings.TrimSpace(task.Config["task_kind"]) == "" {
			task.Config["task_kind"] = "coding"
		}
		if task.AutonomyLevel == "controlled" {
			task.AutonomyLevel = "full"
			task.Config["autonomy_level"] = "full"
		}
		if strings.TrimSpace(task.Config["verify"]) == "" {
			if task.ExecutionMode == "plan" {
				task.Config["verify"] = "false"
			} else {
				task.Config["verify"] = "true"
			}
		}
		if strings.TrimSpace(task.Config["merge_on_success"]) == "" {
			if task.ExecutionMode == "plan" {
				task.Config["merge_on_success"] = "false"
			} else {
				task.Config["merge_on_success"] = "true"
			}
		}
	}

	if workspaceMode == "" {
		if task.Config["task_kind"] == "coding" && task.ExecutionMode == "execute" {
			task.WorkspaceMode = agent.WorkspaceModeWorktree
		} else {
			task.WorkspaceMode = agent.WorkspaceModeShared
		}
		return
	}
	task.WorkspaceMode = agent.WorkspaceMode(strings.TrimSpace(workspaceMode))
}

func normalizeModeString(raw string) string {
	if strings.EqualFold(strings.TrimSpace(raw), "plan") {
		return "plan"
	}
	return "execute"
}

func normalizeAutonomyString(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case "full":
		return "full"
	case "semi":
		return "semi"
	default:
		return "controlled"
	}
}

func topologicalOrder(tasks []bgPlanTask) ([]string, error) {
	inDegree := make(map[string]int, len(tasks))
	adj := make(map[string][]string, len(tasks))
	for _, task := range tasks {
		inDegree[task.ID] = len(task.DependsOn)
		for _, dep := range task.DependsOn {
			adj[dep] = append(adj[dep], task.ID)
		}
	}

	queue := make([]string, 0, len(tasks))
	for _, task := range tasks {
		if inDegree[task.ID] == 0 {
			queue = append(queue, task.ID)
		}
	}

	order := make([]string, 0, len(tasks))
	for len(queue) > 0 {
		current := queue[0]
		queue = queue[1:]
		order = append(order, current)

		for _, next := range adj[current] {
			inDegree[next]--
			if inDegree[next] == 0 {
				queue = append(queue, next)
			}
		}
	}

	if len(order) != len(tasks) {
		return nil, fmt.Errorf("dependency cycle detected in plan")
	}
	return order, nil
}

func formatPlanSummary(planID string, tasks []bgPlanTask, order []string, dispatched bool, dispatchedIDs []string) string {
	taskByID := make(map[string]bgPlanTask, len(tasks))
	for _, task := range tasks {
		taskByID[task.ID] = task
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Plan %q created with %d tasks.\n", planID, len(tasks)))
	sb.WriteString("Topological order:\n")
	for i, taskID := range order {
		task := taskByID[taskID]
		sb.WriteString(fmt.Sprintf("%d. %s [%s] mode=%s autonomy=%s\n", i+1, task.ID, task.AgentType, task.ExecutionMode, task.AutonomyLevel))
		if len(task.DependsOn) > 0 {
			sb.WriteString(fmt.Sprintf("   depends_on: %s\n", strings.Join(task.DependsOn, ", ")))
		}
		sb.WriteString(fmt.Sprintf("   desc: %s\n", task.Description))
	}

	if dispatched {
		sb.WriteString(fmt.Sprintf("\nDispatched %d tasks: %s", len(dispatchedIDs), strings.Join(dispatchedIDs, ", ")))
	} else {
		sb.WriteString("\nPlan only. Set dispatch=true to run this DAG.")
	}
	return sb.String()
}

func collectTaskIDs(tasks []bgPlanTask) []string {
	ids := make([]string, 0, len(tasks))
	for _, task := range tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

func cloneTaskConfig(in map[string]string) map[string]string {
	if len(in) == 0 {
		return nil
	}
	out := make(map[string]string, len(in))
	for key, value := range in {
		out[key] = value
	}
	return out
}
