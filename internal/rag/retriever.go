package rag

import (
	"context"
	"fmt"
	"strings"
)

// RetrieverConfig holds retrieval configuration
type RetrieverConfig struct {
	TopK          int     // Number of results to return (default: 5)
	MinSimilarity float32 // Minimum similarity threshold (0.0-1.0, default: 0.7)
}

// RetrievalResult represents a code search result
type RetrievalResult struct {
	FilePath   string
	StartLine  int
	EndLine    int
	Language   string
	Code       string
	Similarity float32
}

// Retriever searches indexed code
type Retriever struct {
	config   RetrieverConfig
	embedder Embedder
	store    VectorStore
}

// NewRetriever creates a new retriever
func NewRetriever(config RetrieverConfig, embedder Embedder, store VectorStore) *Retriever {
	if config.TopK == 0 {
		config.TopK = 5
	}
	if config.MinSimilarity == 0 {
		config.MinSimilarity = 0.7
	}

	return &Retriever{
		config:   config,
		embedder: embedder,
		store:    store,
	}
}

// Search searches for code using natural language or code query
func (r *Retriever) Search(ctx context.Context, query string) ([]RetrievalResult, error) {
	if query == "" {
		return nil, fmt.Errorf("empty query")
	}

	// Use text-based search (chromem-go generates embeddings internally)
	searchResults, err := r.store.SearchByText(ctx, query, r.config.TopK, r.config.MinSimilarity)
	if err != nil {
		return nil, fmt.Errorf("search store: %w", err)
	}

	// Convert to retrieval results
	results := make([]RetrievalResult, 0, len(searchResults))
	for _, sr := range searchResults {
		result := RetrievalResult{
			FilePath:   sr.Document.Metadata["file_path"],
			Language:   sr.Document.Metadata["language"],
			Code:       sr.Document.Content,
			Similarity: sr.Similarity,
		}

		// Parse line numbers
		if startLine, ok := sr.Document.Metadata["start_line"]; ok {
			fmt.Sscanf(startLine, "%d", &result.StartLine)
		}
		if endLine, ok := sr.Document.Metadata["end_line"]; ok {
			fmt.Sscanf(endLine, "%d", &result.EndLine)
		}

		results = append(results, result)
	}

	return results, nil
}

// FormatResults formats retrieval results for LLM consumption
func (r *Retriever) FormatResults(results []RetrievalResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	sb.WriteString(fmt.Sprintf("Found %d relevant code chunks:\n\n", len(results)))

	for i, result := range results {
		sb.WriteString(fmt.Sprintf("%d. %s:%d-%d (similarity: %.2f)\n",
			i+1, result.FilePath, result.StartLine, result.EndLine, result.Similarity))

		// Add language if known
		if result.Language != "" {
			sb.WriteString(fmt.Sprintf("   Language: %s\n", result.Language))
		}

		// Add code with proper formatting
		sb.WriteString("   Code:\n")
		codeLines := strings.Split(strings.TrimSpace(result.Code), "\n")
		for _, line := range codeLines {
			sb.WriteString("   " + line + "\n")
		}
		sb.WriteString("\n")
	}

	return sb.String()
}

// FormatResultsCompact formats results in a compact format
func (r *Retriever) FormatResultsCompact(results []RetrievalResult) string {
	if len(results) == 0 {
		return "No results found."
	}

	var sb strings.Builder
	for i, result := range results {
		if i > 0 {
			sb.WriteString("\n---\n\n")
		}
		sb.WriteString(fmt.Sprintf("File: %s (lines %d-%d, similarity: %.2f)\n",
			result.FilePath, result.StartLine, result.EndLine, result.Similarity))
		sb.WriteString("```" + result.Language + "\n")
		sb.WriteString(strings.TrimSpace(result.Code))
		sb.WriteString("\n```\n")
	}

	return sb.String()
}
