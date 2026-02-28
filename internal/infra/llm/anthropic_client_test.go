package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
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

func TestAnthropicClientCapturesThinking(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":          "msg-3",
			"role":        "assistant",
			"stop_reason": "end_turn",
			"content": []any{
				map[string]any{"type": "thinking", "thinking": "brainstorm", "signature": "sig"},
				map[string]any{"type": "text", "text": "done"},
			},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 2,
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
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 {
		t.Fatalf("expected thinking parts, got %+v", resp.Thinking)
	}
	if resp.Thinking.Parts[0].Text != "brainstorm" || resp.Thinking.Parts[0].Signature != "sig" {
		t.Fatalf("unexpected thinking part: %+v", resp.Thinking.Parts[0])
	}
}

func TestAnthropicConvertMessagesEmbedsOnlyLatestUserAttachmentMessage(t *testing.T) {
	t.Parallel()

	client := &anthropicClient{}
	msgs := []ports.Message{
		{
			Role:    "user",
			Source:  ports.MessageSourceUserInput,
			Content: "first [old.png]",
			Attachments: map[string]ports.Attachment{
				"old.png": {Name: "old.png", MediaType: "image/png", Data: "ZmFrZQ=="},
			},
		},
		{Role: "assistant", Content: "ack"},
		{
			Role:    "user",
			Source:  ports.MessageSourceUserInput,
			Content: "latest [new.png]",
			Attachments: map[string]ports.Attachment{
				"new.png": {Name: "new.png", MediaType: "image/png", Data: "bmV3"},
			},
		},
	}

	converted, _ := client.convertMessages(msgs)
	if len(converted) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(converted))
	}

	for _, block := range converted[0].Content {
		if block.Type == "image" {
			t.Fatalf("expected first user message to stay text-only")
		}
	}

	foundImage := false
	for _, block := range converted[2].Content {
		if block.Type == "image" {
			foundImage = true
			break
		}
	}
	if !foundImage {
		t.Fatalf("expected latest user message to include image block")
	}
}

func TestNormalizeAnthropicMessages_MergesConsecutiveSameRole(t *testing.T) {
	t.Parallel()
	msgs := []anthropicMessage{
		{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "a"}}},
		{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "b"}}},
		{Role: "assistant", Content: []anthropicContentBlock{{Type: "text", Text: "c"}}},
	}
	result := normalizeAnthropicMessages(msgs)
	if len(result) != 3 { // merged user + assistant + trailing user
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "user" || len(result[0].Content) != 2 {
		t.Fatalf("expected merged user with 2 blocks, got role=%s blocks=%d", result[0].Role, len(result[0].Content))
	}
	if result[1].Role != "assistant" {
		t.Fatalf("expected assistant, got %s", result[1].Role)
	}
	// Trailing user added because assistant was last.
	if result[2].Role != "user" {
		t.Fatalf("expected trailing user, got %s", result[2].Role)
	}
}

func TestNormalizeAnthropicMessages_AddsLeadingUser(t *testing.T) {
	t.Parallel()
	msgs := []anthropicMessage{
		{Role: "assistant", Content: []anthropicContentBlock{{Type: "text", Text: "summary"}}},
		{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "hi"}}},
	}
	result := normalizeAnthropicMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Role != "user" || result[0].Content[0].Text != "Continue." {
		t.Fatalf("expected leading user placeholder, got %+v", result[0])
	}
	if result[1].Role != "assistant" {
		t.Fatalf("expected assistant at index 1, got %s", result[1].Role)
	}
	if result[2].Role != "user" || result[2].Content[0].Text != "hi" {
		t.Fatalf("expected original user at index 2, got %+v", result[2])
	}
}

func TestNormalizeAnthropicMessages_AddsTrailingUser(t *testing.T) {
	t.Parallel()
	msgs := []anthropicMessage{
		{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "hi"}}},
		{Role: "assistant", Content: []anthropicContentBlock{{Type: "text", Text: "done"}}},
	}
	result := normalizeAnthropicMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[2].Role != "user" || result[2].Content[0].Text != "Continue." {
		t.Fatalf("expected trailing user placeholder, got %+v", result[2])
	}
}

func TestNormalizeAnthropicMessages_NoChangeWhenValid(t *testing.T) {
	t.Parallel()
	msgs := []anthropicMessage{
		{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "hi"}}},
		{Role: "assistant", Content: []anthropicContentBlock{{Type: "text", Text: "ok"}}},
		{Role: "user", Content: []anthropicContentBlock{{Type: "text", Text: "bye"}}},
	}
	result := normalizeAnthropicMessages(msgs)
	if len(result) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(result))
	}
	if result[0].Content[0].Text != "hi" || result[2].Content[0].Text != "bye" {
		t.Fatalf("expected original content preserved")
	}
}

func TestNormalizeAnthropicMessages_Empty(t *testing.T) {
	t.Parallel()
	result := normalizeAnthropicMessages(nil)
	if len(result) != 0 {
		t.Fatalf("expected empty, got %d", len(result))
	}
}

func TestAnthropicConvertMessages_TrailingAssistantGetsPaddedUser(t *testing.T) {
	t.Parallel()
	client := &anthropicClient{}
	msgs := []ports.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "summary of context"},
	}
	converted, _ := client.convertMessages(msgs)
	if len(converted) != 3 {
		t.Fatalf("expected 3 messages (user, assistant, trailing user), got %d", len(converted))
	}
	if converted[2].Role != "user" {
		t.Fatalf("expected trailing user, got %s", converted[2].Role)
	}
}

func TestParseAnthropicContentThinkingField(t *testing.T) {
	t.Parallel()

	// Anthropic API returns thinking content in the "thinking" JSON field,
	// not "text". Verify parseAnthropicContent reads the correct field.
	blocks := []anthropicContentBlock{
		{Type: "thinking", Thinking: "deep reasoning here", Signature: "sig123"},
		{Type: "text", Text: "final answer"},
	}

	content, toolCalls, thinking := parseAnthropicContent(blocks)
	if content != "final answer" {
		t.Fatalf("unexpected content: %q", content)
	}
	if len(toolCalls) != 0 {
		t.Fatalf("unexpected tool calls: %v", toolCalls)
	}
	if len(thinking.Parts) != 1 {
		t.Fatalf("expected 1 thinking part, got %d", len(thinking.Parts))
	}
	if thinking.Parts[0].Text != "deep reasoning here" {
		t.Fatalf("expected thinking text from 'thinking' field, got %q", thinking.Parts[0].Text)
	}
	if thinking.Parts[0].Signature != "sig123" {
		t.Fatalf("unexpected signature: %q", thinking.Parts[0].Signature)
	}
}

func TestParseAnthropicContentThinkingFallsBackToText(t *testing.T) {
	t.Parallel()

	// When "thinking" field is empty but "text" is populated (legacy/other providers),
	// parseAnthropicContent should fall back to "text".
	blocks := []anthropicContentBlock{
		{Type: "thinking", Text: "legacy thinking", Signature: "sig-old"},
		{Type: "text", Text: "answer"},
	}

	_, _, thinking := parseAnthropicContent(blocks)
	if len(thinking.Parts) != 1 {
		t.Fatalf("expected 1 thinking part, got %d", len(thinking.Parts))
	}
	if thinking.Parts[0].Text != "legacy thinking" {
		t.Fatalf("expected fallback to text field, got %q", thinking.Parts[0].Text)
	}
}

func TestAnthropicClientUsesClaudeSetupOAuthToken(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer sk-ant-oat01-fake-token" {
			t.Fatalf("expected OAuth Bearer header for sk-ant-oat token, got %q", got)
		}
		if got := r.Header.Get(anthropicRequestHeaderKey); got != "" {
			t.Fatalf("expected no x-api-key header for OAuth token, got %q", got)
		}
		beta := r.Header.Get(anthropicBetaHeaderKey)
		if !strings.Contains(beta, anthropicOAuthBetaHeader) {
			t.Fatalf("expected oauth beta header, got %q", beta)
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":          "msg-setup",
			"role":        "assistant",
			"stop_reason": "end_turn",
			"content":     []any{map[string]any{"type": "text", "text": "ok"}},
			"usage":       map[string]any{"input_tokens": 1, "output_tokens": 1},
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))

	client, err := NewAnthropicClient("claude-sonnet-4-6-20250514", Config{
		APIKey:  "sk-ant-oat01-fake-token",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewAnthropicClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}
	if resp.Content != "ok" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
}
