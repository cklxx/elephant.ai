//go:build !cgo
// +build !cgo

package memory

import (
	"context"
	"fmt"
)

const errSQLiteVecCGODisabled = "sqlite-vec requires CGO (build with CGO_ENABLED=1)"

// StoredChunk represents a chunk stored in the index.
type StoredChunk struct {
	ID        int64
	Path      string
	StartLine int
	EndLine   int
	Text      string
}

// VectorMatch captures a vector search match.
type VectorMatch struct {
	Chunk    StoredChunk
	Distance float64
}

// TextMatch captures a BM25 search match.
type TextMatch struct {
	Chunk StoredChunk
	BM25  float64
}

// IndexedChunk captures a chunk ready for insertion.
type IndexedChunk struct {
	Path      string
	StartLine int
	EndLine   int
	Text      string
	Hash      string
	Embedding []float32
	Edges     []MemoryEdge
}

// MemoryEdge represents a graph edge between memory chunks/files.
type MemoryEdge struct {
	DstPath   string
	DstAnchor string
	EdgeType  string
	Direction string
}

// RelatedMatch captures a linked memory entry returned by graph traversal.
type RelatedMatch struct {
	Path      string
	Anchor    string
	EdgeType  string
	StartLine int
	EndLine   int
	Text      string
	Score     float64
}

// IndexStore is unavailable when CGO is disabled.
type IndexStore struct{}

// OpenIndexStore returns an error when CGO is disabled.
func OpenIndexStore(_ string) (*IndexStore, error) {
	return nil, fmt.Errorf(errSQLiteVecCGODisabled)
}

// EnsureSchema returns an error when CGO is disabled.
func (s *IndexStore) EnsureSchema(_ context.Context, _ int) error {
	return fmt.Errorf(errSQLiteVecCGODisabled)
}

// Close is a no-op stub.
func (s *IndexStore) Close() error {
	return nil
}

// LookupEmbeddings returns an error when CGO is disabled.
func (s *IndexStore) LookupEmbeddings(_ context.Context, _ []string) (map[string][]float32, error) {
	return nil, fmt.Errorf(errSQLiteVecCGODisabled)
}

// ReplaceChunks returns an error when CGO is disabled.
func (s *IndexStore) ReplaceChunks(_ context.Context, _ string, _ []IndexedChunk) error {
	return fmt.Errorf(errSQLiteVecCGODisabled)
}

// DeleteByPath returns an error when CGO is disabled.
func (s *IndexStore) DeleteByPath(_ context.Context, _ string) error {
	return fmt.Errorf(errSQLiteVecCGODisabled)
}

// SearchVector returns an error when CGO is disabled.
func (s *IndexStore) SearchVector(_ context.Context, _ []float32, _ int) ([]VectorMatch, error) {
	return nil, fmt.Errorf(errSQLiteVecCGODisabled)
}

// SearchBM25 returns an error when CGO is disabled.
func (s *IndexStore) SearchBM25(_ context.Context, _ string, _ int) ([]TextMatch, error) {
	return nil, fmt.Errorf(errSQLiteVecCGODisabled)
}

// CountRelated returns an error when CGO is disabled.
func (s *IndexStore) CountRelated(_ context.Context, _ string, _, _ int) (int, error) {
	return 0, fmt.Errorf(errSQLiteVecCGODisabled)
}

// SearchRelated returns an error when CGO is disabled.
func (s *IndexStore) SearchRelated(_ context.Context, _ string, _, _, _ int) ([]RelatedMatch, error) {
	return nil, fmt.Errorf(errSQLiteVecCGODisabled)
}
