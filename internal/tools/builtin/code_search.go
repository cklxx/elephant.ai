package builtin

import (
	"alex/internal/agent/ports"
	tools "alex/internal/agent/ports/tools"
	"alex/internal/config"
	"alex/internal/rag"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// codeSearch implements semantic code search using RAG
type codeSearch struct {
	retriever *rag.Retriever
	mu        sync.RWMutex
}

// NewCodeSearch creates a new code search tool
func NewCodeSearch() tools.ToolExecutor {
	return &codeSearch{}
}

// Execute performs semantic code search
func (t *codeSearch) Execute(ctx context.Context, call ports.ToolCall) (*ports.ToolResult, error) {
	query, ok := call.Arguments["query"].(string)
	if !ok || query == "" {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("missing or invalid 'query' parameter"),
		}, nil
	}

	// Get optional repo path (default to current directory)
	repoPath, ok := call.Arguments["repo_path"].(string)
	if !ok || repoPath == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return &ports.ToolResult{
				CallID: call.ID,
				Error:  fmt.Errorf("get current directory: %w", err),
			}, nil
		}
		repoPath = cwd
	}

	// Initialize retriever if needed
	retriever, err := t.getOrCreateRetriever(ctx, repoPath)
	if err != nil {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("initialize retriever: %w", err),
		}, nil
	}

	// Search
	results, err := retriever.Search(ctx, query)
	if err != nil {
		return &ports.ToolResult{
			CallID: call.ID,
			Error:  fmt.Errorf("search failed: %w", err),
		}, nil
	}

	// Format results
	content := retriever.FormatResultsCompact(results)

	return &ports.ToolResult{
		CallID:  call.ID,
		Content: content,
		Metadata: map[string]any{
			"result_count": len(results),
			"repo_path":    repoPath,
		},
	}, nil
}

// getOrCreateRetriever gets or creates a retriever for the given repository
func (t *codeSearch) getOrCreateRetriever(ctx context.Context, repoPath string) (*rag.Retriever, error) {
	t.mu.Lock()
	defer t.mu.Unlock()

	// Return existing retriever if already initialized
	if t.retriever != nil {
		return t.retriever, nil
	}

	cfg, _, err := config.Load()
	if err != nil {
		return nil, fmt.Errorf("load configuration: %w", err)
	}
	apiKey := cfg.APIKey
	if apiKey == "" {
		return nil, fmt.Errorf("no API key configured; set runtime.api_key in ~/.alex/config.yaml or reference ${OPENAI_API_KEY}")
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
		return nil, fmt.Errorf("create embedder: %w", err)
	}

	// Get index path
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("get home directory: %w", err)
	}

	indexPath := filepath.Join(homeDir, ".alex", "indices", getRepoHash(repoPath))

	// Create vector store
	store, err := rag.NewVectorStore(rag.StoreConfig{
		PersistPath: indexPath,
		Collection:  "code",
	}, embedder)
	if err != nil {
		return nil, fmt.Errorf("create vector store: %w", err)
	}

	// Check if index exists
	if store.Count() == 0 {
		return nil, fmt.Errorf("index not found for repository: %s. Please run 'alex index' first", repoPath)
	}

	// Create retriever
	retriever := rag.NewRetriever(rag.RetrieverConfig{
		TopK:          5,
		MinSimilarity: 0.7,
	}, embedder, store)

	t.retriever = retriever
	return retriever, nil
}

// getRepoHash generates a unique hash for the repository path
func getRepoHash(repoPath string) string {
	// Simple hash: use absolute path basename
	absPath, err := filepath.Abs(repoPath)
	if err != nil {
		absPath = repoPath
	}
	return filepath.Base(absPath)
}

// Definition returns the tool definition
func (t *codeSearch) Definition() ports.ToolDefinition {
	return ports.ToolDefinition{
		Name:        "code_search",
		Description: "Search codebase for relevant code using natural language or code queries. Returns top 5 relevant code chunks with file paths, line numbers, and similarity scores.",
		Parameters: ports.ParameterSchema{
			Type: "object",
			Properties: map[string]ports.Property{
				"query": {
					Type:        "string",
					Description: "Natural language or code query (e.g., 'authentication logic', 'function that handles user login')",
				},
				"repo_path": {
					Type:        "string",
					Description: "Repository path (optional, defaults to current directory)",
				},
			},
			Required: []string{"query"},
		},
	}
}

// Metadata returns tool metadata
func (t *codeSearch) Metadata() ports.ToolMetadata {
	return ports.ToolMetadata{
		Name:     "code_search",
		Version:  "1.0.0",
		Category: "search",
		Tags:     []string{"rag", "semantic-search", "code"},
	}
}
