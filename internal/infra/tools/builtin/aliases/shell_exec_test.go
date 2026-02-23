package aliases

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

func newShellExecTestContext(t *testing.T) (context.Context, string) {
	t.Helper()

	execDir, err := os.MkdirTemp(".", "shell-exec-dir-")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(execDir) })

	execDirAbs, err := filepath.Abs(execDir)
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}
	return pathutil.WithWorkingDir(context.Background(), execDirAbs), execDirAbs
}

func TestShellExecAttachmentsRequireEnabledAutoUpload(t *testing.T) {
	if !localExecEnabled {
		t.Skip("local shell execution disabled")
	}

	baseCtx, execDirAbs := newShellExecTestContext(t)
	outPath := filepath.Join(execDirAbs, "result.txt")

	tool := NewShellExec(shared.ShellToolConfig{})
	call := ports.ToolCall{
		ID:   "call-1",
		Name: "shell_exec",
		Arguments: map[string]any{
			"command":  "echo shell-attachment > result.txt",
			"exec_dir": execDirAbs,
			"attachments": []any{
				outPath,
			},
		},
	}

	ctxDisabled := shared.WithAutoUploadConfig(baseCtx, shared.AutoUploadConfig{
		Enabled:   false,
		MaxBytes:  1024,
		AllowExts: []string{".txt"},
	})
	disabledRes, err := tool.Execute(ctxDisabled, call)
	if err != nil {
		t.Fatalf("disabled Execute returned error: %v", err)
	}
	if disabledRes == nil {
		t.Fatalf("disabled Execute returned nil result")
	}
	if disabledRes.Error != nil {
		t.Fatalf("disabled tool execution failed: %v", disabledRes.Error)
	}
	if len(disabledRes.Attachments) != 0 {
		t.Fatalf("expected no attachments when auto upload disabled, got %d", len(disabledRes.Attachments))
	}
	if errs, ok := disabledRes.Metadata["attachment_errors"].([]string); !ok || len(errs) == 0 {
		t.Fatalf("expected attachment_errors when auto upload disabled, got %#v", disabledRes.Metadata["attachment_errors"])
	}

	ctxEnabled := shared.WithAutoUploadConfig(baseCtx, shared.AutoUploadConfig{
		Enabled:   true,
		MaxBytes:  1024,
		AllowExts: []string{".txt"},
	})
	enabledRes, err := tool.Execute(ctxEnabled, call)
	if err != nil {
		t.Fatalf("enabled Execute returned error: %v", err)
	}
	if enabledRes == nil {
		t.Fatalf("enabled Execute returned nil result")
	}
	if enabledRes.Error != nil {
		t.Fatalf("enabled tool execution failed: %v", enabledRes.Error)
	}
	if len(enabledRes.Attachments) != 1 {
		t.Fatalf("expected 1 attachment when auto upload enabled, got %d", len(enabledRes.Attachments))
	}
	if _, ok := enabledRes.Attachments["result.txt"]; !ok {
		t.Fatalf("expected result.txt attachment, got %#v", enabledRes.Attachments)
	}
}
