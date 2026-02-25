package orchestration

import (
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/domain/agent/ports"
	"alex/internal/domain/workflow"
)

// formatResults formats subtask results for the LLM
func (t *subagent) formatResults(call ports.ToolCall, subtasks []string, results []SubtaskResult, mode string) (*ports.ToolResult, error) {
	var output strings.Builder

	// Calculate summary statistics
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

	// Concise output for LLM
	output.WriteString(fmt.Sprintf("Subagent completed %d/%d tasks (%s mode)\n\n", successCount, len(subtasks), mode))

	for _, r := range results {
		if r.Error != nil {
			output.WriteString(fmt.Sprintf("Task %d failed: %v\n\n", r.Index+1, r.Error))
		} else {
			output.WriteString(fmt.Sprintf("Task %d result:\n%s\n\n", r.Index+1, strings.TrimSpace(r.Answer)))
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

	structured := buildSubtaskMetadata(results)

	// Add individual results to metadata
	resultsJSON, _ := json.Marshal(results)
	metadata["results"] = string(resultsJSON)
	metadata["results_struct"] = structured
	metadata["workflows"] = extractWorkflows(structured)

	return &ports.ToolResult{
		CallID:       call.ID,
		Content:      output.String(),
		Metadata:     metadata,
		SessionID:    call.SessionID,
		TaskID:       call.TaskID,
		ParentTaskID: call.ParentTaskID,
	}, nil
}

type subtaskMetadata struct {
	Index      int                        `json:"index"`
	Task       string                     `json:"task"`
	Answer     string                     `json:"answer,omitempty"`
	Iterations int                        `json:"iterations,omitempty"`
	TokensUsed int                        `json:"tokens_used,omitempty"`
	Workflow   *workflow.WorkflowSnapshot `json:"workflow,omitempty"`
	LogID      string                     `json:"log_id,omitempty"`
	Error      string                     `json:"error,omitempty"`
}

func buildSubtaskMetadata(results []SubtaskResult) []subtaskMetadata {
	structured := make([]subtaskMetadata, 0, len(results))
	for _, r := range results {
		item := subtaskMetadata{
			Index:      r.Index,
			Task:       r.Task,
			Answer:     r.Answer,
			Iterations: r.Iterations,
			TokensUsed: r.TokensUsed,
			Workflow:   r.Workflow,
			LogID:      r.LogID,
		}
		if r.Error != nil {
			item.Error = r.Error.Error()
		}
		structured = append(structured, item)
	}
	return structured
}

func extractWorkflows(results []subtaskMetadata) []*workflow.WorkflowSnapshot {
	workflows := make([]*workflow.WorkflowSnapshot, 0, len(results))
	for _, r := range results {
		if r.Workflow != nil {
			workflows = append(workflows, r.Workflow)
		}
	}
	return workflows
}
