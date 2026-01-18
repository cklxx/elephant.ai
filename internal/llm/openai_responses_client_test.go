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

func TestOpenAIResponsesClientIncludesInstructionsFromSystem(t *testing.T) {
	t.Parallel()

	var gotInstructions string
	var gotInput []any

	server := newIPv4TestServer(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

	if gotInstructions != "system instructions" {
		t.Fatalf("unexpected instructions: %q", gotInstructions)
	}
	if len(gotInput) != 1 {
		t.Fatalf("expected 1 input entry, got %d", len(gotInput))
	}
	entry, ok := gotInput[0].(map[string]any)
	if !ok {
		t.Fatalf("expected input entry map, got %#v", gotInput[0])
	}
	if entry["role"] != "user" {
		t.Fatalf("unexpected input role: %#v", entry["role"])
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
