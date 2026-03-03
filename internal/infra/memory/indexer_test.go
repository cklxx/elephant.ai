package memory

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"
)

type stubEmbeddingProvider struct {
	vectors [][]float32
	err     error
}

func (s stubEmbeddingProvider) Embed(_ context.Context, texts []string) ([][]float32, error) {
	if s.err != nil {
		return nil, s.err
	}
	if len(s.vectors) > 0 {
		return s.vectors, nil
	}
	out := make([][]float32, len(texts))
	for idx := range out {
		out[idx] = []float32{0.1}
	}
	return out, nil
}

func TestMergeMatchesRanking(t *testing.T) {
	vecMatches := []VectorMatch{
		{
			Chunk: StoredChunk{
				ID:        1,
				Path:      "memory/2026-02-02.md",
				StartLine: 1,
				EndLine:   2,
				Text:      "Chunk one",
			},
			Distance: 0.1,
		},
		{
			Chunk: StoredChunk{
				ID:        2,
				Path:      "memory/2026-02-02.md",
				StartLine: 3,
				EndLine:   4,
				Text:      "Chunk two",
			},
			Distance: 0.0,
		},
	}
	textMatches := []TextMatch{
		{
			Chunk: StoredChunk{
				ID:        1,
				Path:      "memory/2026-02-02.md",
				StartLine: 1,
				EndLine:   2,
				Text:      "Chunk one",
			},
			BM25: 10,
		},
		{
			Chunk: StoredChunk{
				ID:        3,
				Path:      "MEMORY.md",
				StartLine: 5,
				EndLine:   6,
				Text:      "Chunk three",
			},
			BM25: 0,
		},
	}

	results := mergeMatches(vecMatches, textMatches, 5, 0.1, 0.7, 0.3)
	if len(results) < 3 {
		t.Fatalf("expected 3 results, got %d", len(results))
	}
	if results[0].StartLine != 3 {
		t.Fatalf("expected chunk with best vector distance first, got %+v", results[0])
	}
	if results[1].StartLine != 1 {
		t.Fatalf("expected blended chunk second, got %+v", results[1])
	}
	if results[2].Path != "MEMORY.md" {
		t.Fatalf("expected long-term memory third, got %+v", results[2])
	}
	if results[2].Source != "long_term" {
		t.Fatalf("expected long_term source, got %+v", results[2])
	}
}

func TestIndexerSearchRunsVectorAndBM25InParallel(t *testing.T) {
	rootDir := t.TempDir()
	indexer, err := NewIndexer(
		rootDir,
		IndexerConfig{DBPath: filepath.Join(rootDir, "index.sqlite")},
		stubEmbeddingProvider{vectors: [][]float32{{0.1, 0.2}}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	indexer.store = &IndexStore{}
	indexer.ensureSchemaFn = func(context.Context, *IndexStore, int) error { return nil }
	indexer.countRelatedFn = func(context.Context, *IndexStore, string, int, int) (int, error) { return 0, nil }

	vectorStarted := make(chan struct{})
	bm25Started := make(chan struct{})
	indexer.searchVectorFn = func(_ context.Context, _ *IndexStore, _ []float32, _ int) ([]VectorMatch, error) {
		close(vectorStarted)
		select {
		case <-bm25Started:
		case <-time.After(time.Second):
			return nil, errors.New("bm25 search did not start in parallel")
		}
		return []VectorMatch{
			{
				Chunk: StoredChunk{
					ID:        1,
					Path:      "memory/2026-02-02.md",
					StartLine: 1,
					EndLine:   2,
					Text:      "vector match",
				},
				Distance: 0.0,
			},
		}, nil
	}
	indexer.searchBM25Fn = func(_ context.Context, _ *IndexStore, _ string, _ int) ([]TextMatch, error) {
		close(bm25Started)
		select {
		case <-vectorStarted:
		case <-time.After(time.Second):
			return nil, errors.New("vector search did not start in parallel")
		}
		return []TextMatch{
			{
				Chunk: StoredChunk{
					ID:        2,
					Path:      "memory/2026-02-03.md",
					StartLine: 3,
					EndLine:   4,
					Text:      "bm25 match",
				},
				BM25: 0,
			},
		}, nil
	}

	results, err := indexer.Search(context.Background(), "user-1", "query", 5, 0.1)
	if err != nil {
		t.Fatalf("Search: %v", err)
	}
	if len(results) != 2 {
		t.Fatalf("expected 2 merged results, got %d", len(results))
	}
}

func TestIndexerSearchReturnsErrgroupError(t *testing.T) {
	rootDir := t.TempDir()
	indexer, err := NewIndexer(
		rootDir,
		IndexerConfig{DBPath: filepath.Join(rootDir, "index.sqlite")},
		stubEmbeddingProvider{vectors: [][]float32{{0.1, 0.2}}},
		nil,
	)
	if err != nil {
		t.Fatalf("NewIndexer: %v", err)
	}
	indexer.store = &IndexStore{}
	indexer.ensureSchemaFn = func(context.Context, *IndexStore, int) error { return nil }

	vectorErr := errors.New("vector search failed")
	bm25Cancelled := make(chan struct{})
	indexer.searchVectorFn = func(_ context.Context, _ *IndexStore, _ []float32, _ int) ([]VectorMatch, error) {
		return nil, vectorErr
	}
	indexer.searchBM25Fn = func(ctx context.Context, _ *IndexStore, _ string, _ int) ([]TextMatch, error) {
		<-ctx.Done()
		close(bm25Cancelled)
		return nil, ctx.Err()
	}

	_, err = indexer.Search(context.Background(), "user-1", "query", 5, 0.1)
	if !errors.Is(err, vectorErr) {
		t.Fatalf("expected vector error, got %v", err)
	}
	select {
	case <-bm25Cancelled:
	case <-time.After(time.Second):
		t.Fatalf("expected bm25 branch to observe errgroup cancellation")
	}
}
