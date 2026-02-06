package output

import (
	"encoding/json"
	"strings"
	"testing"

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
