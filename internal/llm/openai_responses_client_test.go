package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"alex/internal/agent/ports"
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
