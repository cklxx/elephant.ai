package output

import (
	"strings"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/agent/types"
)

func TestRenderToolCallStartShowsArgsInCompactMode(t *testing.T) {
	renderer := NewCLIRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	args := map[string]any{"command": "ls -al"}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)

	if !strings.Contains(rendered, "command=") {
		t.Fatalf("expected command argument to be shown, got %q", rendered)
	}
}

func TestRenderToolCallStartTruncatesLongArgsInCompactMode(t *testing.T) {
	renderer := NewCLIRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	longCommand := strings.Repeat("abc", 100)
	args := map[string]any{"command": longCommand}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)

	if !strings.Contains(rendered, "…") {
		t.Fatalf("expected long args to be truncated with ellipsis, got %q", rendered)
	}
	if strings.Contains(rendered, longCommand) {
		t.Fatalf("expected long command to be truncated, but full command present")
	}
}

func TestRenderToolCallStartKeepsFullArgsInVerboseMode(t *testing.T) {
	renderer := NewCLIRenderer(true)
	ctx := &types.OutputContext{Level: types.LevelCore}
	longCommand := strings.Repeat("abc", 20)
	args := map[string]any{"command": longCommand}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)

	if !strings.Contains(rendered, longCommand) {
		t.Fatalf("expected verbose mode to include full command, got %q", rendered)
	}
}

func TestRenderTaskCompleteRendersMarkdown(t *testing.T) {
	renderer := NewCLIRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := &domain.TaskResult{
		Answer:     "# Title\n\n- item",
		Iterations: 1,
		TokensUsed: 10,
	}

	rendered := renderer.RenderTaskComplete(ctx, result)

	if strings.Contains(rendered, "# Title") {
		t.Fatalf("expected markdown heading to be rendered, got %q", rendered)
	}

	if !strings.Contains(rendered, "Title") {
		t.Fatalf("expected rendered output to include heading text, got %q", rendered)
	}

	if !strings.Contains(rendered, "•") {
		t.Fatalf("expected rendered output to include bullet glyph, got %q", rendered)
	}
}

func TestRenderMarkdownStreamChunkMaintainsTrailingNewline(t *testing.T) {
	renderer := NewCLIRenderer(false)
	chunk := renderer.RenderMarkdownStreamChunk("**bold**", true)
	if !strings.Contains(chunk, "bold") {
		t.Fatalf("expected rendered chunk to include content, got %q", chunk)
	}
	if !strings.HasSuffix(chunk, "\n") {
		t.Fatalf("expected rendered chunk to end with newline, got %q", chunk)
	}
}
