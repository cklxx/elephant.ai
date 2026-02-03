package memory

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
)

func TestOllamaEmbedderBatch(t *testing.T) {
	var embedCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/api/embed" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		atomic.AddInt32(&embedCalls, 1)
		var req struct {
			Model string   `json:"model"`
			Input []string `json:"input"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if req.Model != "test-model" {
			t.Fatalf("expected model test-model, got %s", req.Model)
		}
		resp := map[string]any{
			"embeddings": [][]float32{{0.1, 0.2}, {0.3, 0.4}},
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(resp); err != nil {
			t.Fatalf("encode: %v", err)
		}
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder("test-model", server.URL)
	embeddings, err := embedder.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if atomic.LoadInt32(&embedCalls) != 1 {
		t.Fatalf("expected 1 batch call, got %d", embedCalls)
	}
}

func TestOllamaEmbedderFallback(t *testing.T) {
	var embedCalls int32
	var embeddingCalls int32
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/embed":
			atomic.AddInt32(&embedCalls, 1)
			http.NotFound(w, r)
		case "/api/embeddings":
			atomic.AddInt32(&embeddingCalls, 1)
			resp := map[string]any{
				"embedding": []float32{0.9, 0.8},
			}
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(resp); err != nil {
				t.Fatalf("encode: %v", err)
			}
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer server.Close()

	embedder := NewOllamaEmbedder("test-model", server.URL)
	embeddings, err := embedder.Embed(context.Background(), []string{"a", "b"})
	if err != nil {
		t.Fatalf("embed: %v", err)
	}
	if len(embeddings) != 2 {
		t.Fatalf("expected 2 embeddings, got %d", len(embeddings))
	}
	if atomic.LoadInt32(&embedCalls) != 1 {
		t.Fatalf("expected 1 embed call, got %d", embedCalls)
	}
	if atomic.LoadInt32(&embeddingCalls) != 2 {
		t.Fatalf("expected 2 embedding calls, got %d", embeddingCalls)
	}
}
