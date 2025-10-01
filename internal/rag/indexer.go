package rag

import (
	"context"
	"crypto/sha256"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

// IndexerConfig holds indexing configuration
type IndexerConfig struct {
	RepoPath       string
	ExcludeDirs    []string // e.g., .git, node_modules, vendor
	CodeExtensions []string // e.g., .go, .py, .js
	ChunkConfig    ChunkerConfig
}

// Indexer indexes code files
type Indexer struct {
	config      IndexerConfig
	chunker     Chunker
	embedder    Embedder
	store       VectorStore
	indexedPath string // Path to store indexed file hashes
}

// IndexStats holds indexing statistics
type IndexStats struct {
	TotalFiles   int
	IndexedFiles int
	TotalChunks  int
	SkippedFiles int
	ErrorFiles   int
}

// NewIndexer creates a new indexer
func NewIndexer(config IndexerConfig, chunker Chunker, embedder Embedder, store VectorStore) *Indexer {
	if len(config.ExcludeDirs) == 0 {
		config.ExcludeDirs = []string{".git", "node_modules", "vendor", "__pycache__", ".env", "dist", "build"}
	}
	if len(config.CodeExtensions) == 0 {
		config.CodeExtensions = []string{
			".go", ".py", ".js", ".ts", ".tsx", ".jsx",
			".java", ".rs", ".c", ".cpp", ".h", ".hpp",
			".rb", ".php", ".cs", ".swift", ".kt", ".scala",
			".md", ".txt", ".yaml", ".yml", ".json", ".toml",
		}
	}

	return &Indexer{
		config:   config,
		chunker:  chunker,
		embedder: embedder,
		store:    store,
	}
}

// Index indexes all code files in the repository
func (idx *Indexer) Index(ctx context.Context) (*IndexStats, error) {
	stats := &IndexStats{}

	// Walk repository and collect files
	files, err := idx.collectFiles()
	if err != nil {
		return nil, fmt.Errorf("collect files: %w", err)
	}

	stats.TotalFiles = len(files)

	// Process files in parallel with limited concurrency
	const maxConcurrency = 8
	sem := make(chan struct{}, maxConcurrency)
	var wg sync.WaitGroup
	var mu sync.Mutex

	for _, file := range files {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		wg.Add(1)
		sem <- struct{}{} // Acquire semaphore

		go func(filePath string) {
			defer wg.Done()
			defer func() { <-sem }() // Release semaphore

			err := idx.indexFile(ctx, filePath)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				stats.ErrorFiles++
			} else {
				stats.IndexedFiles++
			}
		}(file)
	}

	wg.Wait()

	// Get final chunk count
	stats.TotalChunks = idx.store.Count()

	return stats, nil
}

// indexFile indexes a single file
func (idx *Indexer) indexFile(ctx context.Context, filePath string) error {
	// Read file
	content, err := os.ReadFile(filePath)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	// Detect language
	ext := filepath.Ext(filePath)
	language := strings.TrimPrefix(ext, ".")

	// Create metadata
	metadata := map[string]string{
		"file_path": filePath,
		"language":  language,
	}

	// Chunk text
	chunks, err := idx.chunker.ChunkText(string(content), metadata)
	if err != nil {
		return fmt.Errorf("chunk text: %w", err)
	}

	// Generate embeddings in batches
	const batchSize = 50
	for i := 0; i < len(chunks); i += batchSize {
		end := i + batchSize
		if end > len(chunks) {
			end = len(chunks)
		}

		batch := chunks[i:end]
		texts := make([]string, len(batch))
		for j, chunk := range batch {
			texts[j] = chunk.Text
		}

		// Generate embeddings
		embeddings, err := idx.embedder.EmbedBatch(ctx, texts)
		if err != nil {
			return fmt.Errorf("embed batch: %w", err)
		}

		// Store documents
		docs := make([]Document, len(batch))
		for j, chunk := range batch {
			// Create unique ID
			docID := fmt.Sprintf("%s:%d-%d", filePath, chunk.StartLine, chunk.EndLine)
			hashID := fmt.Sprintf("%x", sha256.Sum256([]byte(docID)))[:16]

			// Add line number metadata
			chunk.Metadata["start_line"] = fmt.Sprintf("%d", chunk.StartLine)
			chunk.Metadata["end_line"] = fmt.Sprintf("%d", chunk.EndLine)

			docs[j] = Document{
				ID:        hashID,
				Content:   chunk.Text,
				Embedding: embeddings[j],
				Metadata:  chunk.Metadata,
			}
		}

		if err := idx.store.Add(ctx, docs); err != nil {
			return fmt.Errorf("store documents: %w", err)
		}
	}

	return nil
}

// collectFiles walks the repository and collects code files
func (idx *Indexer) collectFiles() ([]string, error) {
	var files []string

	err := filepath.WalkDir(idx.config.RepoPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip excluded directories
		if d.IsDir() {
			for _, excluded := range idx.config.ExcludeDirs {
				if d.Name() == excluded {
					return fs.SkipDir
				}
			}
			return nil
		}

		// Check if file has code extension
		ext := filepath.Ext(path)
		for _, codeExt := range idx.config.CodeExtensions {
			if ext == codeExt {
				files = append(files, path)
				break
			}
		}

		return nil
	})

	return files, err
}

// UpdateIncremental updates index for changed files only
func (idx *Indexer) UpdateIncremental(ctx context.Context, changedFiles []string) (*IndexStats, error) {
	stats := &IndexStats{
		TotalFiles: len(changedFiles),
	}

	// Process each changed file
	for _, file := range changedFiles {
		select {
		case <-ctx.Done():
			return stats, ctx.Err()
		default:
		}

		// Delete old chunks for this file
		// First, we need to find all document IDs for this file
		// For now, we'll just re-index (optimization: track doc IDs per file)

		err := idx.indexFile(ctx, file)
		if err != nil {
			stats.ErrorFiles++
		} else {
			stats.IndexedFiles++
		}
	}

	stats.TotalChunks = idx.store.Count()
	return stats, nil
}
