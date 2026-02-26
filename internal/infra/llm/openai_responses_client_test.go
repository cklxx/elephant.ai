package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestOpenAIResponsesClientCompleteSuccess(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Fatalf("expected POST, got %s", r.Method)
		}
		if got := r.URL.Path; got != "/responses" {
			t.Fatalf("unexpected path: %s", got)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer test-key" {
			t.Fatalf("expected Authorization header, got %q", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload["model"] != "test-model" {
			t.Fatalf("unexpected model: %v", payload["model"])
		}
		if payload["max_output_tokens"] != float64(64) {
			t.Fatalf("expected max_output_tokens 64, got %#v", payload["max_output_tokens"])
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "message",
					"role": "assistant",
					"content": []any{
						map[string]any{"type": "output_text", "text": "hello"},
					},
				},
				map[string]any{
					"type":      "tool_call",
					"id":        "call-1",
					"name":      "toolName",
					"arguments": `{"foo":"bar"}`,
				},
			},
			"usage": map[string]any{
				"input_tokens":  2,
				"output_tokens": 3,
				"total_tokens":  5,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "hello" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 5 {
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

func TestOpenAIResponsesClientOmitsInstructionsField(t *testing.T) {
	t.Parallel()

	var hasInstructions bool

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		if _, ok := payload["instructions"]; ok {
			hasInstructions = true
		}

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if hasInstructions {
		t.Fatalf("expected instructions field to be omitted")
	}
}

func TestOpenAIResponsesClientKeepsSystemMessageInInput(t *testing.T) {
	t.Parallel()

	var gotInput []any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		input, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		gotInput = input

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: "hi"},
		},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(gotInput) != 2 {
		t.Fatalf("expected 2 input entries, got %d", len(gotInput))
	}
	first, ok := gotInput[0].(map[string]any)
	if !ok {
		t.Fatalf("expected input entry map, got %#v", gotInput[0])
	}
	if first["role"] != "system" {
		t.Fatalf("expected system role, got %#v", first["role"])
	}
}

func TestOpenAIResponsesClientSetsStoreFalse(t *testing.T) {
	t.Parallel()

	var gotStore any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		gotStore = payload["store"]

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 16,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if gotStore != false {
		t.Fatalf("expected store false, got %#v", gotStore)
	}
}

func TestOpenAIResponsesClientOmitsMaxOutputTokensForCodex(t *testing.T) {
	t.Parallel()

	var hasMaxOutputTokens bool

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, hasMaxOutputTokens = payload["max_output_tokens"]

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if hasMaxOutputTokens {
		t.Fatalf("expected max_output_tokens to be omitted for codex")
	}
}

func TestOpenAIResponsesClientOmitsTemperatureForCodex(t *testing.T) {
	t.Parallel()

	var hasTemperature bool

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		_, hasTemperature = payload["temperature"]

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 16,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if hasTemperature {
		t.Fatalf("expected temperature to be omitted for codex")
	}
}

func TestOpenAIResponsesClientIncludesReasoningForCodexWhenThinkingEnabled(t *testing.T) {
	t.Parallel()

	var gotReasoning map[string]any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if reasoning, ok := payload["reasoning"].(map[string]any); ok {
			gotReasoning = reasoning
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}
		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("gpt-5.3-codex-spark", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Thinking: ports.ThinkingConfig{Enabled: true, Effort: "high"},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if gotReasoning == nil {
		t.Fatalf("expected reasoning payload for codex when thinking is enabled")
	}
	if gotReasoning["effort"] != "high" {
		t.Fatalf("expected reasoning effort=high, got %#v", gotReasoning["effort"])
	}
	if gotReasoning["summary"] != "auto" {
		t.Fatalf("expected reasoning summary=auto, got %#v", gotReasoning["summary"])
	}
}

func TestOpenAIResponsesClientUsesFlatToolsForCodex(t *testing.T) {
	t.Parallel()

	var gotTool map[string]any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Fatalf("expected tools in payload")
		}

		tool, ok := tools[0].(map[string]any)
		if !ok {
			t.Fatalf("expected tool to be object")
		}
		gotTool = tool

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{
			{
				Name:        "ping",
				Description: "return pong",
				Parameters: ports.ParameterSchema{
					Type:       "object",
					Properties: map[string]ports.Property{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if _, ok := gotTool["name"]; !ok {
		t.Fatalf("expected codex tool name at top-level")
	}
	if gotTool["type"] != "function" {
		t.Fatalf("expected codex tool type function, got %v", gotTool["type"])
	}
	if _, ok := gotTool["function"]; ok {
		t.Fatalf("expected codex tool to omit function wrapper")
	}
}

func TestOpenAIResponsesClientEmbedsOnlyLatestUserAttachmentMessage(t *testing.T) {
	t.Parallel()

	var gotInput []any
	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		input, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		gotInput = input

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{
				map[string]any{
					"type": "message",
					"role": "assistant",
					"content": []any{
						map[string]any{"type": "output_text", "text": "ok"},
					},
				},
			},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "first [old.png]",
				Attachments: map[string]ports.Attachment{
					"old.png": {Name: "old.png", MediaType: "image/png", URI: "https://example.com/old.png"},
				},
			},
			{
				Role:    "assistant",
				Content: "ack",
			},
			{
				Role:    "user",
				Source:  ports.MessageSourceUserInput,
				Content: "latest [new.png]",
				Attachments: map[string]ports.Attachment{
					"new.png": {Name: "new.png", MediaType: "image/png", URI: "https://example.com/new.png"},
				},
			},
		},
		MaxTokens: 32,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if len(gotInput) != 3 {
		t.Fatalf("expected 3 input items, got %d", len(gotInput))
	}

	first, ok := gotInput[0].(map[string]any)
	if !ok {
		t.Fatalf("expected first input map, got %#v", gotInput[0])
	}
	firstContent, ok := first["content"].([]any)
	if !ok {
		t.Fatalf("expected first content array, got %#v", first["content"])
	}
	for _, part := range firstContent {
		block, _ := part.(map[string]any)
		if block["type"] == "input_image" {
			t.Fatalf("expected first user message to stay text-only")
		}
	}

	latest, ok := gotInput[2].(map[string]any)
	if !ok {
		t.Fatalf("expected latest input map, got %#v", gotInput[2])
	}
	latestContent, ok := latest["content"].([]any)
	if !ok {
		t.Fatalf("expected latest content array, got %#v", latest["content"])
	}
	foundImage := false
	for _, part := range latestContent {
		block, _ := part.(map[string]any)
		if block["type"] == "input_image" {
			foundImage = true
			break
		}
	}
	if !foundImage {
		t.Fatalf("expected latest user message to include input_image block")
	}
}

func TestOpenAIResponsesClientUsesFlatToolsForResponses(t *testing.T) {
	t.Parallel()

	var gotTool map[string]any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		tools, ok := payload["tools"].([]any)
		if !ok || len(tools) == 0 {
			t.Fatalf("expected tools in payload")
		}

		tool, ok := tools[0].(map[string]any)
		if !ok {
			t.Fatalf("expected tool to be object")
		}
		gotTool = tool

		w.Header().Set("Content-Type", "application/json")
		resp := map[string]any{
			"id":     "resp-1",
			"status": "completed",
			"output": []any{},
			"usage": map[string]any{
				"input_tokens":  1,
				"output_tokens": 1,
				"total_tokens":  2,
			},
		}
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("write response: %v", err)
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL,
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{
			{
				Name:        "ping",
				Description: "return pong",
				Parameters: ports.ParameterSchema{
					Type:       "object",
					Properties: map[string]ports.Property{},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if _, ok := gotTool["name"]; !ok {
		t.Fatalf("expected responses tool name at top-level")
	}
	if gotTool["type"] != "function" {
		t.Fatalf("expected responses tool type function, got %v", gotTool["type"])
	}
	if _, ok := gotTool["function"]; ok {
		t.Fatalf("expected responses tool to omit function wrapper")
	}
}

func TestOpenAIResponsesClientSetsInstructionsForCodex(t *testing.T) {
	t.Parallel()

	var gotInstructions string
	var sawSystemRole bool

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		instructions, ok := payload["instructions"].(string)
		if !ok {
			t.Fatalf("expected instructions string, got %#v", payload["instructions"])
		}
		gotInstructions = instructions

		input, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		for _, item := range input {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if entry["role"] == "system" || entry["role"] == "developer" {
				sawSystemRole = true
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: "hi"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if strings.TrimSpace(gotInstructions) != "system instructions" {
		t.Fatalf("unexpected instructions: %q", gotInstructions)
	}
	if sawSystemRole {
		t.Fatalf("expected system/developer roles to be omitted from codex input")
	}
}

func TestOpenAIResponsesClientSynthesizesInputWhenCodexInputIsEmpty(t *testing.T) {
	t.Parallel()

	var gotInput []any
	var gotInstructions string

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/backend-api/codex/responses" {
			t.Fatalf("unexpected path: %s", got)
		}

		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}

		instructions, ok := payload["instructions"].(string)
		if !ok {
			t.Fatalf("expected instructions string, got %#v", payload["instructions"])
		}
		gotInstructions = instructions

		input, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		gotInput = input

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "system", Content: "system instructions"},
			{Role: "user", Content: ""},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if strings.TrimSpace(gotInstructions) != "system instructions" {
		t.Fatalf("unexpected instructions: %q", gotInstructions)
	}
	if len(gotInput) != 1 {
		t.Fatalf("expected 1 synthesized input item, got %d", len(gotInput))
	}

	inputMsg, ok := gotInput[0].(map[string]any)
	if !ok {
		t.Fatalf("expected synthesized input item map, got %#v", gotInput[0])
	}
	if inputMsg["role"] != "user" {
		t.Fatalf("expected synthesized input role=user, got %#v", inputMsg["role"])
	}
	content, ok := inputMsg["content"].([]any)
	if !ok || len(content) == 0 {
		t.Fatalf("expected synthesized input content, got %#v", inputMsg["content"])
	}
	part, ok := content[0].(map[string]any)
	if !ok {
		t.Fatalf("expected synthesized content part map, got %#v", content[0])
	}
	if part["type"] != "input_text" {
		t.Fatalf("expected synthesized content type input_text, got %#v", part["type"])
	}
	text, _ := part["text"].(string)
	if strings.TrimSpace(text) == "" {
		t.Fatal("expected synthesized fallback input text to be non-empty")
	}
}

func TestOpenAIResponsesClientErrorsWhenInputEmptyAndNoFallback(t *testing.T) {
	t.Parallel()

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: "http://127.0.0.1:65535/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: ""},
		},
	})
	if err == nil {
		t.Fatal("expected error for empty codex input without fallback context")
	}
	if !strings.Contains(err.Error(), "responses input is empty") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestOpenAIResponsesClientOmitsToolCallsForCodex(t *testing.T) {
	t.Parallel()

	var sawToolCalls bool

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		input, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		for _, item := range input {
			entry, ok := item.(map[string]any)
			if !ok {
				continue
			}
			if _, ok := entry["tool_calls"]; ok {
				sawToolCalls = true
			}
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: "hi"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call-1",
						Name: "plan",
						Arguments: map[string]any{
							"foo": "bar",
						},
					},
				},
			},
			{Role: "tool", Content: "ok", ToolCallID: "call-1"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if sawToolCalls {
		t.Fatalf("expected tool_calls to be omitted for codex input messages")
	}
}

func TestOpenAIResponsesClientConvertsToolMessagesToFunctionCallOutput(t *testing.T) {
	t.Parallel()

	var input []any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		raw, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		input = raw

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: "hi"},
			{
				Role:    "assistant",
				Content: "",
				ToolCalls: []ports.ToolCall{
					{
						ID:   "call-1",
						Name: "plan",
						Arguments: map[string]any{
							"foo": "bar",
						},
					},
				},
			},
			{Role: "tool", Content: "ok", ToolCallID: "call-1"},
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var sawFunctionCall bool
	var sawFunctionOutput bool
	var sawToolRole bool
	for _, item := range input {
		entry, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if entry["role"] == "tool" {
			sawToolRole = true
		}
		if entry["type"] == "function_call" {
			sawFunctionCall = true
		}
		if entry["type"] == "function_call_output" {
			sawFunctionOutput = true
		}
	}

	if sawToolRole {
		t.Fatalf("expected tool role to be omitted for responses input")
	}
	if !sawFunctionCall {
		t.Fatalf("expected function_call input item")
	}
	if !sawFunctionOutput {
		t.Fatalf("expected function_call_output input item")
	}
}

func TestOpenAIResponsesClientDropsFunctionCallOutputWithoutFunctionCall(t *testing.T) {
	t.Parallel()

	var input []any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		raw, ok := payload["input"].([]any)
		if !ok {
			t.Fatalf("expected input list, got %#v", payload["input"])
		}
		input = raw

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"ok"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":1,"total_tokens":2}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{
			{Role: "user", Content: "hi"},
			// Invalid history shape: tool result appears before its function_call.
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
		},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	var outputCount int
	var sawFreshOutput bool
	for _, item := range input {
		entry, ok := item.(map[string]any)
		if !ok || entry["type"] != "function_call_output" {
			continue
		}
		if entry["call_id"] != "call-1" {
			continue
		}
		outputCount++
		if output, _ := entry["output"].(string); output == "fresh-output" {
			sawFreshOutput = true
		}
	}

	if outputCount != 1 {
		t.Fatalf("expected exactly 1 function_call_output for call-1 after pruning, got %d", outputCount)
	}
	if !sawFreshOutput {
		t.Fatalf("expected preserved function_call_output content to be fresh-output")
	}
}

func TestOpenAIResponsesClientCompleteStreamsForCodex(t *testing.T) {
	t.Parallel()

	var gotStream bool

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var payload map[string]any
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if stream, ok := payload["stream"].(bool); ok {
			gotStream = stream
		}

		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.created","response":{"id":"resp-1","created_at":1,"model":"gpt-5.2-codex"}}`,
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"hello "}`,
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"world"}`,
			`{"type":"response.output_item.done","output_index":0,"item":{"type":"function_call","id":"call-1","call_id":"call-1","name":"toolName","arguments":"{\"foo\":\"bar\"}","status":"completed"}}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if !gotStream {
		t.Fatalf("expected stream true for codex")
	}
	if resp.Content != "hello world" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if resp.Usage.TotalTokens != 3 {
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

func TestOpenAIResponsesClientCompleteCollectsReasoningSummaryDeltas(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.created","response":{"id":"resp-1","created_at":1,"model":"gpt-5.2-codex"}}`,
			`{"type":"response.reasoning_summary.delta","delta":"step-1 "}`,
			`{"type":"response.reasoning_summary_text.delta","delta":"step-2"}`,
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"done"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 {
		t.Fatalf("expected one thinking part, got %d", len(resp.Thinking.Parts))
	}
	if resp.Thinking.Parts[0].Text != "step-1 step-2" {
		t.Fatalf("unexpected thinking text: %q", resp.Thinking.Parts[0].Text)
	}
}

func TestOpenAIResponsesClientCompleteDeduplicatesRepeatedReasoningBlocks(t *testing.T) {
	t.Parallel()

	const repeated = "**Responding with concise Chinese options**"

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.created","response":{"id":"resp-1","created_at":1,"model":"gpt-5.2-codex"}}`,
			`{"type":"response.reasoning.delta","delta":"` + repeated + `"}`,
			`{"type":"response.reasoning_summary.delta","delta":"` + repeated + `"}`,
			`{"type":"response.reasoning_summary_text.delta","delta":"` + repeated + `"}`,
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"done"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages:  []ports.Message{{Role: "user", Content: "hi"}},
		MaxTokens: 64,
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 {
		t.Fatalf("expected one thinking part, got %d", len(resp.Thinking.Parts))
	}
	if resp.Thinking.Parts[0].Text != repeated {
		t.Fatalf("expected deduplicated thinking text %q, got %q", repeated, resp.Thinking.Parts[0].Text)
	}
}

func TestOpenAIResponsesClientCompleteParsesThinkingFromCompletedOutput(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.created","response":{"id":"resp-1","created_at":1,"model":"gpt-5.2-codex"}}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3},"output":[{"type":"reasoning","content":[{"type":"text","text":"thinking from completed"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}]}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 {
		t.Fatalf("expected one thinking part, got %d", len(resp.Thinking.Parts))
	}
	if resp.Thinking.Parts[0].Text != "thinking from completed" {
		t.Fatalf("unexpected thinking text: %q", resp.Thinking.Parts[0].Text)
	}
}

func TestOpenAIResponsesClientCompleteParsesThinkingFromOutputItemDoneSummary(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.created","response":{"id":"resp-1","created_at":1,"model":"gpt-5.2-codex"}}`,
			`{"type":"response.output_item.done","item":{"type":"reasoning","summary":[{"type":"summary_text","text":"thinking from summary"}]}}`,
			`{"type":"response.output_text.delta","item_id":"item-1","delta":"done"}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3}}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 {
		t.Fatalf("expected one thinking part, got %d", len(resp.Thinking.Parts))
	}
	if resp.Thinking.Parts[0].Text != "thinking from summary" {
		t.Fatalf("unexpected thinking text: %q", resp.Thinking.Parts[0].Text)
	}
}

func TestOpenAIResponsesClientCompleteParsesThinkingFromCompletedSummary(t *testing.T) {
	t.Parallel()

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected http.Flusher")
		}

		events := []string{
			`{"type":"response.created","response":{"id":"resp-1","created_at":1,"model":"gpt-5.2-codex"}}`,
			`{"type":"response.completed","response":{"usage":{"input_tokens":1,"output_tokens":2,"total_tokens":3},"output":[{"type":"reasoning","summary":[{"type":"summary_text","text":"completed summary"}]},{"type":"message","role":"assistant","content":[{"type":"output_text","text":"done"}]}]}}`,
			`[DONE]`,
		}
		for _, evt := range events {
			if _, err := fmt.Fprintf(w, "data: %s\n\n", evt); err != nil {
				t.Fatalf("write event: %v", err)
			}
			flusher.Flush()
		}
	}))

	client, err := NewOpenAIResponsesClient("test-model", Config{
		APIKey:  "test-key",
		BaseURL: server.URL + "/backend-api/codex",
	})
	if err != nil {
		t.Fatalf("NewOpenAIResponsesClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "done" {
		t.Fatalf("unexpected content: %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 {
		t.Fatalf("expected one thinking part, got %d", len(resp.Thinking.Parts))
	}
	if resp.Thinking.Parts[0].Text != "completed summary" {
		t.Fatalf("unexpected thinking text: %q", resp.Thinking.Parts[0].Text)
	}
}
