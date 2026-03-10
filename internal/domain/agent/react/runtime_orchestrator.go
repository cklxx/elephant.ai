package react

import (
	"strings"

	agent "alex/internal/domain/agent/ports/agent"
	"alex/internal/domain/agent/ports"
	"alex/internal/shared/utils"
)

func (r *reactRuntime) updateOrchestratorState(calls []ToolCall, results []ToolResult) {
	if len(calls) == 0 || len(results) == 0 {
		return
	}
	limit := len(calls)
	if len(results) < limit {
		limit = len(results)
	}

	for i := 0; i < limit; i++ {
		call := calls[i]
		result := results[i]
		if result.Error != nil {
			r.handleToolError(call, result)
			continue
		}

		name := utils.TrimLower(call.Name)
		switch name {
		case "plan":
			r.planEmitted = true
			r.planVersion++
			if raw, ok := call.Arguments["complexity"].(string); ok {
				complexity := utils.TrimLower(raw)
				if complexity == "simple" || complexity == "complex" {
					r.planComplexity = complexity
				}
			} else if result.Metadata != nil {
				if raw, ok := result.Metadata["complexity"].(string); ok {
					complexity := utils.TrimLower(raw)
					if complexity == "simple" || complexity == "complex" {
						r.planComplexity = complexity
					}
				}
			}
			r.maybeTriggerPlanReview(call, result)
		case "ask_user":
			r.handleClarifyResult(result)
			if result.Metadata != nil {
				if needs, ok := result.Metadata["needs_user_input"].(bool); ok && needs {
					r.pauseRequested = true
				}
			}
		}
	}
}

func (r *reactRuntime) handleToolError(_ ToolCall, _ ToolResult) {
	targetID := strings.TrimSpace(r.currentTaskID)
	if targetID == "" && len(r.state.Plans) > 0 {
		targetID = strings.TrimSpace(r.state.Plans[len(r.state.Plans)-1].ID)
	}
	if targetID != "" {
		r.updatePlanStatus(targetID, planStatusBlocked, false)
	}
}

func (r *reactRuntime) handleClarifyResult(result ToolResult) {
	if result.Metadata == nil {
		return
	}
	taskID, _ := result.Metadata["task_id"].(string)
	taskID = strings.TrimSpace(taskID)
	if taskID == "" {
		return
	}
	taskGoal, _ := result.Metadata["task_goal_ui"].(string)
	taskGoal = strings.TrimSpace(taskGoal)

	if r.currentTaskID != "" && r.currentTaskID != taskID {
		r.updatePlanStatus(r.currentTaskID, planStatusCompleted, true)
	}

	node := agent.PlanNode{
		ID:          taskID,
		Title:       taskGoal,
		Status:      planStatusInProgress,
		Description: strings.Join(extractSuccessCriteria(result.Metadata), "\n"),
	}
	r.upsertPlanNode(node)

	r.currentTaskID = taskID
	r.clarifyEmitted[taskID] = true
	r.pendingTaskID = ""
}

func extractSuccessCriteria(metadata map[string]any) []string {
	if metadata == nil {
		return nil
	}
	if raw, ok := metadata["success_criteria"].([]string); ok {
		return append([]string(nil), raw...)
	}
	raw, ok := metadata["success_criteria"].([]any)
	if !ok {
		return nil
	}
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		text, ok := item.(string)
		if !ok {
			continue
		}
		trimmed := strings.TrimSpace(text)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func (r *reactRuntime) upsertPlanNode(node agent.PlanNode) {
	if utils.IsBlank(node.ID) {
		return
	}
	if updatePlanNode(r.state.Plans, node) {
		return
	}
	r.state.Plans = append(r.state.Plans, node)
}

func updatePlanNode(nodes []agent.PlanNode, node agent.PlanNode) bool {
	for i := range nodes {
		if strings.TrimSpace(nodes[i].ID) == strings.TrimSpace(node.ID) {
			nodes[i].Title = node.Title
			nodes[i].Description = node.Description
			nodes[i].Status = node.Status
			return true
		}
		if updatePlanNode(nodes[i].Children, node) {
			return true
		}
	}
	return false
}

func (r *reactRuntime) updatePlanStatus(id string, status string, skipIfBlocked bool) bool {
	return updatePlanStatus(r.state.Plans, strings.TrimSpace(id), status, skipIfBlocked)
}

func updatePlanStatus(nodes []agent.PlanNode, id string, status string, skipIfBlocked bool) bool {
	if utils.IsBlank(id) {
		return false
	}
	for i := range nodes {
		if strings.TrimSpace(nodes[i].ID) == id {
			if skipIfBlocked && nodes[i].Status == planStatusBlocked {
				return true
			}
			nodes[i].Status = status
			return true
		}
		if updatePlanStatus(nodes[i].Children, id, status, skipIfBlocked) {
			return true
		}
	}
	return false
}

func (r *reactRuntime) maybeTriggerPlanReview(call ToolCall, result ToolResult) {
	if !r.planReviewEnabled {
		return
	}
	if !strings.EqualFold(strings.TrimSpace(r.planComplexity), "complex") {
		return
	}
	if r.planVersion <= r.lastPlanReviewVersion {
		return
	}

	goal := ""
	internalPlan := any(nil)
	if result.Metadata != nil {
		if raw, ok := result.Metadata["overall_goal_ui"].(string); ok {
			goal = strings.TrimSpace(raw)
		}
		if raw, ok := result.Metadata["internal_plan"]; ok {
			internalPlan = raw
		}
	}
	if goal == "" {
		if raw, ok := call.Arguments["overall_goal_ui"].(string); ok {
			goal = strings.TrimSpace(raw)
		}
	}
	if internalPlan == nil {
		if raw, ok := call.Arguments["internal_plan"]; ok {
			internalPlan = raw
		}
	}
	if goal == "" {
		return
	}

	r.injectPlanReviewMarker(goal, internalPlan, r.runID)
	r.pauseRequested = true
	r.lastPlanReviewVersion = r.planVersion
}

func (r *reactRuntime) injectPlanReviewMarker(goal string, internalPlan any, runID string) {
	if runID == "" {
		runID = "<run_id>"
	}

	planText := ""
	if internalPlan != nil {
		if data, err := r.engine.jsonCodec(internalPlan); err == nil {
			planText = string(data)
		}
	}

	var sb strings.Builder
	sb.WriteString("<plan_review_pending>\n")
	sb.WriteString("run_id: ")
	sb.WriteString(runID)
	sb.WriteString("\n")
	sb.WriteString("overall_goal_ui: ")
	sb.WriteString(goal)
	sb.WriteString("\n")
	if planText != "" {
		sb.WriteString("internal_plan: ")
		sb.WriteString(planText)
		sb.WriteString("\n")
	}
	sb.WriteString("</plan_review_pending>")

	r.state.Messages = append(r.state.Messages, Message{
		Role:    "system",
		Content: sb.String(),
		Source:  ports.MessageSourceSystemPrompt,
	})
}
