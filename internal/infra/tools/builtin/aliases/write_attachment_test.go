package aliases

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/shared"
)

func TestWriteAttachmentWritesDataURI(t *testing.T) {
	t.Parallel()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	testDir := filepath.Join(cwd, "tmp", "write-attachment-test")
	if err := os.MkdirAll(testDir, 0o755); err != nil {
		t.Fatalf("mkdir test dir: %v", err)
	}
	outPath := filepath.Join(testDir, "note.txt")
	t.Cleanup(func() {
		_ = os.Remove(outPath)
	})

	tool := NewWriteAttachment(shared.FileToolConfig{})
	call := ports.ToolCall{
		ID:   "call-1",
		Name: "write_attachment",
		Arguments: map[string]any{
			"attachment": "data:text/plain;base64," + base64.StdEncoding.EncodeToString([]byte("hello")),
			"path":       outPath,
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("expected successful result, got %+v", result)
	}
	data, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("read output file: %v", err)
	}
	if string(data) != "hello" {
		t.Fatalf("unexpected output content: %q", string(data))
	}
}

func TestWriteAttachmentRejectsEmptyAttachment(t *testing.T) {
	t.Parallel()
	tool := NewWriteAttachment(shared.FileToolConfig{})
	call := ports.ToolCall{
		ID:   "call-2",
		Name: "write_attachment",
		Arguments: map[string]any{
			"path": "/tmp/should-not-exist.txt",
		},
	}

	result, err := tool.Execute(context.Background(), call)
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if result == nil || result.Error == nil {
		t.Fatalf("expected validation error, got %+v", result)
	}
	if !strings.Contains(result.Error.Error(), "attachment is required") {
		t.Fatalf("unexpected error: %v", result.Error)
	}
}
