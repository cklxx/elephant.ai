package fileops

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/pathutil"
)

func TestListFilesIncludesDirsAndFiles(t *testing.T) {
	root := pathutil.DefaultWorkingDir()
	tempDir, err := os.MkdirTemp(root, "list-files-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	if err := os.Mkdir(filepath.Join(tempDir, "docs"), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(tempDir, "readme.txt"), []byte("hello"), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}

	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)
	tool := &listFiles{}
	call := ports.ToolCall{
		ID: "call-list",
		Arguments: map[string]any{
			"path": ".",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got %v", result.Error)
	}

	if !strings.Contains(result.Content, "[DIR]  docs") {
		t.Fatalf("expected directory entry, got %q", result.Content)
	}
	if !strings.Contains(result.Content, "[FILE] readme.txt") {
		t.Fatalf("expected file entry, got %q", result.Content)
	}
}
