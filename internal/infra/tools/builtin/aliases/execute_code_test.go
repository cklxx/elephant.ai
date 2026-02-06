package aliases

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/tools/builtin/execution"
	"alex/internal/tools/builtin/pathutil"
	"alex/internal/tools/builtin/shared"
)

func TestExecuteCodeInlineUsesSystemTempDir(t *testing.T) {
	if !execution.LocalExecEnabled {
		t.Skip("local code execution disabled")
	}

	// exec_dir must stay within the current working directory due to local-path
	// guards; create a temp dir under the package directory instead of /tmp.
	execDir, err := os.MkdirTemp(".", "exec-dir-")
	if err != nil {
		t.Fatalf("MkdirTemp failed: %v", err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(execDir) })

	execDirAbs, err := filepath.Abs(execDir)
	if err != nil {
		t.Fatalf("Abs failed: %v", err)
	}

	ctx := pathutil.WithWorkingDir(context.Background(), execDirAbs)

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
