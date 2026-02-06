package llm

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"alex/internal/domain/agent/ports"
)

func TestAntigravityClientBuildsGeminiPayload(t *testing.T) {
	var gotPath, gotAuth string
	var gotBody map[string]any
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPath = r.URL.Path
		gotAuth = r.Header.Get("Authorization")
		_ = json.NewDecoder(r.Body).Decode(&gotBody)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"ok"}]},"finishReason":"STOP"}],"usageMetadata":{"promptTokenCount":1,"candidatesTokenCount":2,"totalTokenCount":3}}`))
	}))
	defer srv.Close()

	client, err := NewAntigravityClient("gemini-3-pro-high", Config{APIKey: "tok", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("client error: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
		Tools: []ports.ToolDefinition{{
			Name:        "tool",
			Description: "desc",
			Parameters:  ports.ParameterSchema{Type: "object", Properties: map[string]ports.Property{}},
		}},
		Metadata: map[string]any{"request_id": "req-1"},
	})
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}
	if gotPath != "/v1internal:generateContent" {
		t.Fatalf("expected generateContent path, got %q", gotPath)
	}
	if gotAuth != "Bearer tok" {
		t.Fatalf("expected bearer auth, got %q", gotAuth)
	}
	if resp.Content != "ok" {
		t.Fatalf("expected response content, got %q", resp.Content)
	}

	request, _ := gotBody["request"].(map[string]any)
	if request == nil {
		t.Fatalf("expected request payload")
	}
	if request["toolConfig"] == nil {
		t.Fatalf("expected toolConfig")
	}
	if request["tools"] == nil {
		t.Fatalf("expected tools")
	}
}

func TestAntigravityClientConvertsToolCalls(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"functionCall":{"name":"tool","args":{"x":1}}}]},"finishReason":"STOP"}]}`))
	}))
	defer srv.Close()

	client, err := NewAntigravityClient("gemini-3-pro-high", Config{APIKey: "tok", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("client error: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}
	if len(resp.ToolCalls) != 1 || resp.ToolCalls[0].Name != "tool" {
		t.Fatalf("unexpected tool calls: %#v", resp.ToolCalls)
	}
}

func TestAntigravityClientCapturesThinking(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"candidates":[{"content":{"parts":[{"text":"thoughts","thought":true},{"text":"answer"}]},"finishReason":"STOP"}]}`))
	}))
	defer srv.Close()

	client, err := NewAntigravityClient("gemini-3-pro-high", Config{APIKey: "tok", BaseURL: srv.URL})
	if err != nil {
		t.Fatalf("client error: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("complete error: %v", err)
	}
	if resp.Content != "answer" {
		t.Fatalf("expected response content, got %q", resp.Content)
	}
	if len(resp.Thinking.Parts) != 1 || resp.Thinking.Parts[0].Text != "thoughts" {
		t.Fatalf("expected thinking capture, got %+v", resp.Thinking)
	}
}
