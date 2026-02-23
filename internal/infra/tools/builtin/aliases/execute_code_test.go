package aliases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
	"alex/internal/infra/tools/builtin/pathutil"
	"alex/internal/infra/tools/builtin/shared"
)

func newExecuteCodeTestContext(t *testing.T) (context.Context, string) {
	t.Helper()

	execDir, err := os.MkdirTemp(".", "exec-dir-")
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

func TestExecuteCodeInlineUsesSystemTempDir(t *testing.T) {
	if !localExecEnabled {
		t.Skip("local code execution disabled")
	}

	ctx, execDirAbs := newExecuteCodeTestContext(t)

	tool := NewExecuteCode(shared.ShellToolConfig{})
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "1",
		Name: "execute_code",
		Arguments: map[string]any{
			"language": "bash",
			"code":     "echo hello",
			"exec_dir": execDirAbs,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if res == nil {
		t.Fatalf("Execute returned nil result")
	}
	if res.Error != nil {
		t.Fatalf("tool execution failed: %v", res.Error)
	}

	codePath, _ := res.Metadata["code_path"].(string)
	if strings.TrimSpace(codePath) == "" {
		t.Fatalf("missing code_path in metadata: %#v", res.Metadata)
	}
	if codePath == execDirAbs || strings.HasPrefix(codePath, execDirAbs+string(os.PathSeparator)) {
		t.Fatalf("expected code_path outside exec_dir; exec_dir=%q code_path=%q", execDirAbs, codePath)
	}

	entries, err := os.ReadDir(execDirAbs)
	if err != nil {
		t.Fatalf("ReadDir failed: %v", err)
	}
	for _, entry := range entries {
		if entry.IsDir() && strings.HasPrefix(entry.Name(), "alex-exec-") {
			t.Fatalf("unexpected alex-exec-* dir created under exec_dir: %s", filepath.Join(execDirAbs, entry.Name()))
		}
	}
}

func TestExecuteCodeCodePathDoesNotUseShellInterpolation(t *testing.T) {
	if !localExecEnabled {
		t.Skip("local code execution disabled")
	}

	ctx, execDirAbs := newExecuteCodeTestContext(t)
	codePath := filepath.Join(execDirAbs, "script;echo INJECTED_MARKER.sh")
	if err := os.WriteFile(codePath, []byte("#!/bin/bash\necho SAFE_MARKER\n"), 0o755); err != nil {
		t.Fatalf("WriteFile failed: %v", err)
	}

	tool := NewExecuteCode(shared.ShellToolConfig{})
	res, err := tool.Execute(ctx, ports.ToolCall{
		ID:   "2",
		Name: "execute_code",
		Arguments: map[string]any{
			"language":  "bash",
			"code_path": codePath,
			"exec_dir":  execDirAbs,
		},
	})
	if err != nil {
		t.Fatalf("Execute returned error: %v", err)
	}
	if res == nil {
		t.Fatalf("Execute returned nil result")
	}
	if res.Error != nil {
		t.Fatalf("tool execution failed: %v", res.Error)
	}
	if !strings.Contains(res.Content, "SAFE_MARKER") {
		t.Fatalf("expected script output, got: %q", res.Content)
	}
	if strings.Contains(res.Content, "INJECTED_MARKER") {
		t.Fatalf("unexpected shell interpolation marker in output: %q", res.Content)
	}
}

func TestExecuteCodeAttachmentsRequireEnabledAutoUpload(t *testing.T) {
	if !localExecEnabled {
		t.Skip("local code execution disabled")
	}

	baseCtx, execDirAbs := newExecuteCodeTestContext(t)
	outPath := filepath.Join(execDirAbs, "result.txt")

	tool := NewExecuteCode(shared.ShellToolConfig{})
	call := ports.ToolCall{
		ID:   "3",
		Name: "execute_code",
		Arguments: map[string]any{
			"language": "bash",
			"code":     "echo attachment > result.txt",
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
