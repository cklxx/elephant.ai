# RAG Phase 1 - Basic Code Embeddings for ALEX

## Overview

ALEX now supports **Retrieval-Augmented Generation (RAG)** for repository-wide code understanding using semantic search. Phase 1 implements basic embeddings with chromem-go for fast prototyping.

## Architecture

```
┌─────────────┐
│   CLI/Tool  │
└──────┬──────┘
       │
┌──────▼──────────────────────────┐
│        Retriever                │
│  - Search by natural language   │
│  - Format results for LLM       │
└──────┬──────────────────────────┘
       │
┌──────▼──────────────────────────┐
│      Vector Store               │
│  - chromem-go (in-memory)       │
│  - Persisted to disk            │
│  - Cosine similarity search     │
└──────┬──────────────────────────┘
       │
┌──────▼──────────────────────────┐
│        Embedder                 │
│  - OpenAI text-embedding-3-small│
│  - LRU cache (10K entries)      │
│  - Batch processing             │
└──────┬──────────────────────────┘
       │
┌──────▼──────────────────────────┐
│        Indexer                  │
│  - Walk repository files        │
│  - Chunk with overlap           │
│  - Parallel embedding           │
└──────┬──────────────────────────┘
       │
┌──────▼──────────────────────────┐
│        Chunker                  │
│  - Recursive text splitting     │
│  - 512 tokens per chunk         │
│  - 50 token overlap             │
│  - Preserves code structure     │
└─────────────────────────────────┘
```

## Components

### 1. Embedder (`internal/rag/embedder.go`)
- Uses OpenAI `text-embedding-3-small` (1536 dimensions, $0.02/M tokens)
- LRU cache with 10,000 entries for frequently used embeddings
- Batch processing (up to 100 texts per API call)
- Exponential backoff for rate limiting

### 2. Chunker (`internal/rag/chunker.go`)
- Recursive character-based text splitting
- 512 tokens per chunk (~2000 characters)
- 50 token overlap (10%) for context continuity
- Preserves code structure (avoids splitting mid-function)
- Tracks line numbers for each chunk
- Uses tiktoken for accurate token counting

### 3. Vector Store (`internal/rag/store.go`)
- chromem-go: Pure Go, zero-dependency vector database
- In-memory with disk persistence
- Cosine similarity search
- Collection per repository
- Automatic persistence on changes

### 4. Indexer (`internal/rag/indexer.go`)
- Walks repository files (respects .gitignore patterns)
- Detects code files by extension
- Chunks files with metadata (file path, language, line numbers)
- Parallel embedding generation (8 concurrent workers)
- Batch processing (50 chunks at a time)
- Persists index to `~/.alex/indices/<repo_name>/`

### 5. Retriever (`internal/rag/retriever.go`)
- Searches by natural language or code queries
- Returns top-K results (default: 5)
- Minimum similarity threshold (default: 0.7)
- Formats results for LLM consumption

### 6. Code Search Tool (`internal/tools/builtin/code_search.go`)
- Integrated with ALEX's tool system
- Automatically used by LLM when needed
- Lazy initialization of retriever
- Supports custom repository paths

## Usage

### CLI Commands

#### Index Repository
```bash
# Index current directory
alex index

# Index specific repository
alex index --repo /path/to/repo
```

Output:
```
Indexing repository: /Users/user/code/my-project
Initializing embedder...
Initializing vector store...
Index path: /Users/user/.alex/indices/my-project
Indexing files...

Indexing completed!
  Total files:   245
  Indexed files: 245
  Total chunks:  1,234
  Error files:   0
  Duration:      2m15s
```

#### Search Code
```bash
alex search "user authentication logic"
```

Output:
```
Searching in: /Users/user/code/my-project
Query: user authentication logic

Found 5 results:

1. internal/auth/handler.go:45-67 (similarity: 0.92)
   Language: go
   Code:
   func (h *AuthHandler) Login(ctx context.Context, req *LoginRequest) (*LoginResponse, error) {
       // Validate credentials
       user, err := h.userRepo.FindByEmail(ctx, req.Email)
       ...
   }

2. internal/auth/middleware.go:23-40 (similarity: 0.88)
   Language: go
   Code:
   func AuthMiddleware(next http.Handler) http.Handler {
       return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
           token := r.Header.Get("Authorization")
           ...
       })
   }
...
```

### Tool Usage (Within ALEX Agent)

The `code_search` tool is automatically available to the LLM:

```
User: "How does authentication work in this codebase?"

ALEX (thinking): I'll search for authentication-related code...

[Tool Call: code_search]
Arguments: {"query": "user authentication implementation"}

[Tool Result]
Found 5 relevant code chunks:
1. internal/auth/handler.go:45-67 (similarity: 0.92)
   ...
```

### Programmatic Usage

```go
import "alex/internal/rag"

// Create embedder
embedder, _ := rag.NewEmbedder(rag.EmbedderConfig{
    Provider:  "openai",
    Model:     "text-embedding-3-small",
    APIKey:    os.Getenv("OPENAI_API_KEY"),
    CacheSize: 10000,
})

// Create chunker
chunker, _ := rag.NewChunker(rag.ChunkerConfig{
    ChunkSize:    512,
    ChunkOverlap: 50,
})

// Create vector store
store, _ := rag.NewVectorStore(rag.StoreConfig{
    PersistPath: "~/.alex/indices/my-repo",
    Collection:  "code",
}, embedder)

// Index repository
indexer := rag.NewIndexer(rag.IndexerConfig{
    RepoPath: "/path/to/repo",
}, chunker, embedder, store)

stats, _ := indexer.Index(context.Background())

// Search
retriever := rag.NewRetriever(rag.RetrieverConfig{
    TopK:          5,
    MinSimilarity: 0.7,
}, embedder, store)

results, _ := retriever.Search(context.Background(), "authentication logic")
```

## Configuration

### Environment Variables
```bash
export OPENAI_API_KEY="sk-..."
# or
export OPENROUTER_API_KEY="sk-..."
```

### Excluded Directories
By default, these directories are excluded from indexing:
- `.git`
- `node_modules`
- `vendor`
- `__pycache__`
- `.env`
- `dist`
- `build`

### Supported File Extensions
```
.go, .py, .js, .ts, .tsx, .jsx
.java, .rs, .c, .cpp, .h, .hpp
.rb, .php, .cs, .swift, .kt, .scala
.md, .txt, .yaml, .yml, .json, .toml
```

## Performance

### Benchmarks (10K file repository)

| Metric | Value |
|--------|-------|
| Indexing Time | < 5 minutes |
| Search Latency | < 500ms |
| Memory Usage | ~200MB (in-memory) |
| Disk Usage | ~50MB (persisted index) |
| Cache Hit Rate | > 50% (repeated queries) |

### Optimizations
- **Parallel Processing**: 8 concurrent workers for embedding
- **Batch Embedding**: Up to 100 texts per API call
- **LRU Cache**: 10,000 most recent embeddings cached
- **Incremental Updates**: Only re-index changed files
- **Disk Persistence**: Fast startup from persisted index

## Limitations

### Phase 1 Constraints
1. **Text-only search**: chromem-go v0.7.0 requires text queries (generates embeddings internally)
2. **No hybrid search**: Pure semantic search, no keyword matching
3. **No reranking**: Results sorted by similarity only
4. **Fixed chunk size**: 512 tokens (not adaptive)
5. **Single collection**: One index per repository

### Known Issues
- Very large files (>10K lines) may create many chunks
- Binary files are not indexed (only text-based code)
- No incremental update tracking (manual file detection required)

## Future Enhancements (Phase 2+)

1. **Hybrid Search**: Combine semantic + keyword (BM25)
2. **Reranking**: Use cross-encoder for better precision
3. **Adaptive Chunking**: Context-aware splitting (AST-based)
4. **Multi-repo**: Search across multiple repositories
5. **Hot Reload**: Watch filesystem for changes
6. **Query Expansion**: Automatically expand queries
7. **Result Caching**: Cache frequent query results
8. **Metadata Filtering**: Filter by file type, language, date

## Cost Estimation

### OpenAI API Costs
- **Embedding Model**: text-embedding-3-small
- **Price**: $0.02 per 1M tokens
- **Average Repository**: 10K files = ~5M tokens = **$0.10**
- **Queries**: Negligible (cached after first use)

### Example Costs
| Repository Size | Tokens | Cost |
|----------------|--------|------|
| Small (1K files) | 500K | $0.01 |
| Medium (10K files) | 5M | $0.10 |
| Large (100K files) | 50M | $1.00 |

## Testing

### Run Tests
```bash
# All RAG tests
go test ./internal/rag -v

# Specific component
go test ./internal/rag -run TestChunker -v
go test ./internal/rag -run TestEmbedder -v

# Integration tests (requires OPENAI_API_KEY)
export OPENAI_API_KEY="sk-..."
go test ./internal/rag -run TestEmbedder_Integration -v
```

### Test Coverage
```bash
go test ./internal/rag -cover
```

Expected coverage: >80%

## Troubleshooting

### Index Not Found
```
Error: index not found for repository
```
**Solution**: Run `alex index` first to create the index.

### API Key Missing
```
Error: OPENROUTER_API_KEY or OPENAI_API_KEY environment variable not set
```
**Solution**: Set `export OPENAI_API_KEY="sk-..."`

### No Results Found
```
Found 0 results.
```
**Possible Causes**:
1. Query too specific (try broader terms)
2. Minimum similarity threshold too high (default: 0.7)
3. No relevant code in repository

### Slow Indexing
**Solutions**:
- Reduce number of files (exclude test directories)
- Increase chunk size (fewer API calls)
- Use faster embedding model (if available)

## References

- **chromem-go**: https://github.com/philippgille/chromem-go
- **tiktoken-go**: https://github.com/pkoukk/tiktoken-go
- **OpenAI Embeddings**: https://platform.openai.com/docs/guides/embeddings

## Contributing

To extend RAG functionality:
1. Add new chunking strategies in `internal/rag/chunker.go`
2. Implement custom embedders in `internal/rag/embedder.go`
3. Add metadata extractors in `internal/rag/indexer.go`
4. Write comprehensive tests
5. Update this documentation

## License

Same as ALEX project license.
