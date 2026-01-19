package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"alex/internal/agent/ports"
)

func TestAnthropicClientCompleteSuccess(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.URL.Path; got != "/messages" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.Header.Get(anthropicRequestHeaderKey); got != "sk-ant-test" {
			t.Fatalf("expected api key header, got %q", got)
		}
		if got := r.Header.Get(anthropicVersionHeaderKey); got == "" {
			t.Fatalf("expected anthropic version header")
		}
		if got := r.Header.Get(anthropicBetaHeaderKey); got != anthropicToolsBetaHeader {
			t.Fatalf("expected tools beta header, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "claude-test" {
			t.Fatalf("unexpected model: %v", payload["model"])
		}
		if payload["system"] != "system rules" {
			t.Fatalf("unexpected system prompt: %v", payload["system"])
		}

		rawMsgs, ok := payload["messages"].([]any)
		if !ok || len(rawMsgs) != 3 {
			t.Fatalf("expected 3 messages, got %#v", payload["messages"])
		}

		assistant := rawMsgs[1].(map[string]any)
		if assistant["role"] != "assistant" {
			t.Fatalf("expected assistant role, got %#v", assistant["role"])
		}
		contentBlocks, ok := assistant["content"].([]any)
		if !ok {
			t.Fatalf("expected assistant content blocks, got %#v", assistant["content"])
		}
		foundToolUse := false
		for _, block := range contentBlocks {
			entry, _ := block.(map[string]any)
			if entry["type"] == "tool_use" {
				foundToolUse = true
				if entry["name"] != "toolName" {
					t.Fatalf("unexpected tool_use name: %#v", entry["name"])
				}
			}
		}
		if !foundToolUse {
			t.Fatalf("expected tool_use block in assistant content")
		}

		toolResult := rawMsgs[2].(map[string]any)
		if toolResult["role"] != "user" {
			t.Fatalf("expected tool result role user, got %#v", toolResult["role"])
		}
		trContent, ok := toolResult["content"].([]any)
		if !ok || len(trContent) == 0 {
			t.Fatalf("expected tool result content block, got %#v", toolResult["content"])
		}
		firstBlock := trContent[0].(map[string]any)
		if firstBlock["type"] != "tool_result" {
			t.Fatalf("expected tool_result block, got %#v", firstBlock["type"])
		}
		if !strings.Contains(firstBlock["content"].(string), "ok") {
			t.Fatalf("expected tool result content, got %#v", firstBlock["content"])
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":          "msg-1",
			"role":        "assistant",
			"stop_reason": "end_turn",
			"content": []any{
				map[string]any{"type": "text", "text": "hello"},
				map[string]any{
					"type": "tool_use",
					"id":   "call-2",
					"name": "toolName",
					"input": map[string]any{
						"foo": "bar",
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  4,
				"output_tokens": 6,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewAnthropicClient("claude-test", Config{
		APIKey:  "sk-ant-test",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewAnthropicClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "system rules"},
			{Role: "user", Content: "hi"},
			{
				Role: "assistant",
				ToolCalls: []ports.ToolCall{
					{ID: "call-1", Name: "toolName", Arguments: map[string]any{"foo": "bar"}},
				},
			},
			{Role: "tool", Content: "ok", ToolCallID: "call-1"},
		},
		Tools: []ports.ToolDefinition{
			{
				Name:        "toolName",
				Description: "desc",
				Parameters:  ports.ParameterSchema{Type: "object"},
			},
		},
		MaxTokens: 128,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "hello" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 10 {
		t.Fatalf("unexpected usage: %+v", resp.Usage)
	}
	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}
	if resp.ToolCalls[0].Name != "toolName" {
		t.Fatalf("unexpected tool call name: %s", resp.ToolCalls[0].Name)
	}
	if resp.ToolCalls[0].Arguments["foo"] != "bar" {
		t.Fatalf("unexpected tool call args: %+v", resp.ToolCalls[0].Arguments)
	}
}

func TestAnthropicClientUsesOAuthToken(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer oauth-token" {
			t.Fatalf("expected oauth Authorization header, got %q", got)
		}
		if got := r.Header.Get(anthropicRequestHeaderKey); got != "" {
			t.Fatalf("expected no api key header, got %q", got)
		}
		beta := r.Header.Get(anthropicBetaHeaderKey)
		if !strings.Contains(beta, anthropicOAuthBetaHeader) {
			t.Fatalf("expected oauth beta header, got %q", beta)
		}
		if !strings.Contains(beta, anthropicToolsBetaHeader) {
			t.Fatalf("expected tools beta header, got %q", beta)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":          "msg-2",
			"role":        "assistant",
			"stop_reason": "end_turn",
			"content": []any{
				map[string]any{"type": "text", "text": "ok"},
			},
			"usage": map[string]any{
				"input_tokens":  2,
				"output_tokens": 3,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewAnthropicClient("claude-test", Config{
		APIKey:  "oauth-token",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewAnthropicClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{
			{Name: "toolName", Description: "desc", Parameters: ports.ParameterSchema{Type: "object"}},
		},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
}
