package output

import (
	"encoding/json"
	"strings"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/agent/ports"
	"alex/internal/agent/types"
)

func decodeSSEPayload(t *testing.T, raw string) SSEEvent {
	t.Helper()

	const prefix = "data: "
	if !strings.HasPrefix(raw, prefix) {
		t.Fatalf("expected SSE payload to start with %q, got %q", prefix, raw)
	}

	payload := strings.TrimPrefix(raw, prefix)
	payload = strings.TrimSpace(payload)

	var event SSEEvent
	if err := json.Unmarshal([]byte(payload), &event); err != nil {
		t.Fatalf("failed to unmarshal SSE payload: %v", err)
	}

	return event
}

func TestSSERendererIncludesFormattedArguments(t *testing.T) {
	renderer := NewSSERenderer()
	ctx := &types.OutputContext{Level: types.LevelCore}
	args := map[string]any{"command": "ls -al"}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)
	event := decodeSSEPayload(t, rendered)

	arguments, ok := event.Data["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments map in SSE event, got %T", event.Data["arguments"])
	}
	if arguments["command"] != "ls -al" {
		t.Fatalf("expected summarized command, got %v", arguments["command"])
	}

	preview, ok := event.Data["arguments_preview"].(string)
	if !ok {
		t.Fatalf("expected arguments_preview string in SSE event, got %T", event.Data["arguments_preview"])
	}
	if !strings.Contains(preview, "command=") {
		t.Fatalf("expected preview to include command value, got %q", preview)
	}
}

func TestSSERendererTruncatesLongArgumentsPreview(t *testing.T) {
	renderer := NewSSERenderer()
	ctx := &types.OutputContext{Level: types.LevelCore}
	longCommand := strings.Repeat("abc", 200)
	args := map[string]any{"command": longCommand}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)
	event := decodeSSEPayload(t, rendered)

	preview, ok := event.Data["arguments_preview"].(string)
	if !ok {
		t.Fatalf("expected arguments_preview string in SSE event, got %T", event.Data["arguments_preview"])
	}
	if !strings.Contains(preview, "…") {
		t.Fatalf("expected long preview to include ellipsis, got %q", preview)
	}
	if strings.Contains(preview, longCommand) {
		t.Fatalf("expected preview to be truncated, but full command present")
	}

	arguments, ok := event.Data["arguments"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected arguments map in SSE event, got %T", event.Data["arguments"])
	}
	if cmd, _ := arguments["command"].(string); strings.Contains(cmd, longCommand) {
		t.Fatalf("expected arguments map to contain summarized command, got %q", cmd)
	}
}

func TestSSERendererTaskAnalysisPayloadIncludesPlan(t *testing.T) {
	renderer := NewSSERenderer()
	ctx := &types.OutputContext{Level: types.LevelCore, AgentID: "core", SessionID: "session-1", TaskID: "task-1"}
	event := &domain.TaskAnalysisEvent{
		BaseEvent:       domain.BaseEvent{},
		ActionName:      "Investigate issue",
		Goal:            "Fix 500 error",
		Approach:        "Inspect logs then reproduce",
		SuccessCriteria: []string{"Reproduce error", "Ship patch"},
		Steps: []ports.TaskAnalysisStep{{
			Description:          "Collect recent logs",
			NeedsExternalContext: true,
			Rationale:            "Verify stack traces",
		}},
		Retrieval: ports.TaskRetrievalPlan{
			ShouldRetrieve: false,
			LocalQueries:   []string{"error.log", "stack trace"},
			KnowledgeGaps:  []string{"Root cause hypotheses"},
			Notes:          "Check staging if prod data insufficient",
		},
	}

	rendered := renderer.RenderTaskAnalysis(ctx, event)
	payload := decodeSSEPayload(t, rendered)

	if payload.Type != "task_analysis" {
		t.Fatalf("expected task_analysis type, got %s", payload.Type)
	}
	if payload.Data["action_name"] != "Investigate issue" {
		t.Fatalf("expected action name, got %v", payload.Data["action_name"])
	}
	if payload.Data["approach"] != "Inspect logs then reproduce" {
		t.Fatalf("expected approach to be present, got %v", payload.Data["approach"])
	}
	criteria, ok := payload.Data["success_criteria"].([]interface{})
	if !ok || len(criteria) != 2 {
		t.Fatalf("expected success criteria serialized, got %v", payload.Data["success_criteria"])
	}
	steps, ok := payload.Data["steps"].([]interface{})
	if !ok || len(steps) != 1 {
		t.Fatalf("expected steps array, got %v", payload.Data["steps"])
	}
	retrieval, ok := payload.Data["retrieval_plan"].(map[string]interface{})
	if !ok {
		t.Fatalf("expected retrieval plan map, got %T", payload.Data["retrieval_plan"])
	}
	if retrieval["should_retrieve"] != true {
		t.Fatalf("expected coerced retrieval flag true, got %v", retrieval["should_retrieve"])
	}
	if _, ok := retrieval["knowledge_gaps"].([]interface{}); !ok {
		t.Fatalf("expected knowledge gaps slice, got %v", retrieval["knowledge_gaps"])
	}
}
