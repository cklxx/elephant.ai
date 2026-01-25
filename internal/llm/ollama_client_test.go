package llm

import (
	"bufio"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"alex/internal/agent/ports"
	portsllm "alex/internal/agent/ports/llm"
)

func TestOllamaClientComplete(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Stream {
			t.Fatalf("expected stream=false for Complete")
		}
		_ = json.NewEncoder(w).Encode(ollamaResponse{
			Model: "llama3",
			Message: ollamaMessage{
				Role:    "assistant",
				Content: "hello world",
			},
			Done:            true,
			DoneReason:      "stop",
			PromptEvalCount: 5,
			EvalCount:       7,
		})
	}))
	defer server.Close()

	client, err := NewOllamaClient("llama3", Config{BaseURL: server.URL + "/api"})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}

	resp, err := client.Complete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "hi"}},
	})
	if err != nil {
		t.Fatalf("Complete: %v", err)
	}

	if resp.Content != "hello world" {
		t.Fatalf("unexpected content: %s", resp.Content)
	}
	if resp.Usage.TotalTokens != 12 {
		t.Fatalf("unexpected total tokens: %d", resp.Usage.TotalTokens)
	}
}

func TestOllamaClientStreamComplete(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/chat" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Fatalf("expected flusher")
		}
		w.Header().Set("Content-Type", "application/x-ndjson")

		chunks := []ollamaResponse{
			{
				Model: "llama3",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "Hel",
				},
				Done: false,
			},
			{
				Model: "llama3",
				Message: ollamaMessage{
					Role:    "assistant",
					Content: "lo",
				},
				Done: false,
			},
			{
				Model:           "llama3",
				Done:            true,
				DoneReason:      "stop",
				PromptEvalCount: 2,
				EvalCount:       3,
			},
		}

		writer := bufio.NewWriter(w)
		for _, chunk := range chunks {
			data, err := json.Marshal(chunk)
			if err != nil {
				t.Fatalf("marshal chunk: %v", err)
			}
			if _, err := writer.Write(data); err != nil {
				t.Fatalf("write chunk: %v", err)
			}
			if err := writer.WriteByte('\n'); err != nil {
				t.Fatalf("write newline: %v", err)
			}
			if err := writer.Flush(); err != nil {
				t.Fatalf("flush writer: %v", err)
			}
			flusher.Flush()
		}
	}))
	defer server.Close()

	client, err := NewOllamaClient("llama3", Config{BaseURL: server.URL + "/api"})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}

	streaming, ok := client.(portsllm.StreamingLLMClient)
	if !ok {
		t.Fatalf("ollama client should implement StreamingLLMClient")
	}

	var deltas []ports.ContentDelta
	resp, err := streaming.StreamComplete(context.Background(), ports.CompletionRequest{
		Messages: []ports.Message{{Role: "user", Content: "Hello?"}},
	}, ports.CompletionStreamCallbacks{
		OnContentDelta: func(delta ports.ContentDelta) {
			deltas = append(deltas, delta)
		},
	})
	if err != nil {
		t.Fatalf("StreamComplete: %v", err)
	}

	if resp.Content != "Hello" {
		t.Fatalf("unexpected content: %s", resp.Content)
	}
	if resp.StopReason != "stop" {
		t.Fatalf("unexpected stop reason: %s", resp.StopReason)
	}
	if resp.Usage.TotalTokens != 5 {
		t.Fatalf("unexpected usage: %d", resp.Usage.TotalTokens)
	}

	if len(deltas) != 3 {
		t.Fatalf("expected 3 deltas, got %d", len(deltas))
	}
	if !deltas[0].Final && deltas[0].Delta != "Hel" {
		t.Fatalf("unexpected first delta: %+v", deltas[0])
	}
	if !deltas[2].Final {
		t.Fatalf("expected final delta flag")
	}
}

func TestOllamaClientRequestOptions(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if req.Options["temperature"] != 0.2 {
			t.Fatalf("expected temperature option")
		}
		if req.Options["top_p"] != 0.9 {
			t.Fatalf("expected top_p option")
		}
		if req.Options["num_predict"] != float64(256) {
			t.Fatalf("expected num_predict option")
		}
		if stops, ok := req.Options["stop"].([]any); ok {
			if len(stops) != 1 || stops[0] != "DONE" {
				t.Fatalf("unexpected stop option: %#v", req.Options["stop"])
			}
		} else {
			t.Fatalf("expected stop option slice")
		}
		close(done)
		_ = json.NewEncoder(w).Encode(ollamaResponse{Message: ollamaMessage{Content: "ok"}, Done: true})
	}))
	defer server.Close()

	client, err := NewOllamaClient("llama3", Config{BaseURL: server.URL + "/api"})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}

	go func() {
		_, _ = client.Complete(context.Background(), ports.CompletionRequest{
			Messages:      []ports.Message{{Role: "user", Content: "Hi"}},
			Temperature:   0.2,
			TopP:          0.9,
			MaxTokens:     256,
			StopSequences: []string{"DONE"},
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("options were not observed in time")
	}
}

func TestOllamaClientIncludesImagesForUserMessage(t *testing.T) {
	t.Parallel()

	done := make(chan struct{})
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var req ollamaRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(req.Messages) != 1 {
			t.Fatalf("expected 1 message, got %d", len(req.Messages))
		}
		msg := req.Messages[0]
		if msg.Role != "user" {
			t.Fatalf("expected role user, got %q", msg.Role)
		}
		if msg.Content != "Look at [cat.png]." {
			t.Fatalf("unexpected content %q", msg.Content)
		}
		if len(msg.Images) != 1 || msg.Images[0] != "ZmFrZUJhc2U2NA==" {
			t.Fatalf("unexpected images: %#v", msg.Images)
		}
		close(done)
		_ = json.NewEncoder(w).Encode(ollamaResponse{Message: ollamaMessage{Content: "ok"}, Done: true})
	}))
	defer server.Close()

	client, err := NewOllamaClient("llama3", Config{BaseURL: server.URL + "/api"})
	if err != nil {
		t.Fatalf("NewOllamaClient: %v", err)
	}

	go func() {
		_, _ = client.Complete(context.Background(), ports.CompletionRequest{
			Messages: []ports.Message{{
				Role:    "user",
				Content: "Look at [cat.png].",
				Attachments: map[string]ports.Attachment{
					"cat.png": {
						Name:      "cat.png",
						MediaType: "image/png",
						Data:      "ZmFrZUJhc2U2NA==",
					},
				},
			}},
		})
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatalf("expected request to include images")
	}
}
