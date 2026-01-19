package builtin

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"alex/internal/agent/ports"
	"alex/internal/memory"
	id "alex/internal/utils/id"
)

type uiPlan struct {
	memory memory.Service
}

func NewPlan(memoryService memory.Service) ports.ToolExecutor {
	return &uiPlan{memory: memoryService}
}

func (t *uiPlan) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "plan",
		Version:  "1.0.0",
		Category: "ui",
		Tags:     []string{"ui", "orchestration"},
	}
}

func (t *uiPlan) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name: "plan",
		Description: `UI tool: emit Level 1 goal and (optionally) attach a hidden internal plan for the orchestrator.

Rules:
- Must be called before any non-plan/clearify tool call.
- When complexity="simple", overall_goal_ui must be a single line.
- overall_goal_ui must state the deliverable and a measurable acceptance signal (paths/tests/metrics).
- When complexity="simple", you may proceed directly to the required action tool calls after plan(); do NOT call clearify() unless you need to pause for user input.
- When complexity="complex", call clearify() before the first action tool call for each task.`,
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"run_id": {
					Type:        "string",
					Description: "Run identifier (must match the current run_id provided by the system).",
				},
				"session_title": {
					Type:        "string",
					Description: "Short session title used for session headers/lists (single-line).",
				},
				"overall_goal_ui": {
					Type:        "string",
					Description: "User-facing goal UI text. For complexity=simple it must be single-line.",
				},
				"complexity": {
					Type:        "string",
					Description: "simple or complex",
					Enum:        []any{"simple", "complex"},
				},
				"memory_keywords": {
					Type:        "array",
					Description: "Keywords to recall user memories before planning.",
					Items:       &ports.Property{Type: "string"},
				},
				"memory_slots": {
					Type:        "object",
					Description: "Intent slots (key/value) used to recall user memories.",
				},
				"internal_plan": {
					Type:        "object",
					Description: "Hidden internal plan object for orchestrator storage (must not be rendered).",
				},
			},
			Required: []string{"run_id", "overall_goal_ui", "complexity"},
		},
	}
}

func (t *uiPlan) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	runID, ok := call.Arguments["run_id"].(string)
	if !ok {
		err := fmt.Errorf("missing 'run_id'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	runID = strings.TrimSpace(runID)
	if runID == "" {
		err := fmt.Errorf("run_id cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	complexityRaw, ok := call.Arguments["complexity"].(string)
	if !ok {
		err := fmt.Errorf("missing 'complexity'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	complexity := strings.ToLower(strings.TrimSpace(complexityRaw))
	if complexity != "simple" && complexity != "complex" {
		err := fmt.Errorf("complexity must be \"simple\" or \"complex\"")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if complexity != "simple" {
		expected := strings.TrimSpace(id.TaskIDFromContext(ctx))
		if expected != "" && runID != expected {
			err := errors.New("run_id does not match the active task")
			return &ports.ToolResult{
				CallID: call.ID,
				Content: "Request does not match the active task. Please retry " +
					"from the latest conversation turn.",
				Error: err,
			}, nil
		}
	}

	goal, ok := call.Arguments["overall_goal_ui"].(string)
	if !ok {
		err := fmt.Errorf("missing 'overall_goal_ui'")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}
	goal = strings.TrimSpace(goal)
	if goal == "" {
		err := fmt.Errorf("overall_goal_ui cannot be empty")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	if complexity == "simple" && (strings.Contains(goal, "\n") || strings.Contains(goal, "\r")) {
		err := fmt.Errorf("overall_goal_ui must be single-line when complexity=\"simple\"")
		return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
	}

	var sessionTitle string
	if raw, exists := call.Arguments["session_title"]; exists {
		value, ok := raw.(string)
		if !ok {
			err := fmt.Errorf("session_title must be a string")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
		sessionTitle = strings.TrimSpace(value)
		if sessionTitle != "" && (strings.Contains(sessionTitle, "\n") || strings.Contains(sessionTitle, "\r")) {
			err := fmt.Errorf("session_title must be single-line")
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	// Reject unexpected parameters to keep the protocol strict.
	for key := range call.Arguments {
		switch key {
		case "run_id", "session_title", "overall_goal_ui", "complexity", "internal_plan", "memory_keywords", "memory_slots":
		default:
			err := fmt.Errorf("unsupported parameter: %s", key)
			return &ports.ToolResult{CallID: call.ID, Content: err.Error(), Error: err}, nil
		}
	}

	metadata := map[string]any{
		"run_id":          runID,
		"overall_goal_ui": goal,
		"complexity":      complexity,
	}
	if sessionTitle != "" {
		metadata["session_title"] = sessionTitle
	}

	memoryKeywords := parseKeywordArray(call.Arguments["memory_keywords"])
	if len(memoryKeywords) > 0 {
		metadata["memory_keywords"] = memoryKeywords
	}
	memorySlots := parseSlotObject(call.Arguments["memory_slots"])
	if len(memorySlots) > 0 {
		metadata["memory_slots"] = memorySlots
	}
	if internalPlan, exists := call.Arguments["internal_plan"]; exists {
		metadata["internal_plan"] = internalPlan
	}

	if t.memory != nil && (len(memoryKeywords) > 0 || len(memorySlots) > 0) {
		userID := id.UserIDFromContext(ctx)
		if strings.TrimSpace(userID) != "" {
			memories, err := t.memory.Recall(ctx, memory.Query{
				UserID:   userID,
				Keywords: memoryKeywords,
				Slots:    memorySlots,
			})
			if err == nil && len(memories) > 0 {
				metadata["memory_recall"] = serializeMemories(memories)
			}
		}
	}

	return &ports.ToolResult{
		CallID:   call.ID,
		Content:  goal,
		Metadata: metadata,
	}, nil
}
