package output

import (
	"strings"
	"testing"
	"time"

	"alex/internal/domain/agent"
	"alex/internal/domain/agent/types"

	"github.com/charmbracelet/x/ansi"
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

func TestRenderToolCallStartSkipsConversationalTools(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}

	rendered := renderer.RenderToolCallStart(ctx, "plan", map[string]any{"run_id": "task-1"})

	if rendered != "" {
		t.Fatalf("expected conversational tool start to be skipped, got %q", rendered)
	}
}

func TestRenderToolCallCompleteRendersConversationalContent(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}

	rendered := renderer.RenderToolCallComplete(ctx, "clarify", "Need input", nil, 0)

	if strings.Contains(rendered, "clarify") {
		t.Fatalf("expected conversational tool name to be hidden, got %q", rendered)
	}
	if !strings.Contains(rendered, "Need input") {
		t.Fatalf("expected conversational content to be shown, got %q", rendered)
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

func TestRenderToolCallCompleteConstrainsWidth(t *testing.T) {
	renderer := newTestRenderer(false)
	renderer.maxWidth = 24
	ctx := &types.OutputContext{Level: types.LevelCore}
	longResult := strings.Repeat("a", 200)

	rendered := renderer.RenderToolCallComplete(ctx, "bash", longResult, nil, 0)

	if got := maxRenderedWidth(rendered); got > renderer.maxWidth {
		t.Fatalf("expected rendered output to fit within %d columns, got %d", renderer.maxWidth, got)
	}
}

func TestRenderToolCallCompleteSearchSummaryUsesHeaderCount(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := "Found 2 matches:\nfoo.go:1:hello\nbar.go:2:world\n\n[TRUNCATED] Results truncated"

	rendered := renderer.RenderToolCallComplete(ctx, "ripgrep", result, nil, 0)

	if !strings.Contains(rendered, "2 matches") {
		t.Fatalf("expected header match count to be used, got %q", rendered)
	}
	if !strings.Contains(rendered, "truncated") {
		t.Fatalf("expected truncated marker, got %q", rendered)
	}
}

func TestRenderToolCallCompleteListFilesSummary(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := "[DIR]  docs\n[FILE] README.md (1024 bytes)\n[FILE] main.go (1024 bytes)\n"

	rendered := renderer.RenderToolCallComplete(ctx, "list_files", result, nil, 0)

	if !strings.Contains(rendered, "3 entries") {
		t.Fatalf("expected entry count summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "2 files") || !strings.Contains(rendered, "1 dir") {
		t.Fatalf("expected file/dir counts, got %q", rendered)
	}
	if !strings.Contains(rendered, "2.0 KB") {
		t.Fatalf("expected size summary, got %q", rendered)
	}
}

func TestRenderToolCallCompleteWebSearchSummary(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := "Search: golang\n\nSummary: fast\n\n2 Results:\n\n1. Go\n   URL: https://golang.org\n   desc\n\n2. Example\n   URL: https://example.com\n   desc"

	rendered := renderer.RenderToolCallComplete(ctx, "web_search", result, nil, 0)

	if !strings.Contains(rendered, "search \"golang\"") {
		t.Fatalf("expected search query summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "2 results") {
		t.Fatalf("expected results count, got %q", rendered)
	}
}

func TestRenderToolCallCompleteWebFetchSummary(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := "Source: https://example.com (cached)\n\nQuestion: What is it?\n\nAnalysis:\nfirst line\nsecond line"

	rendered := renderer.RenderToolCallComplete(ctx, "web_fetch", result, nil, 0)

	if !strings.Contains(rendered, "analyzed example.com") {
		t.Fatalf("expected analyzed host summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "cached") {
		t.Fatalf("expected cached marker, got %q", rendered)
	}
	if !strings.Contains(rendered, "2 lines") {
		t.Fatalf("expected line count, got %q", rendered)
	}
}

func TestRenderToolCallCompleteFileWriteSummary(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := "Wrote 128 bytes to /tmp/demo.txt"

	rendered := renderer.RenderToolCallComplete(ctx, "file_write", result, nil, 0)

	if !strings.Contains(rendered, "wrote /tmp/demo.txt") {
		t.Fatalf("expected write summary, got %q", rendered)
	}
	if !strings.Contains(rendered, "128 B") {
		t.Fatalf("expected size summary, got %q", rendered)
	}
}

func TestRenderToolCallCompleteAddsDurationSuffix(t *testing.T) {
	renderer := newTestRenderer(false)
	ctx := &types.OutputContext{Level: types.LevelCore}
	result := "No matches found"

	rendered := renderer.RenderToolCallComplete(ctx, "find", result, nil, 1500*time.Millisecond)

	if !strings.Contains(rendered, "(1.50s)") {
		t.Fatalf("expected duration suffix, got %q", rendered)
	}
}

func maxRenderedWidth(rendered string) int {
	maxWidth := 0
	for _, line := range strings.Split(rendered, "\n") {
		width := ansi.StringWidth(line)
		if width > maxWidth {
			maxWidth = width
		}
	}
	return maxWidth
}
