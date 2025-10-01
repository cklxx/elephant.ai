package rag

import (
	"context"
	"os"
	"testing"
)

func TestEmbedder_Integration(t *testing.T) {
	// Skip if no API key
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set, skipping integration test")
	}

	embedder, err := NewEmbedder(EmbedderConfig{
		Provider:  "openai",
		Model:     "text-embedding-3-small",
		APIKey:    apiKey,
		BaseURL:   "https://api.openai.com/v1",
		CacheSize: 10,
	})
	if err != nil {
		t.Fatalf("failed to create embedder: %v", err)
	}

	ctx := context.Background()

	t.Run("single embedding", func(t *testing.T) {
		text := "Hello, World!"
		embedding, err := embedder.Embed(ctx, text)
		if err != nil {
			t.Fatalf("failed to embed text: %v", err)
		}

		if len(embedding) != 1536 {
			t.Errorf("expected 1536 dimensions, got %d", len(embedding))
		}
	})

	t.Run("batch embedding", func(t *testing.T) {
		texts := []string{"Hello", "World", "Test"}
		embeddings, err := embedder.EmbedBatch(ctx, texts)
		if err != nil {
			t.Fatalf("failed to embed batch: %v", err)
		}

		if len(embeddings) != len(texts) {
			t.Errorf("expected %d embeddings, got %d", len(texts), len(embeddings))
		}

		for i, emb := range embeddings {
			if len(emb) != 1536 {
				t.Errorf("embedding %d: expected 1536 dimensions, got %d", i, len(emb))
			}
		}
	})

	t.Run("cache hit", func(t *testing.T) {
		text := "cached text"

		// First call
		emb1, err := embedder.Embed(ctx, text)
		if err != nil {
			t.Fatalf("failed to embed text: %v", err)
		}

		// Second call (should hit cache)
		emb2, err := embedder.Embed(ctx, text)
		if err != nil {
			t.Fatalf("failed to embed text: %v", err)
		}

		// Compare embeddings
		if len(emb1) != len(emb2) {
			t.Error("cached embedding has different length")
		}
	})
}

func TestEmbedder_Dimensions(t *testing.T) {
	embedder, _ := NewEmbedder(EmbedderConfig{
		Model: "text-embedding-3-small",
	})

	if embedder.Dimensions() != 1536 {
		t.Errorf("expected 1536 dimensions, got %d", embedder.Dimensions())
	}
}
