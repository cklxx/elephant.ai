package main

import (
	"alex/internal/rag"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

func resolveAPIKey() (string, error) {
	cfg, _, err := loadRuntimeConfigSnapshot()
	if err != nil {
		return "", fmt.Errorf("load runtime configuration: %w", err)
	}
	if cfg.APIKey == "" {
		return "", fmt.Errorf("API key not configured (set runtime.api_key in ~/.alex/config.yaml or reference ${OPENAI_API_KEY})")
	}
	return cfg.APIKey, nil
}

// handleIndex indexes the repository for code search
func (c *CLI) handleIndex(args []string) error {
	// Parse arguments
	repoPath := "."
	for i, arg := range args {
		if arg == "--repo" && i+1 < len(args) {
			repoPath = args[i+1]
			break
		}
	}

	// Get absolute path
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		return fmt.Errorf("get absolute path: %w", err)
	}

	fmt.Printf("Indexing repository: %s\n", absPath)

	apiKey, err := resolveAPIKey()
	if err != nil {
		return err
	}

	// Create embedder
	fmt.Println("Initializing embedder...")
	embedder, err := rag.NewEmbedder(rag.EmbedderConfig{
		Provider:  "openai",
		Model:     "text-embedding-3-small",
		APIKey:    apiKey,
		BaseURL:   "https://api.openai.com/v1",
		CacheSize: 10000,
	})
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	// Create chunker
	chunker, err := rag.NewChunker(rag.ChunkerConfig{
		ChunkSize:    512,
		ChunkOverlap: 50,
	})
	if err != nil {
		return fmt.Errorf("create chunker: %w", err)
	}

	// Get index path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	indexPath := filepath.Join(homeDir, ".alex", "indices", filepath.Base(absPath))
	if err := os.MkdirAll(indexPath, 0755); err != nil {
		return fmt.Errorf("create index directory: %w", err)
	}

	fmt.Printf("Index path: %s\n", indexPath)

	// Create vector store
	fmt.Println("Initializing vector store...")
	store, err := rag.NewVectorStore(rag.StoreConfig{
		PersistPath: indexPath,
		Collection:  "code",
	}, embedder)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Create indexer
	indexer := rag.NewIndexer(rag.IndexerConfig{
		RepoPath: absPath,
	}, chunker, embedder, store)

	// Index repository
	fmt.Println("Indexing files...")
	start := time.Now()

	ctx := cliBaseContext()
	stats, err := indexer.Index(ctx)
	if err != nil {
		return fmt.Errorf("index repository: %w", err)
	}

	duration := time.Since(start)

	// Print statistics
	fmt.Println("\nIndexing completed!")
	fmt.Printf("  Total files:   %d\n", stats.TotalFiles)
	fmt.Printf("  Indexed files: %d\n", stats.IndexedFiles)
	fmt.Printf("  Total chunks:  %d\n", stats.TotalChunks)
	fmt.Printf("  Error files:   %d\n", stats.ErrorFiles)
	fmt.Printf("  Duration:      %s\n", duration.Round(time.Second))

	return nil
}

// handleSearch searches the indexed code
func (c *CLI) handleSearch(query string) error {
	// Get repository path
	repoPath, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("get current directory: %w", err)
	}

	fmt.Printf("Searching in: %s\n", repoPath)
	fmt.Printf("Query: %s\n\n", query)

	apiKey, err := resolveAPIKey()
	if err != nil {
		return err
	}

	// Create embedder
	embedder, err := rag.NewEmbedder(rag.EmbedderConfig{
		Provider:  "openai",
		Model:     "text-embedding-3-small",
		APIKey:    apiKey,
		BaseURL:   "https://api.openai.com/v1",
		CacheSize: 10000,
	})
	if err != nil {
		return fmt.Errorf("create embedder: %w", err)
	}

	// Get index path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return fmt.Errorf("get home directory: %w", err)
	}

	indexPath := filepath.Join(homeDir, ".alex", "indices", filepath.Base(repoPath))

	// Create vector store
	store, err := rag.NewVectorStore(rag.StoreConfig{
		PersistPath: indexPath,
		Collection:  "code",
	}, embedder)
	if err != nil {
		return fmt.Errorf("create vector store: %w", err)
	}
	defer func() { _ = store.Close() }()

	// Check if index exists
	if store.Count() == 0 {
		return fmt.Errorf("index not found. Please run 'alex index' first")
	}

	// Create retriever
	retriever := rag.NewRetriever(rag.RetrieverConfig{
		TopK:          5,
		MinSimilarity: 0.7,
	}, embedder, store)

	// Search
	ctx := cliBaseContext()
	results, err := retriever.Search(ctx, query)
	if err != nil {
		return fmt.Errorf("search failed: %w", err)
	}

	// Display results
	if len(results) == 0 {
		fmt.Println("No results found.")
		return nil
	}

	fmt.Printf("Found %d results:\n\n", len(results))
	for i, result := range results {
		fmt.Printf("%d. %s:%d-%d (similarity: %.2f)\n",
			i+1, result.FilePath, result.StartLine, result.EndLine, result.Similarity)
		fmt.Printf("   Language: %s\n", result.Language)
		fmt.Println("   Code:")
		codeLines := splitLines(result.Code)
		for _, line := range codeLines {
			fmt.Printf("   %s\n", line)
		}
		fmt.Println()
	}

	return nil
}

// splitLines splits text into lines
func splitLines(text string) []string {
	lines := []string{}
	current := ""
	for _, ch := range text {
		if ch == '\n' {
			lines = append(lines, current)
			current = ""
		} else {
			current += string(ch)
		}
	}
	if current != "" {
		lines = append(lines, current)
	}
	return lines
}
