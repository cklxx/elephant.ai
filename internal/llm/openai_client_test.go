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

	"alex/internal/agent/ports"
	alexerrors "alex/internal/errors"
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

func TestConvertMessagesKeepsToolAttachmentsAsText(t *testing.T) {
	t.Parallel()

	client := &openaiClient{}
	msgs := []ports.Message{
		{
			Role:    "tool",
			Content: "Generated 1 image: [cat.png]",
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
