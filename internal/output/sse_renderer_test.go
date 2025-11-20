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
		BaseEvent:  domain.BaseEvent{},
		ActionName: "Investigate issue",
		Goal:       "Fix 500 error",
		Approach:   "Inspect logs then reproduce",
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
	if _, ok := payload.Data["success_criteria"]; ok {
		t.Fatalf("expected success criteria omitted, got %v", payload.Data["success_criteria"])
	}
	if _, ok := payload.Data["steps"]; ok {
		t.Fatalf("expected steps omitted, got %v", payload.Data["steps"])
	}
	if _, ok := payload.Data["retrieval_plan"]; ok {
		t.Fatalf("expected retrieval plan omitted, got %v", payload.Data["retrieval_plan"])
	}
}
