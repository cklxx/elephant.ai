package aliases

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
)

func TestWriteFileAutoUpload(t *testing.T) {
	t.Helper()

	baseDir := pathutil.DefaultWorkingDir()
	if baseDir == "" {
		t.Fatalf("default working dir is empty")
	}
	tempDir, err := os.MkdirTemp(baseDir, "alias-test-")
	if err != nil {
		t.Fatalf("create temp dir: %v", err)
	}
	t.Cleanup(func() {
		_ = os.RemoveAll(tempDir)
	})
	ctx := pathutil.WithWorkingDir(context.Background(), tempDir)
	ctx = shared.WithAutoUploadConfig(ctx, shared.AutoUploadConfig{
		Enabled:   true,
		MaxBytes:  1024,
		AllowExts: []string{".txt"},
	})

	tool := NewWriteFile(shared.FileToolConfig{})
	targetPath := filepath.Join(tempDir, "note.txt")
	call := ports.ToolCall{
		ID:   "call-1",
		Name: "write_file",
		Arguments: map[string]any{
			"path":    targetPath,
			"content": "hello",
		},
	}

	result, err := tool.Execute(ctx, call)
	if err != nil {
		t.Fatalf("write_file failed: %v", err)
	}
	if result == nil {
		t.Fatalf("expected non-nil result")
	}
	if result.Error != nil {
		t.Fatalf("write_file returned error: %v", result.Error)
	}
	if len(result.Attachments) == 0 {
		t.Fatalf("expected attachment to be uploaded")
	}
	if _, ok := result.Attachments["note.txt"]; !ok {
		t.Fatalf("expected attachment named note.txt")
	}
}
