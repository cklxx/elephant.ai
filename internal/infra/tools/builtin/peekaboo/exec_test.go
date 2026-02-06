package peekaboo

import (
	"context"
	"encoding/base64"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

type recordingRunner struct {
	called bool
	binary string
	args   []string
	env    []string
	dir    string

	stdout   []byte
	stderr   []byte
	exitCode int
	err      error

	hook func(dir string) error
}

func (r *recordingRunner) Run(_ context.Context, binary string, args []string, env []string, dir string) ([]byte, []byte, int, error) {
	r.called = true
	r.binary = binary
	r.args = append([]string(nil), args...)
	r.env = append([]string(nil), env...)
	r.dir = dir

	if r.hook != nil {
		if err := r.hook(dir); err != nil {
			return nil, nil, -1, err
		}
	}

	return r.stdout, r.stderr, r.exitCode, r.err
}

func TestPeekabooExecArgsRequired(t *testing.T) {
	tool := newPeekabooExec("true", &recordingRunner{})
	res, err := tool.Execute(context.Background(), ports.ToolCall{ID: "1", Arguments: map[string]any{}})
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if res == nil || res.Error == nil {
		t.Fatalf("expected ToolResult error, got %#v", res)
	}
}

func TestPeekabooExecUsesTempDirByDefault(t *testing.T) {
	runner := &recordingRunner{
		hook: func(dir string) error {
			return os.WriteFile(filepath.Join(dir, "out.png"), []byte("hello"), 0o644)
		},
		exitCode: 0,
	}
	tool := newPeekabooExec("true", runner)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "1",
		Arguments: map[string]any{
			"args": []any{"see"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if !runner.called {
		t.Fatalf("expected runner to be called")
	}
	if runner.dir == "" {
		t.Fatalf("expected temp dir to be passed to runner")
	}
	if !strings.HasPrefix(filepath.Base(runner.dir), "alex-peekaboo-") {
		t.Fatalf("expected temp dir prefix alex-peekaboo-, got %q", runner.dir)
	}
	if _, err := os.Stat(runner.dir); err == nil || !os.IsNotExist(err) {
		t.Fatalf("expected temp dir to be removed after execution, stat err=%v", err)
	}
	if res.Attachments == nil {
		t.Fatalf("expected attachments to be present")
	}
	att, ok := res.Attachments["out.png"]
	if !ok {
		t.Fatalf("expected out.png attachment, got keys=%v", keys(res.Attachments))
	}
	if att.MediaType != "image/png" {
		t.Fatalf("expected image/png, got %q", att.MediaType)
	}
	if att.Data != base64.StdEncoding.EncodeToString([]byte("hello")) {
		t.Fatalf("unexpected attachment data: %q", att.Data)
	}
	if !strings.Contains(res.Content, "[out.png]") {
		t.Fatalf("expected content to mention [out.png], got %q", res.Content)
	}
}

func TestPeekabooExecParsesJSONStdout(t *testing.T) {
	runner := &recordingRunner{
		stdout:   []byte(`{"ok":true}`),
		exitCode: 0,
	}
	tool := newPeekabooExec("true", runner)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "1",
		Arguments: map[string]any{
			"args": []any{"list"},
		},
	})
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if res == nil || res.Metadata == nil {
		t.Fatalf("expected metadata to be present")
	}
	jsonMeta, ok := res.Metadata["json"].(map[string]any)
	if !ok {
		t.Fatalf("expected metadata[json] to be a map, got %#v", res.Metadata["json"])
	}
	if okVal, ok := jsonMeta["ok"].(bool); !ok || !okVal {
		t.Fatalf("expected json.ok=true, got %#v", jsonMeta["ok"])
	}
}

func TestPeekabooExecMaxAttachments(t *testing.T) {
	runner := &recordingRunner{
		hook: func(dir string) error {
			for i := 0; i < 10; i++ {
				name := filepath.Join(dir, "out"+string(rune('0'+i))+".png")
				if err := os.WriteFile(name, []byte("x"), 0o644); err != nil {
					return err
				}
			}
			return nil
		},
		exitCode: 0,
	}
	tool := newPeekabooExec("true", runner)
	res, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "1",
		Arguments: map[string]any{
			"args":            []any{"see"},
			"max_attachments": 8,
		},
	})
	if err != nil {
		t.Fatalf("Execute() returned unexpected error: %v", err)
	}
	if len(res.Attachments) != 8 {
		t.Fatalf("expected 8 attachments, got %d", len(res.Attachments))
	}
	if truncated, ok := res.Metadata["truncated"].(bool); !ok || !truncated {
		t.Fatalf("expected metadata.truncated=true, got %#v", res.Metadata["truncated"])
	}
	attached, ok := res.Metadata["attached_files"].([]string)
	if !ok {
		t.Fatalf("expected metadata.attached_files to be []string, got %#v", res.Metadata["attached_files"])
	}
	if len(attached) != 8 {
		t.Fatalf("expected 8 attached_files, got %d", len(attached))
	}
	if attached[0] != "out0.png" || attached[7] != "out7.png" {
		t.Fatalf("unexpected attached_files ordering: %v", attached)
	}
}

func keys(m map[string]ports.Attachment) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	return out
}
