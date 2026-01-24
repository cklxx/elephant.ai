package rag

import (
	"context"
	"fmt"
	"path/filepath"

	chromem "github.com/philippgille/chromem-go"
)

// StoreConfig holds vector store configuration
type StoreConfig struct {
	PersistPath string // Path to persist data
	Collection  string // Collection name (repo-specific)
}

// Document represents a stored document
type Document struct {
	ID        string
	Content   string
	Embedding []float32
	Metadata  map[string]string
}

// SearchResult represents a search result
type SearchResult struct {
	Document   Document
	Similarity float32 // 0.0 to 1.0
}

// VectorStore manages embeddings and similarity search
type VectorStore interface {
	// Add adds documents to the store
	Add(ctx context.Context, docs []Document) error

	// Search performs similarity search by embedding
	Search(ctx context.Context, queryEmbedding []float32, topK int, minSimilarity float32) ([]SearchResult, error)

	// SearchByText performs similarity search by text query
	SearchByText(ctx context.Context, queryText string, topK int, minSimilarity float32) ([]SearchResult, error)

	// Delete removes documents by ID
	Delete(ctx context.Context, ids []string) error

	// Count returns total document count
	Count() int

	// Close closes the store
	Close() error
}

// MetadataDeleter allows removing documents by metadata filters when supported by the backend.
type MetadataDeleter interface {
	DeleteByMetadata(ctx context.Context, metadata map[string]string) error
}

// chromemStore implements VectorStore using chromem-go
type chromemStore struct {
	db         *chromem.DB
	collection *chromem.Collection
	config     StoreConfig
	embedder   Embedder
}

// NewVectorStore creates a new vector store
func NewVectorStore(config StoreConfig, embedder Embedder) (VectorStore, error) {
	if config.Collection == "" {
		config.Collection = "default"
	}

	// Create DB with persistence
	var db *chromem.DB
	var err error

	if config.PersistPath != "" {
		persistFile := filepath.Join(config.PersistPath, "chromem.gob")
		db, err = chromem.NewPersistentDB(persistFile, false)
		if err != nil {
			return nil, fmt.Errorf("create persistent DB: %w", err)
		}
	} else {
		db = chromem.NewDB()
	}

	// Create embedding function wrapper
	embeddingFunc := func(ctx context.Context, text string) ([]float32, error) {
		return embedder.Embed(ctx, text)
	}

	// Create or get collection
	collection, err := db.GetOrCreateCollection(config.Collection, nil, embeddingFunc)
	if err != nil {
		return nil, fmt.Errorf("create collection: %w", err)
	}

	return &chromemStore{
		db:         db,
		collection: collection,
		config:     config,
		embedder:   embedder,
	}, nil
}

// Add adds documents to the store
func (s *chromemStore) Add(ctx context.Context, docs []Document) error {
	if len(docs) == 0 {
		return nil
	}

	// Add documents one by one (chromem-go API)
	for _, doc := range docs {
		err := s.collection.AddDocument(ctx, chromem.Document{
			ID:        doc.ID,
			Content:   doc.Content,
			Embedding: doc.Embedding,
			Metadata:  doc.Metadata,
		})
		if err != nil {
			return fmt.Errorf("add document %s: %w", doc.ID, err)
		}
	}

	return nil
}

// Search performs similarity search by embedding
// NOTE: chromem-go v0.7.0 doesn't support direct embedding queries
func (s *chromemStore) Search(ctx context.Context, queryEmbedding []float32, topK int, minSimilarity float32) ([]SearchResult, error) {
	// Not supported by chromem-go v0.7.0
	return nil, fmt.Errorf("embedding-based search not supported by chromem-go. Use SearchByText instead")
}

// SearchByText performs similarity search using text query
func (s *chromemStore) SearchByText(ctx context.Context, queryText string, topK int, minSimilarity float32) ([]SearchResult, error) {
	if topK <= 0 {
		topK = 5
	}

	// Query collection (chromem-go will generate embedding internally)
	results, err := s.collection.Query(ctx, queryText, topK, nil, nil)
	if err != nil {
		return nil, fmt.Errorf("query collection: %w", err)
	}

	// Convert results
	var searchResults []SearchResult
	for _, r := range results {
		// chromem uses cosine similarity (higher is better, 0-1 range)
		similarity := r.Similarity

		// Filter by minimum similarity
		if similarity < minSimilarity {
			continue
		}

		searchResults = append(searchResults, SearchResult{
			Document: Document{
				ID:        r.ID,
				Content:   r.Content,
				Embedding: r.Embedding,
				Metadata:  r.Metadata,
			},
			Similarity: similarity,
		})
	}

	return searchResults, nil
}

// Delete removes documents by ID
func (s *chromemStore) Delete(ctx context.Context, ids []string) error {
	if len(ids) == 0 {
		return nil
	}

	// Delete from collection
	for _, id := range ids {
		err := s.collection.Delete(ctx, nil, map[string]string{"id": id})
		if err != nil {
			return fmt.Errorf("delete document %s: %w", id, err)
		}
	}

	return nil
}

// DeleteByMetadata removes documents matching the provided metadata filters.
func (s *chromemStore) DeleteByMetadata(ctx context.Context, metadata map[string]string) error {
	if len(metadata) == 0 {
		return nil
	}
	return s.collection.Delete(ctx, nil, metadata)
}

// Count returns total document count
func (s *chromemStore) Count() int {
	return s.collection.Count()
}

// Close closes the store
func (s *chromemStore) Close() error {
	// chromem-go auto-persists on changes, no explicit close needed
	return nil
}
