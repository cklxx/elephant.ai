//go:build !cgo
// +build !cgo

package memory

import (
	"context"
	"fmt"
)

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
}

// IndexStore is unavailable when CGO is disabled.
type IndexStore struct{}

// OpenIndexStore returns an error when CGO is disabled.
func OpenIndexStore(_ string) (*IndexStore, error) {
	return nil, fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}

// EnsureSchema returns an error when CGO is disabled.
func (s *IndexStore) EnsureSchema(_ context.Context, _ int) error {
	if s == nil {
		return fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
	}
	return fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}

// Close is a no-op stub.
func (s *IndexStore) Close() error {
	return nil
}

// LookupEmbeddings returns an error when CGO is disabled.
func (s *IndexStore) LookupEmbeddings(_ context.Context, _ []string) (map[string][]float32, error) {
	return nil, fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}

// ReplaceChunks returns an error when CGO is disabled.
func (s *IndexStore) ReplaceChunks(_ context.Context, _ string, _ []IndexedChunk) error {
	return fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}

// DeleteByPath returns an error when CGO is disabled.
func (s *IndexStore) DeleteByPath(_ context.Context, _ string) error {
	return fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}

// SearchVector returns an error when CGO is disabled.
func (s *IndexStore) SearchVector(_ context.Context, _ []float32, _ int) ([]VectorMatch, error) {
	return nil, fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}

// SearchBM25 returns an error when CGO is disabled.
func (s *IndexStore) SearchBM25(_ context.Context, _ string, _ int) ([]TextMatch, error) {
	return nil, fmt.Errorf("sqlite-vec requires CGO (build with CGO_ENABLED=1)")
}
