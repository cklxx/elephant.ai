package builtin

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"alex/internal/agent/ports"
	"alex/internal/sandbox"
)

func TestSandboxCodeExecuteWritesAndRuns(t *testing.T) {
	var wrote bool
	var wrotePath string
	var wroteContent string
	var command string

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/file/write":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			wrote = true
			wrotePath, _ = payload["file"].(string)
			wroteContent, _ = payload["content"].(string)
			response := sandbox.Response[sandbox.FileWriteResult]{
				Success: true,
				Data: &sandbox.FileWriteResult{
					File: wrotePath,
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		case "/v1/shell/exec":
			var payload map[string]any
			if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
			command, _ = payload["command"].(string)
			exitCode := 0
			output := "ok"
			response := sandbox.Response[sandbox.ShellCommandResult]{
				Success: true,
				Data: &sandbox.ShellCommandResult{
					SessionID: "sandbox-1",
					Command:   command,
					Status:    "completed",
					Output:    &output,
					ExitCode:  &exitCode,
				},
			}
			_ = json.NewEncoder(w).Encode(response)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)

	tool := NewSandboxCodeExecute(SandboxConfig{BaseURL: server.URL})
	result, err := tool.Execute(context.Background(), ports.ToolCall{
		ID: "call-1",
		Arguments: map[string]any{
			"language": "python",
			"code":     "print('hi')",
		},
	})
	if err != nil {
		t.Fatalf("Execute error: %v", err)
	}
	if result == nil || result.Error != nil {
		t.Fatalf("unexpected result: %+v", result)
	}
	if !wrote {
		t.Fatalf("expected code to be written to sandbox")
	}
	if !strings.HasSuffix(wrotePath, ".py") {
		t.Fatalf("expected python file extension, got %q", wrotePath)
	}
	if !strings.Contains(wroteContent, "print('hi')") {
		t.Fatalf("unexpected written content: %s", wroteContent)
	}
	if !strings.Contains(command, "python3") {
		t.Fatalf("expected python3 command, got %q", command)
	}
	if !strings.Contains(result.Content, "Command status") {
		t.Fatalf("unexpected content: %s", result.Content)
	}
}
