package rag

import (
	"context"
	"testing"
)

type stubEmbedder struct{}

func (s stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2, 0.3}, nil
}

func (s stubEmbedder) EmbedBatch(_ context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	for i := range texts {
		out[i] = []float32{0.1, 0.2, 0.3}
	}
	return out, nil
}

func (s stubEmbedder) Dimensions() int {
	return 3
}

func TestVectorStore_DeleteByID(t *testing.T) {
	ctx := context.Background()
	store, err := NewVectorStore(StoreConfig{PersistPath: t.TempDir(), Collection: "test"}, stubEmbedder{})
	if err != nil {
		t.Fatalf("new vector store: %v", err)
	}

	doc := Document{
		ID:        "doc-1",
		Content:   "hello",
		Embedding: []float32{0.1, 0.2, 0.3},
		Metadata:  map[string]string{},
	}

	if err := store.Add(ctx, []Document{doc}); err != nil {
		t.Fatalf("add document: %v", err)
	}
	if got := store.Count(); got != 1 {
		t.Fatalf("expected count 1, got %d", got)
	}

	if err := store.Delete(ctx, []string{"doc-1"}); err != nil {
		t.Fatalf("delete document: %v", err)
	}
	if got := store.Count(); got != 0 {
		t.Fatalf("expected count 0 after delete, got %d", got)
	}
}
