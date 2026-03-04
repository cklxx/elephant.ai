package llm

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alex/internal/domain/agent/ports"
	alexerrors "alex/internal/shared/errors"
	"alex/internal/shared/utils"
)

func TestOpenAIClientCompleteSuccess(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}

		if got := r.URL.Path; got != "/chat/completions" {
			t.Fatalf("unexpected path: %s", got)
		}

		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Fatalf("expected Content-Type header, got %q", got)
		}

		if got := r.Header.Get("X-Retry-Limit"); got != "2" {
			t.Fatalf("expected X-Retry-Limit header, got %q", got)
		}

		if got := r.Header.Get("X-Custom"); got != "value" {
			t.Fatalf("expected custom header, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if payload["model"] != "test-model" {
			t.Fatalf("unexpected model: %v", payload["model"])
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": "hello",
						"tool_calls": []any{
							map[string]any{
								"id":   "call-1",
								"type": "function",
								"function": map[string]any{
									"name":      "toolName",
									"arguments": `{"foo":"bar"}`,
								},
							},
						},
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     3,
				"completion_tokens": 4,
				"total_tokens":      7,
			},
		}

		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIClient("test-model", Config{
		APIKey:     "test-key",
		BaseURL:    server.URL,
		Timeout:    1,
		MaxRetries: 2,
		Headers: map[string]string{
			"X-Custom": "value",
		},
	})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}

	req := ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	}

	resp, err := client.Complete(context.Background(), req)
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "hello" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}

	if resp.StopReason != "stop" {
		t.Fatalf("unexpected stop reason: %q", resp.StopReason)
	}

	if resp.Usage.TotalTokens != 7 {
		t.Fatalf("unexpected tokens: %+v", resp.Usage)
	}

	if len(resp.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(resp.ToolCalls))
	}

	if resp.ToolCalls[0].Name != "toolName" {
		t.Fatalf("unexpected tool call name: %s", resp.ToolCalls[0].Name)
	}

	if resp.ToolCalls[0].Arguments["foo"] != "bar" {
		t.Fatalf("unexpected tool call arguments: %+v", resp.ToolCalls[0].Arguments)
	}
}

func TestOpenAIClientStreamComplete(t *testing.T) {
	t.Parallel()

	streamPayloads := []string{
		"data: {\"choices\":[{\"delta\":{\"content\":\"Hello\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{\"content\":\" world\"}}]}\n\n",
		"data: {\"choices\":[{\"delta\":{},\"finish_reason\":\"stop\"}],\"usage\":{\"prompt_tokens\":3,\"completion_tokens\":4,\"total_tokens\":7}}\n\n",
		"data: [DONE]\n\n",
	}

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		for _, payload := range streamPayloads {
			if _, err := io.WriteString(w, payload); err != nil {
				t.Fatalf("failed to write payload: %v", err)
			}
			if flusher != nil {
				flusher.Flush()
			}
		}
	}))

	clientIface, err := NewOpenAIClient("test-model", Config{APIKey: "test-key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}

	client := clientIface.(*openaiClient)

	var deltas []ports.ContentDelta
	callbacks := ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	}

	resp, err := client.StreamComplete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	}, callbacks)
	if err != nil {
		t.Fatalf("StreamComplete: %v", err)
	}

	if got, want := resp.Content, "Hello world"; got != want {
		t.Fatalf("expected content %q, got %q", want, got)
	}

	if resp.StopReason != "stop" {
		t.Fatalf("expected stop reason 'stop', got %q", resp.StopReason)
	}

	if resp.Usage.TotalTokens != 7 {
		t.Fatalf("expected usage total 7, got %+v", resp.Usage)
	}

	if len(deltas) < 3 {
		t.Fatalf("expected at least 3 deltas (including final), got %d", len(deltas))
	}

	if deltas[0].Delta != "Hello" || deltas[0].Final {
		t.Fatalf("unexpected first delta: %+v", deltas[0])
	}

	if deltas[1].Delta != " world" || deltas[1].Final {
		t.Fatalf("unexpected second delta: %+v", deltas[1])
	}

	lastDelta := deltas[len(deltas)-1]
	if !lastDelta.Final {
		t.Fatalf("expected final delta to have Final=true, got %+v", lastDelta)
	}
}

func TestOpenAIClientCompleteTimeout(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"choices":[{"message":{"content":"slow"},"finish_reason":"stop"}],"usage":{}}`))
	}))

	clientIface, err := NewOpenAIClient("test-model", Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}

	client := clientIface.(*openaiClient)
	client.httpClient.Timeout = 50 * time.Millisecond

	_, err = client.Complete(context.Background(), ports.CompletionRequest{})
	if err == nil {
		t.Fatalf("expected timeout error")
	}

	var transient *alexerrors.TransientError
	if !errors.As(err, &transient) {
		t.Fatalf("expected transient error, got %T", err)
	}
}

func TestOpenAIClientCompleteInvalidAPIKey(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		_, _ = w.Write([]byte(`{"error":{"message":"invalid api key"}}`))
	}))

	client, err := NewOpenAIClient("test-model", Config{APIKey: "bad", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}

	var perr *alexerrors.PermanentError
	if !errors.As(err, &perr) {
		t.Fatalf("expected permanent error, got %T", err)
	}

	if perr.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, perr.StatusCode)
	}
}

func TestOpenAIClientCompleteQuotaExceeded(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Retry-After", "3")
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"quota exceeded"}}`))
	}))

	client, err := NewOpenAIClient("test-model", Config{APIKey: "key", BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewOpenAIClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{})
	if err == nil {
		t.Fatalf("expected error")
	}

	var terr *alexerrors.TransientError
	if !errors.As(err, &terr) {
		t.Fatalf("expected transient error, got %T", err)
	}

	if terr.RetryAfter != 3 {
		t.Fatalf("expected retry-after 3, got %d", terr.RetryAfter)
	}

	if terr.StatusCode != http.StatusTooManyRequests {
		t.Fatalf("expected status %d, got %d", http.StatusTooManyRequests, terr.StatusCode)
	}
}

func TestOpenAIClientBuildOpenAIRequestStreamAndStop(t *testing.T) {
	t.Parallel()

	client := &openaiClient{
		baseClient: baseClient{
			model:   "test-model",
			baseURL: "https://api.openai.com/v1",
		},
	}

	req := ports.CompletionRequest{
		Messages:      []ports.Message{{Role: "user", Content: "hi"}},
		StopSequences: []string{"END", "STOP"},
	}

	nonStream := client.buildOpenAIRequest(req, false)
	if got, ok := nonStream["stream"].(bool); !ok || got {
		t.Fatalf("expected non-stream request to set stream=false, got %v", nonStream["stream"])
	}
	if _, ok := nonStream["stop"]; ok {
		t.Fatalf("expected non-stream request to omit stop, got %v", nonStream["stop"])
	}

	streamReq := client.buildOpenAIRequest(req, true)
	if got, ok := streamReq["stream"].(bool); !ok || !got {
		t.Fatalf("expected stream request to set stream=true, got %v", streamReq["stream"])
	}
	stop, ok := streamReq["stop"].([]string)
	if !ok {
		t.Fatalf("expected stop to be []string, got %T", streamReq["stop"])
	}
	if len(stop) != 2 || stop[0] != "END" || stop[1] != "STOP" {
		t.Fatalf("unexpected stop sequences: %+v", stop)
	}

	req.StopSequences[0] = "MUTATED"
	if stop[0] != "END" {
		t.Fatalf("expected stop sequences to be copied, got %+v", stop)
	}
}

func TestOpenAIClientBuildOpenAIRequestArkMaxCompletionTokens(t *testing.T) {
	t.Parallel()

	client := &openaiClient{
		baseClient: baseClient{
			model:   "test-model",
			baseURL: "https://ark.cn-beijing.volces.com/api/v3",
		},
	}

	req := ports.CompletionRequest{
		Messages:    []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens:   128,
		Temperature: 0.7,
		Thinking: ports.ThinkingConfig{
			Enabled: true,
		},
	}

	payload := client.buildOpenAIRequest(req, false)
	if got, ok := payload["reasoning_effort"].(string); !ok || got != "medium" {
		t.Fatalf("expected ARK reasoning_effort=medium, got %v", payload["reasoning_effort"])
	}
	if _, ok := payload["max_tokens"]; ok {
		t.Fatalf("expected ARK request to omit max_tokens when MaxTokens>0, got %v", payload["max_tokens"])
	}
	if got, ok := payload["max_completion_tokens"].(int); !ok || got != 128 {
		t.Fatalf("expected max_completion_tokens=128, got %v", payload["max_completion_tokens"])
	}
}

func TestOpenAIClientBuildOpenAIRequestArkKeepsMaxTokensWhenNonPositive(t *testing.T) {
	t.Parallel()

	client := &openaiClient{
		baseClient: baseClient{
			model:   "test-model",
			baseURL: "https://ark.cn-beijing.volces.com/api/v3",
		},
	}

	req := ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 0,
		Thinking: ports.ThinkingConfig{
			Enabled: true,
		},
	}

	payload := client.buildOpenAIRequest(req, true)
	if _, ok := payload["max_completion_tokens"]; ok {
		t.Fatalf("expected no max_completion_tokens for non-positive MaxTokens, got %v", payload["max_completion_tokens"])
	}
	if got, ok := payload["max_tokens"].(int); !ok || got != 0 {
		t.Fatalf("expected max_tokens=0, got %v", payload["max_tokens"])
	}
}

func TestConvertMessagesKeepsToolAttachmentsAsText(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{
					ID:   "call-1",
					Name: "plan",
					Arguments: map[string]any{
						"goal": "x",
					},
				},
			},
		},
		{
			Role:       "tool",
			ToolCallID: "call-1",
			Content:    "Generated 1 image: [cat.png]",
			Attachments: map[string]ports.Attachment{
				"cat.png": {
					Name:      "cat.png",
					MediaType: "image/png",
					Data:      "ZmFrZUJhc2U2NA==",
					Source:    "seedream",
				},
			},
		},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(converted))
	}

	content, ok := converted[1]["content"].(string)
	if !ok {
		t.Fatalf("expected string content, got %T", converted[1]["content"])
	}
	if content != msgs[1].Content {
		t.Fatalf("expected content %q, got %q", msgs[1].Content, content)
	}
}

func TestConvertMessagesDropsToolMessagesWithoutCallID(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{Role: "user", Content: "hi"},
		{Role: "tool", Content: "stale-output-without-id"},
		{Role: "assistant", Content: "ack"},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 2 {
		t.Fatalf("expected 2 messages after dropping invalid tool message, got %d", len(converted))
	}
	for _, msg := range converted {
		if role, _ := msg["role"].(string); role == "tool" {
			t.Fatalf("expected tool message without tool_call_id to be dropped: %+v", msg)
		}
	}
}

func TestConvertMessagesDropsOrphanToolMessageBeforeToolCall(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{Role: "user", Content: "hi"},
		{Role: "tool", Content: "stale-output", ToolCallID: "call-1"},
		{
			Role: "assistant",
			ToolCalls: []ports.ToolCall{
				{
					ID:   "call-1",
					Name: "plan",
					Arguments: map[string]any{
						"goal": "x",
					},
				},
			},
		},
		{Role: "tool", Content: "fresh-output", ToolCallID: "call-1"},
	}

	converted := client.convertMessages(msgs)
	var toolOutputs []map[string]any
	for _, msg := range converted {
		if role, _ := msg["role"].(string); role == "tool" {
			toolOutputs = append(toolOutputs, msg)
		}
	}
	if len(toolOutputs) != 1 {
		t.Fatalf("expected exactly one tool output after pruning orphan, got %d", len(toolOutputs))
	}
	if got, _ := toolOutputs[0]["content"].(string); got != "fresh-output" {
		t.Fatalf("expected preserved tool output content to be fresh-output, got %q", got)
	}
	if got, _ := toolOutputs[0]["tool_call_id"].(string); got != "call-1" {
		t.Fatalf("expected preserved tool_call_id call-1, got %q", got)
	}
}

func TestConvertMessagesKimiDropsEmptyAssistantMessages(t *testing.T) {
	t.Parallel()

	// Kimi API rejects assistant messages with empty content and no tool_calls.
	// This happens after checkpoint recovery where MessageState drops ToolCalls.
	client := &openaiClient{baseClient: baseClient{baseURL: "https://api.kimi.com/coding/v1"}, kimiCompat: true}
	msgs := []ports.Message{
		{Role: "system", Content: "system prompt"},
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: "thinking..."},
		{Role: "user", Content: "next question"},
		{Role: "assistant", Content: ""},   // empty — should be dropped
		{Role: "assistant", Content: "  "}, // whitespace only — should be dropped
		{Role: "assistant", Content: "", ToolCalls: []ports.ToolCall{ // has tool_calls — should be kept
			{ID: "call-1", Name: "plan", Arguments: map[string]any{"goal": "x"}},
		}},
		{Role: "tool", Content: "done", ToolCallID: "call-1"},
		{Role: "assistant", Content: "final answer"},
	}

	converted := client.convertMessages(msgs)

	var assistantContents []string
	for _, msg := range converted {
		if role, _ := msg["role"].(string); role == "assistant" {
			content, _ := msg["content"].(string)
			assistantContents = append(assistantContents, content)
		}
	}

	// Expect: "thinking...", "" (with tool_calls), "final answer" — the two empty ones dropped
	if len(assistantContents) != 3 {
		t.Fatalf("expected 3 assistant messages (dropped 2 empty), got %d: %v", len(assistantContents), assistantContents)
	}
	if assistantContents[0] != "thinking..." {
		t.Errorf("expected first assistant content to be 'thinking...', got %q", assistantContents[0])
	}
	if assistantContents[2] != "final answer" {
		t.Errorf("expected last assistant content to be 'final answer', got %q", assistantContents[2])
	}
}

func TestConvertMessagesKimiAlwaysSetsReasoningContentOnToolCallMessages(t *testing.T) {
	t.Parallel()

	// Kimi API error: "thinking is enabled but reasoning_content is missing in
	// assistant tool call message at index N".
	// reasoning_content must always be present (even as "") in assistant messages
	// with tool_calls when using Kimi, regardless of whether thinking text exists.
	client := &openaiClient{baseClient: baseClient{baseURL: "https://api.kimi.com/coding/v1"}, kimiCompat: true}

	t.Run("sets empty reasoning_content when no thinking", func(t *testing.T) {
		msgs := []ports.Message{
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ports.ToolCall{
					{ID: "call-1", Name: "plan", Arguments: map[string]any{"goal": "x"}},
				},
				// No Thinking field set
			},
		}
		converted := client.convertMessages(msgs)
		if len(converted) != 1 {
			t.Fatalf("expected 1 message, got %d", len(converted))
		}
		rc, exists := converted[0]["reasoning_content"]
		if !exists {
			t.Fatal("reasoning_content field must be present in Kimi assistant tool_call messages, but it was missing")
		}
		if rc != "" {
			t.Fatalf("expected empty string for reasoning_content when no thinking, got %q", rc)
		}
	})

	t.Run("sets actual reasoning_content when thinking exists", func(t *testing.T) {
		msgs := []ports.Message{
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ports.ToolCall{
					{ID: "call-2", Name: "search", Arguments: map[string]any{"query": "test"}},
				},
				Thinking: ports.Thinking{
					Parts: []ports.ThinkingPart{
						{Kind: "thinking", Text: "I should search for this."},
					},
				},
			},
		}
		converted := client.convertMessages(msgs)
		if len(converted) != 1 {
			t.Fatalf("expected 1 message, got %d", len(converted))
		}
		rc, exists := converted[0]["reasoning_content"]
		if !exists {
			t.Fatal("reasoning_content field must be present in Kimi assistant tool_call messages, but it was missing")
		}
		if rc != "I should search for this." {
			t.Fatalf("expected reasoning_content to be thinking text, got %q", rc)
		}
	})

	t.Run("non-kimi client does not set reasoning_content", func(t *testing.T) {
		nonKimiClient := &openaiClient{baseClient: baseClient{baseURL: "https://api.openai.com/v1"}}
		msgs := []ports.Message{
			{
				Role:    "assistant",
				Content: "ok",
				ToolCalls: []ports.ToolCall{
					{ID: "call-3", Name: "search", Arguments: map[string]any{"query": "test"}},
				},
			},
		}
		converted := nonKimiClient.convertMessages(msgs)
		if len(converted) != 1 {
			t.Fatalf("expected 1 message, got %d", len(converted))
		}
		if _, exists := converted[0]["reasoning_content"]; exists {
			t.Fatal("reasoning_content should not be set for non-Kimi providers")
		}
	})

	t.Run("sets reasoning_content when model name contains kimi (proxy URL)", func(t *testing.T) {
		// Kimi routed through a proxy — baseURL does not contain kimi.com,
		// but model name "kimi-for-coding" should trigger the same contract.
		proxyKimiClient := &openaiClient{
			baseClient: baseClient{
				baseURL: "https://proxy.internal.company.com/v1",
				model:   "kimi-for-coding",
			},
			kimiCompat: true,
		}
		msgs := []ports.Message{
			{
				Role:    "assistant",
				Content: "ok",
				ToolCalls: []ports.ToolCall{
					{ID: "call-proxy", Name: "tool_x", Arguments: map[string]any{"key": "val"}},
				},
				Thinking: ports.Thinking{Parts: []ports.ThinkingPart{{Text: "my reasoning"}}},
			},
		}
		converted := proxyKimiClient.convertMessages(msgs)
		if len(converted) != 1 {
			t.Fatalf("expected 1 message, got %d", len(converted))
		}
		rc, exists := converted[0]["reasoning_content"]
		if !exists {
			t.Fatal("reasoning_content must be set when model name contains 'kimi'")
		}
		if rc != "my reasoning" {
			t.Fatalf("expected reasoning_content 'my reasoning', got %q", rc)
		}
	})
}

func TestConvertMessagesNonKimiKeepsEmptyAssistantMessages(t *testing.T) {
	t.Parallel()

	// Non-kimi providers should keep empty assistant messages (they may tolerate them).
	client := &openaiClient{baseClient: baseClient{baseURL: "https://api.openai.com/v1"}}
	msgs := []ports.Message{
		{Role: "user", Content: "hello"},
		{Role: "assistant", Content: ""},
		{Role: "user", Content: "again"},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 3 {
		t.Fatalf("expected 3 messages for non-kimi client, got %d", len(converted))
	}
}

func TestConvertMessagesEmbedsUserImageAttachmentsWithPlaceholders(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{
			Role:    "user",
			Content: "Describe [cat.png] and [dog.png].",
			Attachments: map[string]ports.Attachment{
				"cat.png": {
					Name:      "cat.png",
					MediaType: "image/png",
					URI:       "https://example.com/cat.png",
				},
				"dog.png": {
					Name:      "dog.png",
					MediaType: "image/png",
					Data:      "ZmFrZUJhc2U2NA==",
				},
			},
		},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 1 {
		t.Fatalf("expected 1 message, got %d", len(converted))
	}

	parts, ok := converted[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected multipart content, got %T", converted[0]["content"])
	}

	var imageURLs []string
	for _, part := range parts {
		if part["type"] != "image_url" {
			continue
		}
		imageURL, _ := part["image_url"].(map[string]any)
		url, _ := imageURL["url"].(string)
		imageURLs = append(imageURLs, url)
	}

	if got, want := len(imageURLs), 2; got != want {
		t.Fatalf("expected %d image urls, got %d (%v)", want, got, imageURLs)
	}
	if imageURLs[0] != "https://example.com/cat.png" {
		t.Fatalf("unexpected first image url: %q", imageURLs[0])
	}
	if wantPrefix := "data:image/png;base64,"; len(imageURLs[1]) < len(wantPrefix) || imageURLs[1][:len(wantPrefix)] != wantPrefix {
		t.Fatalf("expected data uri prefix %q, got %q", wantPrefix, imageURLs[1])
	}
}

func TestConvertMessagesAppendsUnreferencedUserImages(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{
			Role:    "user",
			Content: "Describe the attachment.",
			Attachments: map[string]ports.Attachment{
				"cat.png": {
					Name:      "cat.png",
					MediaType: "image/png",
					URI:       "https://example.com/cat.png",
				},
			},
		},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 1 {
		t.Fatalf("expected 1 message, got %d", len(converted))
	}

	parts, ok := converted[0]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected multipart content, got %T", converted[0]["content"])
	}

	if len(parts) < 3 {
		t.Fatalf("expected at least 3 parts, got %d", len(parts))
	}

	lastText, _ := parts[len(parts)-2]["text"].(string)
	if lastText != "[cat.png]" {
		t.Fatalf("expected placeholder tag before image, got %q", lastText)
	}

	imageURL, _ := parts[len(parts)-1]["image_url"].(map[string]any)
	url, _ := imageURL["url"].(string)
	if url != "https://example.com/cat.png" {
		t.Fatalf("unexpected image url: %q", url)
	}
}

func TestConvertMessagesKeepsNonImageAttachmentsAsText(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{
			Role:    "user",
			Content: "Analyze [doc.pdf].",
			Attachments: map[string]ports.Attachment{
				"doc.pdf": {
					Name:      "doc.pdf",
					MediaType: "application/pdf",
					Data:      "ZmFrZUJhc2U2NA==",
				},
			},
		},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 1 {
		t.Fatalf("expected 1 message, got %d", len(converted))
	}

	content, ok := converted[0]["content"].(string)
	if !ok {
		t.Fatalf("expected string content, got %T", converted[0]["content"])
	}
	if content != msgs[0].Content {
		t.Fatalf("expected content %q, got %q", msgs[0].Content, content)
	}
}

func TestConvertMessagesEmbedsOnlyLatestUserAttachmentMessage(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{
			Role:    "user",
			Source:  ports.MessageSourceUserInput,
			Content: "First turn [old.png]",
			Attachments: map[string]ports.Attachment{
				"old.png": {
					Name:      "old.png",
					MediaType: "image/png",
					URI:       "https://example.com/old.png",
				},
			},
		},
		{Role: "assistant", Content: "noted"},
		{
			Role:    "user",
			Source:  ports.MessageSourceUserInput,
			Content: "Latest turn [new.png]",
			Attachments: map[string]ports.Attachment{
				"new.png": {
					Name:      "new.png",
					MediaType: "image/png",
					URI:       "https://example.com/new.png",
				},
			},
		},
	}

	converted := client.convertMessages(msgs)
	if len(converted) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(converted))
	}

	firstContent, ok := converted[0]["content"].(string)
	if !ok {
		t.Fatalf("expected first user message content to stay text-only, got %T", converted[0]["content"])
	}
	if firstContent != msgs[0].Content {
		t.Fatalf("unexpected first content: %q", firstContent)
	}

	latestParts, ok := converted[2]["content"].([]map[string]any)
	if !ok {
		t.Fatalf("expected latest user message multipart content, got %T", converted[2]["content"])
	}
	foundImage := false
	for _, part := range latestParts {
		if part["type"] != "image_url" {
			continue
		}
		foundImage = true
	}
	if !foundImage {
		t.Fatalf("expected latest user message to include image_url block")
	}
}

func TestShouldEmbedAttachmentsSkipsToolResultSources(t *testing.T) {
	msg := ports.Message{
		Role:   "system",
		Source: ports.MessageSourceToolResult,
		Attachments: map[string]ports.Attachment{
			"doc.txt": {Name: "doc.txt"},
		},
	}

	if shouldEmbedAttachmentsInContent(msg) {
		t.Fatalf("expected tool-result message to skip attachment embedding")
	}
}

func TestConvertToolsSkipsInvalidFunctionNames(t *testing.T) {
	t.Parallel()

	client := &openaiClient{
		baseClient: baseClient{
			logger: utils.NewCategorizedLogger(utils.LogCategoryLLM, "test"),
		},
	}

	tools := []ports.ToolDefinition{
		{
			Name:        "valid_tool",
			Description: "valid",
			Parameters:  ports.ParameterSchema{Type: "object"},
		},
		{
			Name:        "invalid.tool.name",
			Description: "invalid",
			Parameters:  ports.ParameterSchema{Type: "object"},
		},
		{
			Name:        "also-valid-1",
			Description: "valid",
			Parameters:  ports.ParameterSchema{Type: "object"},
		},
	}

	converted := client.convertTools(tools)

	if got, want := len(converted), 2; got != want {
		t.Fatalf("expected %d tools after filtering, got %d", want, got)
	}

	names := []string{
		converted[0]["function"].(map[string]any)["name"].(string),
		converted[1]["function"].(map[string]any)["name"].(string),
	}

	if names[0] != "valid_tool" || names[1] != "also-valid-1" {
		t.Fatalf("unexpected tool names: %v", names)
	}
}

func newIPv4TestServer(t *testing.T, handler http.Handler) *httptest.Server {
	t.Helper()

	ln, err := net.Listen("tcp4", "127.0.0.1:0")
	if err != nil {
		t.Skipf("skipping test: unable to create loopback listener: %v", err)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = ln
	server.Start()
	t.Cleanup(server.Close)

	return server
}
