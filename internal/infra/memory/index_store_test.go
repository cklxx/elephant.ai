//go:build cgo
// +build cgo

package memory

import (
	"context"
	"path/filepath"
	"testing"
)

func TestIndexStoreReplaceSearchAndDelete(t *testing.T) {
	ctx := context.Background()
	dir := t.TempDir()
	store, err := OpenIndexStore(filepath.Join(dir, "index.sqlite"))
	if err != nil {
		t.Fatalf("open store: %v", err)
	}
	defer store.Close()

	embedding := []float32{0.1, 0.2, 0.3}
	if err := store.EnsureSchema(ctx, len(embedding)); err != nil {
		t.Fatalf("ensure schema: %v", err)
	}

	path := filepath.Join("memory", "2026-02-02.md")
	hash := hashText("Prefer TypeScript over JavaScript.")
	err = store.ReplaceChunks(ctx, path, []IndexedChunk{
		{
			Path:      path,
			StartLine: 1,
			EndLine:   2,
			Text:      "Prefer TypeScript over JavaScript.",
			Hash:      hash,
			Embedding: embedding,
		},
	})
	if err != nil {
		t.Fatalf("replace chunks: %v", err)
	}

	var bm25Matches []TextMatch
	if store.ftsEnabled {
		bm25Matches, err = store.SearchBM25(ctx, "TypeScript", 5)
		if err != nil {
			t.Fatalf("bm25 search: %v", err)
		}
		if len(bm25Matches) == 0 {
			t.Fatalf("expected bm25 matches")
		}
	}

	vecMatches, err := store.SearchVector(ctx, embedding, 5)
	if err != nil {
		t.Fatalf("vector search: %v", err)
	}
	if len(vecMatches) == 0 {
		t.Fatalf("expected vector matches")
	}

	cache, err := store.LookupEmbeddings(ctx, []string{hash})
	if err != nil {
		t.Fatalf("lookup cache: %v", err)
	}
	if len(cache[hash]) != len(embedding) {
		t.Fatalf("expected cached embedding length %d, got %d", len(embedding), len(cache[hash]))
	}

	if err := store.DeleteByPath(ctx, path); err != nil {
		t.Fatalf("delete by path: %v", err)
	}
	if store.ftsEnabled {
		bm25Matches, err = store.SearchBM25(ctx, "TypeScript", 5)
		if err != nil {
			t.Fatalf("bm25 search after delete: %v", err)
		}
		if len(bm25Matches) != 0 {
			t.Fatalf("expected no matches after delete")
		}
	}
}
