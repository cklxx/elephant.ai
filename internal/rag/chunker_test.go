package rag

import (
	"testing"
)

func TestChunker_ChunkText(t *testing.T) {
	chunker, err := NewChunker(ChunkerConfig{
		ChunkSize:    100, // Small for testing
		ChunkOverlap: 10,
	})
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	text := `package main

import "fmt"

func main() {
	fmt.Println("Hello, World!")
}

func add(a, b int) int {
	return a + b
}

func multiply(a, b int) int {
	return a * b
}`

	metadata := map[string]string{
		"file_path": "test.go",
		"language":  "go",
	}

	chunks, err := chunker.ChunkText(text, metadata)
	if err != nil {
		t.Fatalf("failed to chunk text: %v", err)
	}

	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}

	// Verify metadata is preserved
	for i, chunk := range chunks {
		if chunk.Metadata["file_path"] != "test.go" {
			t.Errorf("chunk %d: file_path metadata not preserved", i)
		}
		if chunk.Metadata["language"] != "go" {
			t.Errorf("chunk %d: language metadata not preserved", i)
		}
		if chunk.StartLine < 0 {
			t.Errorf("chunk %d: invalid start line %d", i, chunk.StartLine)
		}
		if chunk.EndLine < chunk.StartLine {
			t.Errorf("chunk %d: end line %d < start line %d", i, chunk.EndLine, chunk.StartLine)
		}
	}
}

func TestChunker_CountTokens(t *testing.T) {
	chunker, err := NewChunker(ChunkerConfig{})
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	tests := []struct {
		text      string
		minTokens int
	}{
		{"Hello", 1},
		{"Hello, World!", 2},
		{"package main\n\nimport \"fmt\"", 4},
	}

	for _, tt := range tests {
		count, err := chunker.CountTokens(tt.text)
		if err != nil {
			t.Errorf("failed to count tokens for %q: %v", tt.text, err)
		}
		if count < tt.minTokens {
			t.Errorf("expected at least %d tokens for %q, got %d", tt.minTokens, tt.text, count)
		}
	}
}

func TestChunker_MetadataIsolation(t *testing.T) {
	chunker, err := NewChunker(ChunkerConfig{
		ChunkSize:    10,
		ChunkOverlap: 0,
	})
	if err != nil {
		t.Fatalf("failed to create chunker: %v", err)
	}

	text := ""
	for i := 0; i < 40; i++ {
		text += "line one two three four five\n"
	}

	metadata := map[string]string{
		"file_path": "test.go",
		"language":  "go",
	}

	chunks, err := chunker.ChunkText(text, metadata)
	if err != nil {
		t.Fatalf("failed to chunk text: %v", err)
	}
	if len(chunks) < 2 {
		t.Fatalf("expected at least 2 chunks, got %d", len(chunks))
	}

	chunks[0].Metadata["file_path"] = "mutated.go"
	if chunks[1].Metadata["file_path"] != "test.go" {
		t.Fatalf("chunk metadata should be isolated; got %q", chunks[1].Metadata["file_path"])
	}
	if metadata["file_path"] != "test.go" {
		t.Fatalf("original metadata map should not be mutated; got %q", metadata["file_path"])
	}
}
