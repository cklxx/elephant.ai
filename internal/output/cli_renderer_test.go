package output

import (
	"strings"
	"testing"

	"alex/internal/agent/domain"
	"alex/internal/agent/types"
)

type stubMarkdownRenderer struct{}

func (stubMarkdownRenderer) Render(input string) (string, error) {
	normalized := strings.ReplaceAll(input, "\r", "")
	normalized = strings.ReplaceAll(normalized, "**", "")
	normalized = strings.ReplaceAll(normalized, "# ", "")
	normalized = strings.ReplaceAll(normalized, "- ", "• ")
	normalized = strings.TrimSpace(normalized)
	if normalized == "" {
		return "", nil
	}
	return normalized + "\n", nil
}

type paddedMarkdownRenderer struct{}

func (paddedMarkdownRenderer) Render(_ string) (string, error) {
	return "\x1b[0m   \nHello\n\x1b[0m \n", nil
}

func newTestRenderer(verbose bool) *CLIRenderer {
	return NewCLIRendererWithMarkdown(verbose, stubMarkdownRenderer{})
}

func TestRenderToolCallStartShowsArgsInCompactMode(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	args := map[string]any{"command": "ls -al"}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)

	if !strings.Contains(rendered, "command=") {
		t.Fatalf("expected command argument to be shown, got %q", rendered)
	}
}

func TestRenderToolCallStartTruncatesLongArgsInCompactMode(t *testing.T) {
	renderer := newTestRenderer(false)
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
	renderer := newTestRenderer(true)
	ctx := &types.OutputContext{Level: types.LevelCore}
	longCommand := strings.Repeat("abc", 20)
	args := map[string]any{"command": longCommand}

	rendered := renderer.RenderToolCallStart(ctx, "bash", args)

	if !strings.Contains(rendered, longCommand) {
		t.Fatalf("expected verbose mode to include full command, got %q", rendered)
	}
}

func TestRenderTaskCompleteRendersMarkdown(t *testing.T) {
	renderer := newTestRenderer(false)
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
	renderer := newTestRenderer(false)
	chunk := renderer.RenderMarkdownStreamChunk("**bold**", true)
	if !strings.Contains(chunk, "bold") {
		t.Fatalf("expected rendered chunk to include content, got %q", chunk)
	}
	if !strings.HasSuffix(chunk, "\n") {
		t.Fatalf("expected rendered chunk to end with newline, got %q", chunk)
	}
}

func TestRenderMarkdownStreamChunkTrimsWhitespaceEdges(t *testing.T) {
	renderer := NewCLIRendererWithMarkdown(false, paddedMarkdownRenderer{})
	chunk := renderer.RenderMarkdownStreamChunk("ignored", true)
	if chunk != "Hello\n" {
		t.Fatalf("expected trimmed output, got %q", chunk)
	}
}
