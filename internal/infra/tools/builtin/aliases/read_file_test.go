package aliases

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

// newTestReadFile creates a readFile tool and a context rooted at tempDir.
func newTestReadFile(t *testing.T) (*readFile, context.Context, string) {
	t.Helper()
	baseDir := pathutil.DefaultWorkingDir()
	if baseDir == "" {
		t.Fatalf("default working dir is empty")
	}
	tempDir, err := os.MkdirTemp(baseDir, "read-file-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tempDir) })

	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)
	tool := NewReadFile(shared.FileToolConfig{}).(*readFile)
	return tool, ctx, tempDir
}

func writeTestFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatalf("write test file: %v", err)
	}
	return p
}

// --- Small file (under threshold) ---

func TestReadFileSmallFile(t *testing.T) {
	tool, ctx, dir := newTestReadFile(t)

	content := "line1\nline2\nline3\n"
	fp := writeTestFile(t, dir, "small.txt", content)

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-1",
		Name:      "read_file",
		Arguments: map[string]any{"path": fp},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}

	// Content returned unchanged.
	if result.Content != content {
		t.Fatalf("expected full content, got %q", result.Content)
	}

	// Metadata enriched.
	if result.Metadata["tool_name"] != "read_file" {
		t.Fatalf("expected tool_name=read_file, got %v", result.Metadata["tool_name"])
	}
	if result.Metadata["total_lines"] == nil {
		t.Fatalf("expected total_lines in metadata")
	}
	if result.Metadata["file_size_bytes"] == nil {
		t.Fatalf("expected file_size_bytes in metadata")
	}
}

// --- Line range slicing ---

func TestReadFileWithLineRange(t *testing.T) {
	tool, ctx, dir := newTestReadFile(t)

	var b strings.Builder
	for i := 0; i < 50; i++ {
		fmt.Fprintf(&b, "line %d\n", i)
	}
	fp := writeTestFile(t, dir, "ranged.txt", b.String())

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-2",
		Name: "read_file",
		Arguments: map[string]any{
			"path":       fp,
			"start_line": 10,
			"end_line":   15,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}

	lines := strings.Split(result.Content, "\n")
	if len(lines) != 5 {
		t.Fatalf("expected 5 lines, got %d: %q", len(lines), result.Content)
	}
	if !strings.Contains(lines[0], "line 10") {
		t.Fatalf("expected first line to be 'line 10', got %q", lines[0])
	}

	// shown_range should be present.
	sr, ok := result.Metadata["shown_range"]
	if !ok {
		t.Fatalf("expected shown_range in metadata")
	}
	arr, ok := sr.([2]int)
	if !ok {
		t.Fatalf("shown_range type mismatch: %T", sr)
	}
	if arr[0] != 10 || arr[1] != 15 {
		t.Fatalf("expected shown_range=[10,15], got %v", arr)
	}
}

// --- Large file preview (no range) ---

func TestReadFileLargeFilePreview(t *testing.T) {
	tool, ctx, dir := newTestReadFile(t)

	// Create a file > 50 KB with many lines.
	var b strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "line %04d: %s\n", i, strings.Repeat("x", 40))
	}
	content := b.String()
	if len(content) < readFileLargeFileThreshold {
		t.Fatalf("test setup: content should exceed 50KB, got %d bytes", len(content))
	}
	fp := writeTestFile(t, dir, "large.txt", content)

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-3",
		Name:      "read_file",
		Arguments: map[string]any{"path": fp},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}

	// Should contain preview hint.
	if !strings.Contains(result.Content, "[File preview:") {
		t.Fatalf("expected preview hint in content")
	}

	// Should show only first 200 lines of content (before the hint).
	beforeHint := result.Content[:strings.Index(result.Content, "\n\n[File preview:")]
	previewLines := strings.Split(beforeHint, "\n")
	if len(previewLines) != readFileMaxPreviewLines {
		t.Fatalf("expected %d preview lines, got %d", readFileMaxPreviewLines, len(previewLines))
	}

	// Metadata.
	if result.Metadata["preview_mode"] != true {
		t.Fatalf("expected preview_mode=true")
	}
	if result.Metadata["total_lines"].(int) != 5000 {
		t.Fatalf("expected total_lines=5000, got %v", result.Metadata["total_lines"])
	}

	// Hint should suggest next range.
	if !strings.Contains(result.Content, "start_line=200") {
		t.Fatalf("expected hint to suggest start_line=200, got: %s", result.Content[len(result.Content)-200:])
	}
}

// --- Large single-line file ---

func TestReadFileLargeSingleLine(t *testing.T) {
	tool, ctx, dir := newTestReadFile(t)

	// Single line > 50 KB.
	bigLine := strings.Repeat("{\"key\":\"value\"},", 5000) // ~75 KB
	fp := writeTestFile(t, dir, "minified.json", bigLine)

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-4",
		Name:      "read_file",
		Arguments: map[string]any{"path": fp},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}

	// Should return description, not the raw content.
	if !strings.Contains(result.Content, "[Large single-line file:") {
		t.Fatalf("expected single-line file description, got: %q", result.Content[:min(200, len(result.Content))])
	}
	// Should suggest shell_exec.
	if !strings.Contains(result.Content, "shell_exec") {
		t.Fatalf("expected shell_exec suggestion")
	}
	// Metadata flag.
	if result.Metadata["single_line_file"] != true {
		t.Fatalf("expected single_line_file=true in metadata")
	}
}

// --- Large file with explicit range bypasses preview ---

func TestReadFileLargeFileWithRange(t *testing.T) {
	tool, ctx, dir := newTestReadFile(t)

	// Create a file > 50 KB.
	var b strings.Builder
	for i := 0; i < 5000; i++ {
		fmt.Fprintf(&b, "line %04d: %s\n", i, strings.Repeat("x", 40))
	}
	fp := writeTestFile(t, dir, "large_ranged.txt", b.String())

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "call-5",
		Name: "read_file",
		Arguments: map[string]any{
			"path":       fp,
			"start_line": 100,
			"end_line":   110,
		},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("tool error: %v", result.Error)
	}

	// Should NOT be in preview mode — explicit range requested.
	if result.Metadata["preview_mode"] != nil {
		t.Fatalf("should not be in preview mode when range is specified")
	}
	// Should return exact 10 lines.
	lines := strings.Split(result.Content, "\n")
	if len(lines) != 10 {
		t.Fatalf("expected 10 lines, got %d", len(lines))
	}
	if !strings.Contains(lines[0], "line 0100") {
		t.Fatalf("expected first line to be 'line 0100', got %q", lines[0])
	}
}

// --- File not found ---

func TestReadFileNotFound(t *testing.T) {
	tool, ctx, dir := newTestReadFile(t)

	result, err := tool.Execute(ctx, ports.ToolCall{
		ID:        "call-6",
		Name:      "read_file",
		Arguments: map[string]any{"path": filepath.Join(dir, "nonexistent.txt")},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected error for non-existent file")
	}
}
