package builtin

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
)

type explore struct {
	subagent ports.ToolExecutor
}

// NewExplore creates an explore tool that wraps the subagent executor.
func NewExplore(subagent ports.ToolExecutor) ports.ToolExecutor {
	return &explore{subagent: subagent}
}

func (e *explore) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "explore",
		Description: "Plan and delegate multi-scope investigations via the subagent tool. Automatically prepares local, web, and custom subtasks and synthesizes a concise summary of findings.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"objective": {
					Type:        "string",
					Description: "High-level goal to investigate.",
				},
				"local_scope": {
					Type:        "array",
					Description: "Specific local/codebase areas to inspect.",
				},
				"web_scope": {
					Type:        "array",
					Description: "Web research focus areas.",
				},
				"custom_tasks": {
					Type:        "array",
					Description: "Additional custom subtasks to run.",
				},
				"notes": {
					Type:        "string",
					Description: "Context or constraints shared with every subtask.",
				},
				"mode": {
					Type:        "string",
					Description: "Delegation mode forwarded to subagent (parallel or serial).",
					Enum:        []any{"parallel", "serial"},
				},
			},
			Required: []string{"objective"},
		},
	}
}

func (e *explore) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "explore",
		Version:  "1.0.0",
		Category: "orchestration",
		Tags:     []string{"planning", "delegation", "subagent"},
	}
}

func (e *explore) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	if e.subagent == nil {
		err := fmt.Errorf("explore tool is unavailable: subagent not registered")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	objectiveVal, ok := call.Arguments["objective"].(string)
	if !ok || strings.TrimSpace(objectiveVal) == "" {
		err := fmt.Errorf("objective must be a non-empty string")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	objective := strings.TrimSpace(objectiveVal)

	localScope, err := parseStringList(call.Arguments, "local_scope")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	webScope, err := parseStringList(call.Arguments, "web_scope")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	customTasks, err := parseStringList(call.Arguments, "custom_tasks")
	if err != nil {
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	notes := ""
	if raw, exists := call.Arguments["notes"]; exists {
		noteStr, ok := raw.(string)
		if !ok {
			err := fmt.Errorf("notes must be a string when provided")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		notes = strings.TrimSpace(noteStr)
	}

	mode := "parallel"
	if raw, exists := call.Arguments["mode"]; exists {
		modeStr, ok := raw.(string)
		if !ok {
			err := fmt.Errorf("mode must be a string when provided")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		normalized := strings.ToLower(strings.TrimSpace(modeStr))
		if normalized != "parallel" && normalized != "serial" {
			err := fmt.Errorf("mode must be either 'parallel' or 'serial'")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		mode = normalized
	}

	subtasks := buildExploreSubtasks(objective, localScope, webScope, customTasks, notes)
	if len(subtasks) == 0 {
		base := fmt.Sprintf("[CUSTOM] %s", objective)
		subtasks = []string{appendNotes(base, notes)}
	}

	subagentSubtasks := make([]any, len(subtasks))
	for i, task := range subtasks {
		subagentSubtasks[i] = task
	}

	delegationArgs := map[string]any{
		"subtasks": subagentSubtasks,
		"mode":     mode,
	}

	delegationCall := ports.ToolCall{
		ID:        call.ID + ":explore",
		Name:      "subagent",
		Arguments: delegationArgs,
	}

	delegateResult, execErr := e.subagent.Execute(ctx, delegationCall)
	if execErr != nil {
		return &ports.ToolResult{CallID: call.ID, Content: fmt.Sprintf("subagent execution failed: %v", execErr), Error: execErr}, nil
	}

	delegationMetadata := cloneMetadata(delegateResult.Metadata)
	delegationDetails := map[string]any{
		"call": map[string]any{
			"tool": delegationCall.Name,
			"arguments": map[string]any{
				"mode":     mode,
				"subtasks": subtasks,
			},
		},
		"result_metadata": delegationMetadata,
		"result_content":  delegateResult.Content,
	}
	if delegateResult.Error != nil {
		delegationDetails["error"] = delegateResult.Error.Error()
		return &ports.ToolResult{
			CallID:  call.ID,
			Content: fmt.Sprintf("Delegation failed: %s", delegateResult.Error.Error()),
			Error:   delegateResult.Error,
			Metadata: map[string]any{
				"objective":     objective,
				"local_scope":   localScope,
				"web_scope":     webScope,
				"custom_tasks":  customTasks,
				"notes":         notes,
				"mode":          mode,
				"subtasks":      subtasks,
				"delegation":    delegationDetails,
				"total_tasks":   len(subtasks),
				"success_count": 0,
				"failure_count": len(subtasks),
			},
		}, nil
	}

	parsedResults := parseDelegationResults(delegationMetadata)
	successCount, failureCount := countDelegationOutcomes(parsedResults, delegationMetadata)
	totalTasks := len(parsedResults)
	if totalTasks == 0 {
		totalTasks = len(subtasks)
	}

	highlights := buildDelegationHighlights(parsedResults)

	summary := buildExploreSummary(objective, len(localScope), len(webScope), len(customTasks), totalTasks, successCount, failureCount, highlights, notes)

	resultMetadata := map[string]any{
		"objective":          objective,
		"local_scope":        localScope,
		"web_scope":          webScope,
		"custom_tasks":       customTasks,
		"notes":              notes,
		"mode":               mode,
		"subtasks":           subtasks,
		"total_tasks":        totalTasks,
		"success_count":      successCount,
		"failure_count":      failureCount,
		"summary_highlights": highlights,
		"delegation":         delegationDetails,
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  summary,
		Metadata: resultMetadata,
	}, nil
}

func parseStringList(args map[string]any, key string) ([]string, error) {
	raw, exists := args[key]
	if !exists || raw == nil {
		return nil, nil
	}

	switch v := raw.(type) {
	case []string:
		result := make([]string, 0, len(v))
		for _, item := range v {
			if strings.TrimSpace(item) != "" {
				result = append(result, strings.TrimSpace(item))
			}
		}
		return result, nil
	case []any:
		result := make([]string, 0, len(v))
		for i, item := range v {
			str, ok := item.(string)
			if !ok {
				return nil, fmt.Errorf("%s[%d] must be a string", key, i)
			}
			if trimmed := strings.TrimSpace(str); trimmed != "" {
				result = append(result, trimmed)
			}
		}
		return result, nil
	default:
		return nil, fmt.Errorf("%s must be an array of strings when provided", key)
	}
}

func buildExploreSubtasks(objective string, localScope, webScope, customTasks []string, notes string) []string {
	var subtasks []string

	for _, focus := range localScope {
		base := fmt.Sprintf("[LOCAL] %s — Focus on %s.", objective, focus)
		subtasks = append(subtasks, appendNotes(base, notes))
	}
	for _, focus := range webScope {
		base := fmt.Sprintf("[WEB] %s — Research %s.", objective, focus)
		subtasks = append(subtasks, appendNotes(base, notes))
	}
	for _, task := range customTasks {
		base := fmt.Sprintf("[CUSTOM] %s", task)
		subtasks = append(subtasks, appendNotes(base, notes))
	}

	if len(subtasks) == 0 {
		return subtasks
	}

	return subtasks
}

func appendNotes(base, notes string) string {
	if strings.TrimSpace(notes) == "" {
		return strings.TrimSpace(base)
	}

	trimmed := strings.TrimSpace(base)
	if strings.HasSuffix(trimmed, ".") {
		return fmt.Sprintf("%s Notes: %s", trimmed, notes)
	}
	return fmt.Sprintf("%s. Notes: %s", trimmed, notes)
}

type delegationSubtask struct {
	Index      int         `json:"index"`
	Task       string      `json:"task"`
	Answer     string      `json:"answer"`
	Iterations int         `json:"iterations"`
	TokensUsed int         `json:"tokens_used"`
	Error      interface{} `json:"error"`
}

func parseDelegationResults(metadata map[string]any) []delegationSubtask {
	if metadata == nil {
		return nil
	}
	raw, ok := metadata["results"]
	if !ok || raw == nil {
		return nil
	}

	var data []byte
	switch v := raw.(type) {
	case string:
		data = []byte(v)
	case []byte:
		data = v
	default:
		marshaled, err := json.Marshal(v)
		if err != nil {
			return nil
		}
		data = marshaled
	}

	var results []delegationSubtask
	if err := json.Unmarshal(data, &results); err != nil {
		return nil
	}
	return results
}

func countDelegationOutcomes(results []delegationSubtask, metadata map[string]any) (int, int) {
	success := 0
	failure := 0
	for _, r := range results {
		if hasDelegationError(r.Error) {
			failure++
		} else {
			success++
		}
	}

	if success == 0 && failure == 0 {
		if s, ok := toInt(metadata["success_count"]); ok {
			success = s
		}
		if f, ok := toInt(metadata["failure_count"]); ok {
			failure = f
		}
	}
	return success, failure
}

func buildDelegationHighlights(results []delegationSubtask) []string {
	highlights := make([]string, 0, len(results))
	for _, r := range results {
		if len(highlights) >= 3 {
			break
		}
		if hasDelegationError(r.Error) {
			highlights = append(highlights, fmt.Sprintf("Task %d failed: %s", r.Index+1, delegationErrorString(r.Error)))
			continue
		}
		summary := strings.TrimSpace(r.Answer)
		if summary == "" {
			continue
		}
		summary = firstLine(summary)
		if len(summary) > 120 {
			summary = summary[:117] + "..."
		}
		highlights = append(highlights, fmt.Sprintf("Task %d: %s", r.Index+1, summary))
	}
	return highlights
}

func buildExploreSummary(objective string, localCount, webCount, customCount, total, success, failure int, highlights []string, notes string) string {
	summary := fmt.Sprintf("Delegated objective \"%s\" across %d subtask(s) (local:%d, web:%d, custom:%d) with %d success/%d failure.", objective, total, localCount, webCount, customCount, success, failure)

	if len(highlights) > 0 {
		summary += "\nHighlights:"
		for _, h := range highlights {
			summary += "\n- " + h
		}
	}
	if strings.TrimSpace(notes) != "" {
		summary += "\nShared notes: " + notes
	}
	return summary
}

func hasDelegationError(err interface{}) bool {
	switch v := err.(type) {
	case nil:
		return false
	case string:
		return strings.TrimSpace(v) != ""
	case bool:
		return v
	case map[string]any:
		return len(v) > 0
	case []any:
		return len(v) > 0
	default:
		return true
	}
}

func delegationErrorString(err interface{}) string {
	switch v := err.(type) {
	case nil:
		return ""
	case string:
		return strings.TrimSpace(v)
	case fmt.Stringer:
		return v.String()
	case map[string]any:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	case []any:
		b, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(b)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func firstLine(text string) string {
	lines := strings.Split(text, "\n")
	if len(lines) == 0 {
		return ""
	}
	return strings.TrimSpace(lines[0])
}

func toInt(value interface{}) (int, bool) {
	switch v := value.(type) {
	case nil:
		return 0, false
	case int:
		return v, true
	case int32:
		return int(v), true
	case int64:
		return int(v), true
	case float32:
		return int(v), true
	case float64:
		return int(v), true
	case json.Number:
		i, err := v.Int64()
		if err != nil {
			return 0, false
		}
		return int(i), true
	default:
		return 0, false
	}
}

func cloneMetadata(metadata map[string]any) map[string]any {
	if metadata == nil {
		return nil
	}
	cloned := make(map[string]any, len(metadata))
	for k, v := range metadata {
		cloned[k] = v
	}
	return cloned
}
