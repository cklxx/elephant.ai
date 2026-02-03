package codex

import (
	"bufio"
	"context"
	"encoding/json"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	agent "alex/internal/agent/ports/agent"
)

const testCodexMCPEnv = "ALEX_TEST_CODEX_MCP_SERVER"

func init() {
	if os.Getenv(testCodexMCPEnv) == "1" {
		runFakeCodexMCPServer()
		os.Exit(0)
	}
}

func runFakeCodexMCPServer() {
	scanner := bufio.NewScanner(os.Stdin)
	buf := make([]byte, 0, 64*1024)
	scanner.Buffer(buf, 1024*1024)

	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()

	for scanner.Scan() {
		line := scanner.Bytes()
		var msg map[string]any
		if err := json.Unmarshal(line, &msg); err != nil {
			continue
		}
		method, _ := msg["method"].(string)
		switch method {
		case "initialize":
			idStr, _ := msg["id"].(string)
			idNum, _ := strconv.Atoi(idStr)
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      idNum, // intentionally numeric
				"result": map[string]any{
					"protocolVersion": "2024-11-05",
					"serverInfo": map[string]any{
						"name":    "fake-codex",
						"version": "0.0.1",
					},
					"capabilities": map[string]any{
						"tools": map[string]any{},
					},
				},
			}
			writeJSONLine(w, resp)
		case "notifications/initialized":
			// Ignore.
		case "tools/call":
			idStr, _ := msg["id"].(string)
			idNum, _ := strconv.Atoi(idStr)

			// Emit noisy template (should be dropped).
			notify(w, "codex/event", map[string]any{
				"msg": map[string]any{
					"type": "raw_response_item",
					"raw_response_item": map[string]any{
						"role":    "developer",
						"content": "TEMPLATE_SHOULD_BE_DROPPED",
					},
				},
			})

			// Emit a delta.
			notify(w, "codex/event", map[string]any{
				"msg": map[string]any{
					"type":  "agent_message_delta",
					"delta": "working...",
				},
			})

			// Emit usage.
			notify(w, "codex/event", map[string]any{
				"msg": map[string]any{
					"type": "token_count",
					"info": map[string]any{
						"total_token_usage": map[string]any{
							"total_tokens": 123,
						},
					},
				},
			})

			result := map[string]any{
				"content": []any{
					map[string]any{"type": "text", "text": "final answer"},
				},
				"structuredContent": map[string]any{
					"thread_id": "thread-123",
				},
			}
			resp := map[string]any{
				"jsonrpc": "2.0",
				"id":      idNum,
				"result":  result,
			}
			writeJSONLine(w, resp)
		default:
			// Ignore.
		}
	}
}

func notify(w *bufio.Writer, method string, params map[string]any) {
	payload := map[string]any{
		"jsonrpc": "2.0",
		"method": method,
		"params": params,
	}
	writeJSONLine(w, payload)
}

func writeJSONLine(w *bufio.Writer, payload map[string]any) {
	b, err := json.Marshal(payload)
	if err != nil {
		return
	}
	_, _ = w.Write(append(b, '\n'))
	_ = w.Flush()
}

func TestExecutor_Execute_ReportsProgressAndMetadata(t *testing.T) {
	exec := New(Config{
		BinaryPath:   os.Args[0],
		Timeout:      5 * time.Second,
		DefaultModel: "",
		Env: map[string]string{
			testCodexMCPEnv: "1",
		},
	})

	var progress []agent.ExternalAgentProgress
	req := agent.ExternalAgentRequest{
		TaskID:    "t1",
		AgentType: "codex",
		Prompt:    "hello",
		OnProgress: func(p agent.ExternalAgentProgress) {
			progress = append(progress, p)
		},
	}

	res, err := exec.Execute(context.Background(), req)
	if err != nil {
		t.Fatalf("execute failed: %v", err)
	}
	if res == nil {
		t.Fatalf("expected result")
	}
	if strings.TrimSpace(res.Answer) != "final answer" {
		t.Fatalf("unexpected answer: %q", res.Answer)
	}
	if res.TokensUsed != 123 {
		t.Fatalf("unexpected tokensUsed: %d", res.TokensUsed)
	}
	if res.Metadata == nil {
		t.Fatalf("expected metadata")
	}
	if res.Metadata["thread_id"] != "thread-123" {
		t.Fatalf("unexpected thread_id: %#v", res.Metadata["thread_id"])
	}

	if len(progress) == 0 {
		t.Fatalf("expected progress callbacks")
	}
	for _, p := range progress {
		if strings.Contains(p.CurrentArgs, "TEMPLATE_SHOULD_BE_DROPPED") {
			t.Fatalf("template noise should not be forwarded via progress")
		}
	}
}
