package memory

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"alex/internal/rag"
)

type stubKeywordStore struct {
	searchResults []Entry
	lastQuery     Query
	lastInsert    Entry
}

func (s *stubKeywordStore) EnsureSchema(_ context.Context) error { return nil }
func (s *stubKeywordStore) Insert(_ context.Context, entry Entry) (Entry, error) {
	if entry.Key == "" {
		entry.Key = "stub-key"
	}
	if entry.CreatedAt.IsZero() {
		entry.CreatedAt = time.Now()
	}
	s.lastInsert = entry
	return entry, nil
}
func (s *stubKeywordStore) Search(_ context.Context, query Query) ([]Entry, error) {
	s.lastQuery = query
	return s.searchResults, nil
}
func (s *stubKeywordStore) Delete(_ context.Context, _ []string) error { return nil }
func (s *stubKeywordStore) Prune(_ context.Context, _ RetentionPolicy) ([]string, error) {
	return nil, nil
}

type stubVectorStore struct {
	addedDocs     []rag.Document
	searchResults []rag.SearchResult
	lastFilter    map[string]string
}

func (s *stubVectorStore) Add(_ context.Context, docs []rag.Document) error {
	s.addedDocs = append(s.addedDocs, docs...)
	return nil
}
func (s *stubVectorStore) Search(_ context.Context, _ []float32, _ int, _ float32, _ map[string]string) ([]rag.SearchResult, error) {
	return nil, nil
}
func (s *stubVectorStore) SearchByText(_ context.Context, _ string, _ int, _ float32, metadata map[string]string) ([]rag.SearchResult, error) {
	s.lastFilter = metadata
	return s.searchResults, nil
}
func (s *stubVectorStore) Delete(_ context.Context, _ []string) error { return nil }
func (s *stubVectorStore) Count() int                                 { return 0 }
func (s *stubVectorStore) Close() error                               { return nil }

type stubEmbedder struct{}

func (s stubEmbedder) Embed(_ context.Context, _ string) ([]float32, error) {
	return []float32{0.1, 0.2}, nil
}
func (s stubEmbedder) EmbedBatch(_ context.Context, _ []string) ([][]float32, error) {
	return [][]float32{{0.1, 0.2}}, nil
}
func (s stubEmbedder) Dimensions() int { return 2 }

func TestHybridStoreInsertAddsVectorDoc(t *testing.T) {
	keywordStore := &stubKeywordStore{}
	vectorStore := &stubVectorStore{}
	store := NewHybridStore(keywordStore, vectorStore, stubEmbedder{}, 0.5, 0.7, false)

	entry, err := store.Insert(context.Background(), Entry{
		Key:      "mem-1",
		UserID:   "user-1",
		Content:  "deployment notes",
		Keywords: []string{"deployment", "release"},
	})
	if err != nil {
		t.Fatalf("insert error: %v", err)
	}
	if entry.Key != "mem-1" {
		t.Fatalf("expected key mem-1, got %s", entry.Key)
	}
	if len(vectorStore.addedDocs) != 1 {
		t.Fatalf("expected vector add, got %d", len(vectorStore.addedDocs))
	}
	doc := vectorStore.addedDocs[0]
	if doc.ID != "mem-1" {
		t.Fatalf("expected doc id mem-1, got %s", doc.ID)
	}
	if doc.Metadata["user_id"] != "user-1" {
		t.Fatalf("expected user_id metadata, got %q", doc.Metadata["user_id"])
	}
	if !strings.Contains(doc.Metadata["keywords"], "deployment") {
		t.Fatalf("expected keywords metadata, got %q", doc.Metadata["keywords"])
	}
}

func TestHybridStoreSearchMergesResults(t *testing.T) {
	keywordStore := &stubKeywordStore{
		searchResults: []Entry{
			{Key: "a", Content: "keyword a"},
			{Key: "b", Content: "keyword b"},
		},
	}
	vectorStore := &stubVectorStore{
		searchResults: []rag.SearchResult{
			{Document: rag.Document{ID: "c", Content: "vector c", Metadata: map[string]string{"user_id": "user-1"}}},
			{Document: rag.Document{ID: "a", Content: "vector a", Metadata: map[string]string{"user_id": "user-1"}}},
		},
	}
	store := NewHybridStore(keywordStore, vectorStore, stubEmbedder{}, 0.5, 0.2, false)

	results, err := store.Search(context.Background(), Query{
		UserID:   "user-1",
		Text:     "deployment",
		Keywords: []string{"deploy"},
		Limit:    5,
	})
	if err != nil {
		t.Fatalf("search error: %v", err)
	}
	if len(results) != 3 {
		t.Fatalf("expected 3 merged results, got %d", len(results))
	}
	if results[0].Key != "a" {
		t.Fatalf("expected result 'a' to rank first, got %q", results[0].Key)
	}
	if vectorStore.lastFilter["user_id"] != "user-1" {
		t.Fatalf("expected vector filter user_id, got %+v", vectorStore.lastFilter)
	}
}

type failingVectorStore struct{}

func (s failingVectorStore) Add(_ context.Context, _ []rag.Document) error {
	return fmt.Errorf("vector add failed")
}
func (s failingVectorStore) Search(_ context.Context, _ []float32, _ int, _ float32, _ map[string]string) ([]rag.SearchResult, error) {
	return nil, nil
}
func (s failingVectorStore) SearchByText(_ context.Context, _ string, _ int, _ float32, _ map[string]string) ([]rag.SearchResult, error) {
	return nil, nil
}
func (s failingVectorStore) Delete(_ context.Context, _ []string) error { return nil }
func (s failingVectorStore) Count() int                                 { return 0 }
func (s failingVectorStore) Close() error                               { return nil }

func TestHybridStoreInsertAllowsVectorFailures(t *testing.T) {
	keywordStore := &stubKeywordStore{}
	vectorStore := failingVectorStore{}
	store := NewHybridStore(keywordStore, vectorStore, stubEmbedder{}, 0.5, 0.7, true)

	_, err := store.Insert(context.Background(), Entry{
		Key:     "mem-1",
		UserID:  "user-1",
		Content: "deployment notes",
	})
	if err != nil {
		t.Fatalf("expected insert to succeed when vector failures allowed: %v", err)
	}
}
