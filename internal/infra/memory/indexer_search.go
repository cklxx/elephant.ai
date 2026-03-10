package memory

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// Search performs hybrid (vector + BM25) retrieval.
func (i *Indexer) Search(ctx context.Context, _ string, query string, maxResults int, minScore float64) ([]SearchHit, error) {
	if i == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = defaultSearchMax
	}
	if minScore <= 0 {
		minScore = i.cfg.MinScore
	}

	store, err := i.storeForUser()
	if err != nil {
		return nil, err
	}
	embeddings, err := i.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) != 1 {
		return nil, fmt.Errorf("unexpected embedding response size")
	}
	if len(embeddings[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	if err := i.ensureSchema(ctx, store, len(embeddings[0])); err != nil {
		return nil, err
	}

	var (
		vecMatches  []VectorMatch
		textMatches []TextMatch
	)
	group, searchCtx := newIndexerErrGroup(ctx)
	group.Go(func() error {
		matches, err := i.searchVector(searchCtx, store, embeddings[0], maxResults)
		if err != nil {
			return err
		}
		vecMatches = matches
		return nil
	})
	group.Go(func() error {
		matches, err := i.searchBM25(searchCtx, store, query, maxResults)
		if err != nil {
			return err
		}
		textMatches = matches
		return nil
	})
	if err := group.Wait(); err != nil {
		return nil, err
	}
	results := mergeMatches(vecMatches, textMatches, maxResults, minScore, i.cfg.FusionWeightVector, i.cfg.FusionWeightBM25)
	for idx := range results {
		results[idx].NodeID = buildNodeID(results[idx].Path, results[idx].StartLine, results[idx].EndLine)
		relatedCount, err := i.countRelated(ctx, store, results[idx].Path, results[idx].StartLine, results[idx].EndLine)
		if err != nil {
			continue
		}
		results[idx].RelatedCount = relatedCount
	}
	return results, nil
}

// Related returns graph-adjacent memory entries for a source path or range.
func (i *Indexer) Related(ctx context.Context, _ string, path string, fromLine, toLine, maxResults int) ([]RelatedHit, error) {
	if i == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}
	if maxResults <= 0 {
		maxResults = defaultSearchMax
	}
	store, err := i.storeForUser()
	if err != nil {
		return nil, err
	}
	normalizedPath, err := i.normalizePath(path)
	if err != nil {
		return nil, err
	}
	results, err := store.SearchRelated(ctx, normalizedPath, fromLine, toLine, maxResults)
	if err != nil {
		return nil, err
	}
	out := make([]RelatedHit, 0, len(results))
	for _, match := range results {
		out = append(out, RelatedHit{
			Path:         match.Path,
			StartLine:    match.StartLine,
			EndLine:      match.EndLine,
			Score:        match.Score,
			Snippet:      buildSnippet(match.Text),
			RelationType: match.EdgeType,
			NodeID:       buildNodeID(match.Path, match.StartLine, match.EndLine),
		})
	}
	return out, nil
}

func (i *Indexer) searchVector(
	ctx context.Context,
	store *IndexStore,
	embedding []float32,
	maxResults int,
) ([]VectorMatch, error) {
	if i.searchVectorFn != nil {
		return i.searchVectorFn(ctx, store, embedding, maxResults)
	}
	return store.SearchVector(ctx, embedding, maxResults)
}

func (i *Indexer) searchBM25(
	ctx context.Context,
	store *IndexStore,
	query string,
	maxResults int,
) ([]TextMatch, error) {
	if i.searchBM25Fn != nil {
		return i.searchBM25Fn(ctx, store, query, maxResults)
	}
	return store.SearchBM25(ctx, query, maxResults)
}

func (i *Indexer) ensureSchema(ctx context.Context, store *IndexStore, dim int) error {
	if i.ensureSchemaFn != nil {
		return i.ensureSchemaFn(ctx, store, dim)
	}
	return store.EnsureSchema(ctx, dim)
}

func (i *Indexer) countRelated(
	ctx context.Context,
	store *IndexStore,
	path string,
	fromLine, toLine int,
) (int, error) {
	if i.countRelatedFn != nil {
		return i.countRelatedFn(ctx, store, path, fromLine, toLine)
	}
	return store.CountRelated(ctx, path, fromLine, toLine)
}

type indexerErrGroup struct {
	cancel context.CancelFunc

	wg sync.WaitGroup

	errOnce sync.Once
	err     error
}

func newIndexerErrGroup(ctx context.Context) (*indexerErrGroup, context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}
	groupCtx, cancel := context.WithCancel(ctx)
	return &indexerErrGroup{
		cancel: cancel,
	}, groupCtx
}

func (g *indexerErrGroup) Go(fn func() error) {
	if fn == nil {
		return
	}
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		if err := fn(); err != nil {
			g.errOnce.Do(func() {
				g.err = err
				g.cancel()
			})
		}
	}()
}

func (g *indexerErrGroup) Wait() error {
	g.wg.Wait()
	g.cancel()
	return g.err
}

func mergeMatches(vec []VectorMatch, text []TextMatch, limit int, minScore float64, weightVector, weightBM25 float64) []SearchHit {
	if limit <= 0 {
		limit = defaultSearchMax
	}
	if minScore <= 0 {
		minScore = defaultSearchMinScore
	}
	if weightVector == 0 && weightBM25 == 0 {
		weightVector = 0.7
		weightBM25 = 0.3
	}

	type scoreEntry struct {
		chunk      StoredChunk
		vector     float64
		text       float64
		finalScore float64
	}
	entries := map[int64]*scoreEntry{}

	for _, match := range vec {
		entry := entries[match.Chunk.ID]
		if entry == nil {
			entry = &scoreEntry{chunk: match.Chunk}
			entries[match.Chunk.ID] = entry
		}
		entry.vector = 1.0 / (1.0 + match.Distance)
	}
	for _, match := range text {
		entry := entries[match.Chunk.ID]
		if entry == nil {
			entry = &scoreEntry{chunk: match.Chunk}
			entries[match.Chunk.ID] = entry
		}
		score := match.BM25
		if score < 0 {
			score = 0
		}
		entry.text = 1.0 / (1.0 + score)
	}

	results := make([]SearchHit, 0, len(entries))
	for _, entry := range entries {
		entry.finalScore = (weightVector * entry.vector) + (weightBM25 * entry.text)
		if entry.finalScore < minScore {
			continue
		}
		source := "memory"
		if filepath.Base(entry.chunk.Path) == memoryFileName {
			source = "long_term"
		}
		results = append(results, SearchHit{
			Path:      entry.chunk.Path,
			StartLine: entry.chunk.StartLine,
			EndLine:   entry.chunk.EndLine,
			Score:     entry.finalScore,
			Snippet:   buildSnippet(entry.chunk.Text),
			Source:    source,
			NodeID:    buildNodeID(entry.chunk.Path, entry.chunk.StartLine, entry.chunk.EndLine),
		})
	}
	if len(results) > 1 {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Score == results[j].Score {
				return results[i].Path < results[j].Path
			}
			return results[i].Score > results[j].Score
		})
	}
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}
