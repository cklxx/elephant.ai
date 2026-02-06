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

func TestFileEditCreatesNewFile(t *testing.T) {
	root := pathutil.DefaultWorkingDir()
	tempDir, err := os.MkdirTemp(root, "file-edit-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)
	tool := &fileEdit{}

	call := ports.ToolCall{
		ID: "call-create",
		Arguments: map[string]any{
			"file_path":  "note.txt",
			"old_string": "",
			"new_string": "hello",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got %v", result.Error)
	}

	contents, err := os.ReadFile(filepath.Join(tempDir, "note.txt"))
	if err != nil {
		t.Fatalf("expected file to exist: %v", err)
	}
	if string(contents) != "hello" {
		t.Fatalf("unexpected file contents: %q", contents)
	}
	if result.Metadata["operation"] != "created" {
		t.Fatalf("expected operation metadata to be created, got %v", result.Metadata["operation"])
	}
}

func TestFileEditUpdatesExistingFile(t *testing.T) {
	root := pathutil.DefaultWorkingDir()
	tempDir, err := os.MkdirTemp(root, "file-edit-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	path := filepath.Join(tempDir, "note.txt")
	if err := os.WriteFile(path, []byte("hello world"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)
	tool := &fileEdit{}
	call := ports.ToolCall{
		ID: "call-edit",
		Arguments: map[string]any{
			"file_path":  "note.txt",
			"old_string": "world",
			"new_string": "alex",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error != nil {
		t.Fatalf("expected no tool error, got %v", result.Error)
	}

	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read updated file: %v", err)
	}
	if string(contents) != "hello alex" {
		t.Fatalf("unexpected updated contents: %q", contents)
	}
}

func TestFileEditRejectsNonUniqueOldString(t *testing.T) {
	root := pathutil.DefaultWorkingDir()
	tempDir, err := os.MkdirTemp(root, "file-edit-")
	if err != nil {
		t.Fatalf("mkdir temp: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	path := filepath.Join(tempDir, "note.txt")
	if err := os.WriteFile(path, []byte("hello hello"), 0o644); err != nil {
		t.Fatalf("write seed file: %v", err)
	}

	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)
	tool := &fileEdit{}
	call := ports.ToolCall{
		ID: "call-duplicate",
		Arguments: map[string]any{
			"file_path":  "note.txt",
			"old_string": "hello",
			"new_string": "hi",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("Execute returned unexpected error: %v", err)
	}
	if result.Error == nil {
		t.Fatalf("expected tool error for duplicate old_string")
	}
	if !strings.Contains(result.Error.Error(), "appears") {
		t.Fatalf("unexpected error message: %v", result.Error)
	}
}
