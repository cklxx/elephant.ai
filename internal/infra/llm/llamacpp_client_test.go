package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestNewLlamaCppClientDefaultBaseURL(t *testing.T) {
	t.Parallel()

	client, err := NewLlamaCppClient("test-model", Config{})
	if err != nil {
		t.Fatalf("NewLlamaCppClient error: %v", err)
	}
	llama, ok := client.(*llamaCppClient)
	if !ok {
		t.Fatalf("unexpected client type: %T", client)
	}
	if llama.inner == nil {
		t.Fatalf("expected inner client")
	}
	if llama.inner.baseURL != defaultLlamaCppBaseURL {
		t.Fatalf("unexpected base url: got %q want %q", llama.inner.baseURL, defaultLlamaCppBaseURL)
	}
}

func TestLlamaCppClientUsageCallbackProvider(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path; got != "/chat/completions" {
			t.Fatalf("unexpected path: %s", got)
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"choices": []any{
				map[string]any{
					"message": map[string]any{
						"content": "ok",
					},
					"finish_reason": "stop",
				},
			},
			"usage": map[string]any{
				"prompt_tokens":     1,
				"completion_tokens": 2,
				"total_tokens":      3,
			},
		})
	}))
	t.Cleanup(server.Close)

	client, err := NewLlamaCppClient("test-model", Config{BaseURL: server.URL})
	if err != nil {
		t.Fatalf("NewLlamaCppClient error: %v", err)
	}

	var gotProvider string
	if tracking, ok := client.(interface {
		SetUsageCallback(func(ports.TokenUsage, string, string))
	}); ok {
		tracking.SetUsageCallback(func(_ ports.TokenUsage, _ string, provider string) {
			gotProvider = provider
		})
	} else {
		t.Fatalf("client does not implement usage tracking")
	}

	_, err = client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete error: %v", err)
	}
	if gotProvider != "llama.cpp" {
		t.Fatalf("unexpected provider: %q", gotProvider)
	}
}
