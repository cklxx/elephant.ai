package memory

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"alex/internal/rag"
)

const defaultRRFK = 60.0

// HybridStore combines keyword-based memory search with vector similarity search.
type HybridStore struct {
	keywordStore  Store
	vectorStore   rag.VectorStore
	embedder      rag.Embedder
	alpha         float64
	minSimilarity float32
}

// NewHybridStore constructs a HybridStore.
func NewHybridStore(keyword Store, vector rag.VectorStore, embedder rag.Embedder, alpha float64, minSimilarity float32) *HybridStore {
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	if minSimilarity < 0 {
		minSimilarity = 0
	}
	return &HybridStore{
		keywordStore:  keyword,
		vectorStore:   vector,
		embedder:      embedder,
		alpha:         alpha,
		minSimilarity: minSimilarity,
	}
}

// EnsureSchema initializes underlying stores.
func (s *HybridStore) EnsureSchema(ctx context.Context) error {
	if s.keywordStore == nil {
		return fmt.Errorf("hybrid memory store missing keyword store")
	}
	return s.keywordStore.EnsureSchema(ctx)
}

// Insert writes to the keyword store and indexes the entry into the vector store.
func (s *HybridStore) Insert(ctx context.Context, entry Entry) (Entry, error) {
	if s.keywordStore == nil {
		return entry, fmt.Errorf("hybrid memory store missing keyword store")
	}
	saved, err := s.keywordStore.Insert(ctx, entry)
	if err != nil {
		return saved, err
	}
	if s.vectorStore == nil || s.embedder == nil {
		return saved, nil
	}

	embedding, err := s.embedder.Embed(ctx, saved.Content)
	if err != nil {
		return saved, fmt.Errorf("embed memory: %w", err)
	}

	meta := map[string]string{
		"user_id":    saved.UserID,
		"key":        saved.Key,
		"keywords":   strings.Join(saved.Keywords, ","),
		"created_at": saved.CreatedAt.Format(time.RFC3339),
	}
	if saved.Slots != nil {
		if data, err := json.Marshal(saved.Slots); err == nil {
			meta["slots"] = string(data)
		}
	}

	doc := rag.Document{
		ID:        saved.Key,
		Content:   saved.Content,
		Embedding: embedding,
		Metadata:  meta,
	}

	if err := s.vectorStore.Add(ctx, []rag.Document{doc}); err != nil {
		return saved, fmt.Errorf("index memory: %w", err)
	}
	return saved, nil
}

// Search performs hybrid keyword + semantic search using reciprocal rank fusion.
func (s *HybridStore) Search(ctx context.Context, query Query) ([]Entry, error) {
	if s.keywordStore == nil {
		return nil, fmt.Errorf("hybrid memory store missing keyword store")
	}

	keywordResults, err := s.keywordStore.Search(ctx, query)
	if err != nil {
		return nil, err
	}

	vectorResults, err := s.vectorSearch(ctx, query)
	if err != nil {
		return nil, err
	}

	merged := reciprocalRankFusion(keywordResults, vectorResults, s.alpha)
	if query.Limit > 0 && len(merged) > query.Limit {
		merged = merged[:query.Limit]
	}
	return merged, nil
}

func (s *HybridStore) vectorSearch(ctx context.Context, query Query) ([]Entry, error) {
	if s.vectorStore == nil || s.embedder == nil {
		return nil, nil
	}

	queryText := strings.TrimSpace(query.Text)
	if queryText == "" {
		queryText = strings.Join(query.Keywords, " ")
	}
	if queryText == "" && len(query.Terms) > 0 {
		queryText = strings.Join(query.Terms, " ")
	}
	if queryText == "" {
		return nil, nil
	}

	filters := map[string]string{}
	if query.UserID != "" {
		filters["user_id"] = query.UserID
	}

	results, err := s.vectorStore.SearchByText(ctx, queryText, query.Limit, s.minSimilarity, filters)
	if err != nil {
		return nil, err
	}

	out := make([]Entry, 0, len(results))
	for _, res := range results {
		entry := entryFromVectorResult(res)
		if !matchSlots(entry.Slots, query.Slots) {
			continue
		}
		out = append(out, entry)
	}
	return out, nil
}

func entryFromVectorResult(result rag.SearchResult) Entry {
	entry := Entry{
		Key:      result.Document.ID,
		UserID:   result.Document.Metadata["user_id"],
		Content:  result.Document.Content,
		Keywords: splitKeywords(result.Document.Metadata["keywords"]),
	}
	if created := strings.TrimSpace(result.Document.Metadata["created_at"]); created != "" {
		if ts, err := time.Parse(time.RFC3339, created); err == nil {
			entry.CreatedAt = ts
		}
	}
	if rawSlots := strings.TrimSpace(result.Document.Metadata["slots"]); rawSlots != "" {
		var slots map[string]string
		if err := json.Unmarshal([]byte(rawSlots), &slots); err == nil {
			entry.Slots = slots
		}
	}
	entry.Terms = collectTerms(entry.Content, entry.Keywords, entry.Slots)
	return entry
}

func splitKeywords(raw string) []string {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}

func reciprocalRankFusion(keywordResults, vectorResults []Entry, alpha float64) []Entry {
	if len(keywordResults) == 0 && len(vectorResults) == 0 {
		return nil
	}
	if alpha < 0 {
		alpha = 0
	}
	if alpha > 1 {
		alpha = 1
	}
	keywordWeight := 1 - alpha

	type scoredEntry struct {
		entry Entry
		score float64
	}

	scored := make(map[string]*scoredEntry)
	add := func(entries []Entry, weight float64) {
		for idx, entry := range entries {
			if entry.Key == "" {
				continue
			}
			score := weight / (defaultRRFK + float64(idx+1))
			item, ok := scored[entry.Key]
			if !ok {
				item = &scoredEntry{entry: entry}
				scored[entry.Key] = item
			}
			item.score += score
		}
	}

	add(keywordResults, keywordWeight)
	add(vectorResults, alpha)

	sorted := make([]scoredEntry, 0, len(scored))
	for _, entry := range scored {
		sorted = append(sorted, *entry)
	}
	sort.SliceStable(sorted, func(i, j int) bool {
		if sorted[i].score == sorted[j].score {
			return sorted[i].entry.CreatedAt.After(sorted[j].entry.CreatedAt)
		}
		return sorted[i].score > sorted[j].score
	})

	out := make([]Entry, 0, len(sorted))
	for _, entry := range sorted {
		out = append(out, entry.entry)
	}
	return out
}
