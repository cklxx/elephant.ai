# ALEX Optimization Roadmap - 20+ Tasks

> **Based on**: Comprehensive research of Claude Code, production agents, MCP, and RAG
> **Goal**: Transform ALEX into a production-grade code agent
> **Timeline**: 8-12 weeks (phased approach)

## Executive Summary

This roadmap presents **24 concrete optimization tasks** organized into 4 phases, each with clear objectives, acceptance criteria, and implementation guidance. All tasks are derived from production best practices and validated against industry benchmarks.

---

## Phase 1: Safety & Reliability (Weeks 1-2)

**Objective**: Prevent catastrophic failures, enable safe autonomous operation

### Task 1.1: Human-in-the-Loop Approval Gates

**Priority**: CRITICAL
**Effort**: 3 days
**Dependencies**: None

**Description**: Implement approval mechanism for destructive operations (file deletion, code execution, git operations)

**Acceptance Criteria**:
- [ ] Decorator `@require_approval` for tool functions
- [ ] Interactive prompt with diff preview before destructive operations
- [ ] Configuration: `~/.alex/config.yaml` with tool allowlist/blocklist
- [ ] Skip approval mode: `--auto-approve` flag for CI/CD
- [ ] Test: Verify approval prompt appears for `file_write`, `bash` with destructive commands

**Implementation**:
```go
// internal/tools/approval/gate.go
type ApprovalGate interface {
    RequestApproval(operation string, details string) (bool, error)
}

// Decorate tools with approval requirement
func WithApproval(tool Tool, gate ApprovalGate) Tool {
    return &ApprovalTool{wrapped: tool, gate: gate}
}
```

**Files to modify**:
- `internal/tools/builtin/file_write.go`
- `internal/tools/builtin/bash.go`
- `internal/agent/app/coordinator.go` (inject approval gate)

---

### Task 1.2: Exponential Backoff with Jitter for Rate Limits

**Priority**: HIGH
**Effort**: 2 days
**Dependencies**: None

**Description**: Handle API rate limits (HTTP 429) with exponential backoff and jitter to prevent cascading failures

**Acceptance Criteria**:
- [ ] Detect HTTP 429 status codes from all LLM providers
- [ ] Exponential backoff: 2s → 4s → 8s → 16s (max 5 retries)
- [ ] Random jitter: ±25% to distribute load
- [ ] Return descriptive error to LLM: "Rate limited, retrying in Xs"
- [ ] Test: Mock 429 response, verify retry behavior

**Implementation**:
```go
// internal/llm/retry.go
type RetryConfig struct {
    MaxRetries    int
    BaseDelay     time.Duration
    MaxDelay      time.Duration
    JitterPercent float64
}

func WithExponentialBackoff(client LLMClient, config RetryConfig) LLMClient {
    return &RetryClient{wrapped: client, config: config}
}
```

**Files to modify**:
- `internal/llm/openai_client.go`
- `internal/llm/deepseek_client.go`
- `internal/llm/factory.go` (wrap all clients)

---

### Task 1.3: Enhanced Error Recovery with Retry Logic

**Priority**: HIGH
**Effort**: 3 days
**Dependencies**: Task 1.2

**Description**: Graceful error handling with retry logic for transient failures

**Acceptance Criteria**:
- [ ] Distinguish transient (network timeout) vs permanent (invalid API key) errors
- [ ] Retry transient errors up to 3 times
- [ ] Return actionable error messages to LLM (not stack traces)
- [ ] Log all errors with context (tool name, parameters, attempt number)
- [ ] Test: Simulate network timeout, verify retry behavior

**Implementation**:
```go
// internal/tools/executor/retry.go
func (e *Executor) ExecuteWithRetry(ctx context.Context, call ToolCall) (string, error) {
    for attempt := 0; attempt < maxRetries; attempt++ {
        result, err := e.Execute(ctx, call)
        if err == nil {
            return result, nil
        }
        if !isRetriable(err) {
            return "", formatErrorForLLM(err)
        }
        time.Sleep(retryDelay(attempt))
    }
    return "", errors.New("max retries exceeded")
}
```

**Files to modify**:
- `internal/tools/executor.go`
- All tool implementations (return structured errors)

---

### Task 1.4: Circuit Breaker Pattern for Tool Dependencies

**Priority**: MEDIUM
**Effort**: 2 days
**Dependencies**: Task 1.3

**Description**: Prevent cascading failures when external tools/APIs are down

**Acceptance Criteria**:
- [ ] Circuit breaker for external APIs (web search, web fetch)
- [ ] States: Closed (normal) → Open (failing) → Half-Open (testing)
- [ ] Open circuit after 5 consecutive failures
- [ ] Half-open after 30s, test with single request
- [ ] Return cached/degraded results when circuit is open
- [ ] Test: Simulate external API failure, verify circuit opens

**Implementation**:
```go
// internal/tools/circuit/breaker.go
type CircuitBreaker struct {
    state         State // Closed, Open, HalfOpen
    failureCount  int
    successCount  int
    lastAttempt   time.Time
}
```

**Files to modify**:
- `internal/tools/builtin/web_search.go`
- `internal/tools/builtin/web_fetch.go`

---

### Task 1.5: Diff Preview Before File Edits

**Priority**: HIGH
**Effort**: 3 days
**Dependencies**: None

**Description**: Show unified diff before applying file changes (prevent accidental overwrites)

**Acceptance Criteria**:
- [ ] Generate unified diff before applying edits
- [ ] Display diff in TUI with syntax highlighting
- [ ] Require approval (unless `--auto-approve`)
- [ ] Support rollback: `alex undo` command
- [ ] Test: Edit file, verify diff displayed, apply, verify file changed

**Implementation**:
```go
// internal/tools/builtin/file_edit.go
import "github.com/sergi/go-diff/diffmatchpatch"

func (t *FileEdit) Execute(ctx context.Context, call ToolCall) (string, error) {
    dmp := diffmatchpatch.New()
    diffs := dmp.DiffMain(oldContent, newContent, false)
    unifiedDiff := dmp.DiffPrettyText(diffs)

    // Request approval with diff
    approved, _ := approvalGate.RequestApproval("file_edit", unifiedDiff)
    if !approved {
        return "", errors.New("edit cancelled by user")
    }

    // Apply changes and backup
    backup(filePath, oldContent)
    writeFile(filePath, newContent)
}
```

**Files to modify**:
- `internal/tools/builtin/file_edit.go`
- `cmd/alex/cli.go` (add `undo` command)

---

## Phase 2: Developer Experience (Weeks 3-4)

**Objective**: Improve observability, cost control, and workflow integration

### Task 2.1: Cost Tracking & Token Usage Analytics

**Priority**: HIGH
**Effort**: 4 days
**Dependencies**: None

**Description**: Track input/output tokens, costs across models/sessions

**Acceptance Criteria**:
- [ ] Track tokens per request (input/output separately)
- [ ] Calculate costs based on model pricing ($0.003/1K input, $0.015/1K output for GPT-4)
- [ ] Aggregate by session, day, month
- [ ] CLI command: `alex cost [--session SESSION_ID] [--day YYYY-MM-DD]`
- [ ] TUI display: Real-time cost counter in status bar
- [ ] Export: CSV/JSON for billing analysis
- [ ] Test: Run session, verify cost calculation matches expected

**Implementation**:
```go
// internal/agent/app/cost_tracker.go
type CostTracker struct {
    db storage.Store
}

type Usage struct {
    SessionID    string
    Model        string
    InputTokens  int
    OutputTokens int
    Cost         float64
    Timestamp    time.Time
}

func (ct *CostTracker) RecordUsage(usage Usage) error {
    usage.Cost = calculateCost(usage.Model, usage.InputTokens, usage.OutputTokens)
    return ct.db.Save(usage)
}
```

**Files to modify**:
- `internal/llm/openai_client.go` (extract token counts from response)
- `internal/agent/app/coordinator.go` (inject cost tracker)
- `cmd/alex/cli.go` (add `cost` command)
- `cmd/alex/tui_modern.go` (display cost in status bar)

---

### Task 2.2: Git Integration - Auto-Commit with Smart Messages

**Priority**: HIGH
**Effort**: 5 days
**Dependencies**: None

**Description**: Automatically commit changes with AI-generated commit messages

**Acceptance Criteria**:
- [ ] Detect modified files via `git status`
- [ ] Generate commit message from `git diff` using LLM
- [ ] Follow Conventional Commits format (`feat:`, `fix:`, `refactor:`)
- [ ] Add footer: "🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>"
- [ ] CLI command: `alex commit [--message "custom message"]`
- [ ] Interactive approval before commit
- [ ] Test: Modify files, run `alex commit`, verify commit created

**Implementation**:
```go
// internal/tools/builtin/git_commit.go
func (t *GitCommit) Execute(ctx context.Context, call ToolCall) (string, error) {
    // Get diff
    diff := runCommand("git", "diff", "--staged")

    // Generate message via LLM
    prompt := fmt.Sprintf("Generate a conventional commit message for:\n%s", diff)
    message := llm.Generate(prompt)

    // Append footer
    message += "\n\n🤖 Generated with ALEX\nCo-Authored-By: ALEX <noreply@alex.com>"

    // Commit
    runCommand("git", "commit", "-m", message)
    return message, nil
}
```

**Files to modify**:
- `internal/tools/builtin/git_commit.go` (new file)
- `internal/tools/registry.go` (register tool)
- `cmd/alex/cli.go` (add `commit` command)

---

### Task 2.3: Git Integration - Pull Request Creation

**Priority**: MEDIUM
**Effort**: 4 days
**Dependencies**: Task 2.2

**Description**: Create GitHub PRs with AI-generated descriptions

**Acceptance Criteria**:
- [ ] Detect current branch and base branch
- [ ] Generate PR title and description from commit history
- [ ] Format: "## Summary\n...\n## Test Plan\n..."
- [ ] Use `gh` CLI for PR creation
- [ ] CLI command: `alex pr [--base main] [--title "..."]`
- [ ] Return PR URL
- [ ] Test: Create feature branch, make commits, run `alex pr`, verify PR created

**Implementation**:
```go
// internal/tools/builtin/git_pr.go
func (t *GitPR) Execute(ctx context.Context, call ToolCall) (string, error) {
    // Get commits since base branch
    commits := runCommand("git", "log", "main..HEAD", "--oneline")
    diff := runCommand("git", "diff", "main...HEAD")

    // Generate PR description
    prompt := fmt.Sprintf("Generate PR description from commits:\n%s\n\nDiff:\n%s", commits, diff)
    description := llm.Generate(prompt)

    // Create PR via gh CLI
    title := extractFirstLine(description)
    url := runCommand("gh", "pr", "create", "--title", title, "--body", description)
    return url, nil
}
```

**Files to modify**:
- `internal/tools/builtin/git_pr.go` (new file)
- `internal/tools/registry.go`
- `cmd/alex/cli.go` (add `pr` command)

---

### Task 2.4: Comprehensive Observability (OpenTelemetry)

**Priority**: MEDIUM
**Effort**: 5 days
**Dependencies**: None

**Description**: Instrument ALEX with logs, metrics, and traces

**Acceptance Criteria**:
- [ ] **Logs**: Structured logging with `slog` (JSON format)
- [ ] **Metrics**: Token usage, latency, error rate, cost (Prometheus format)
- [ ] **Traces**: End-to-end request tracing with OpenTelemetry
- [ ] Export to Jaeger (traces), Prometheus (metrics)
- [ ] Dashboard: Grafana with pre-built dashboards
- [ ] Test: Run session, view traces in Jaeger, metrics in Grafana

**Implementation**:
```go
// internal/observability/telemetry.go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/trace"
)

func InstrumentCoordinator(coord *Coordinator) {
    tracer := otel.Tracer("alex")

    coord.SolveTask = func(ctx context.Context, task string) (string, error) {
        ctx, span := tracer.Start(ctx, "SolveTask")
        defer span.End()

        result, err := coord.originalSolveTask(ctx, task)
        if err != nil {
            span.RecordError(err)
        }
        return result, err
    }
}
```

**Files to modify**:
- `internal/observability/telemetry.go` (new file)
- `internal/agent/app/coordinator.go` (add tracing)
- `internal/llm/openai_client.go` (add metrics)
- `cmd/alex/main.go` (initialize telemetry)

---

### Task 2.5: Pre-Commit Hooks Integration

**Priority**: MEDIUM
**Effort**: 3 days
**Dependencies**: None

**Description**: Run linting, formatting, tests before commits

**Acceptance Criteria**:
- [ ] Detect project type (Go, Python, TypeScript)
- [ ] Run appropriate hooks: `gofmt`, `golangci-lint`, `go test` (for Go)
- [ ] Block commit if hooks fail (return error to LLM)
- [ ] CLI command: `alex install-hooks` (setup git hooks)
- [ ] Configuration: `.alex/hooks.yaml` to customize hooks
- [ ] Test: Modify code with linting error, attempt commit, verify blocked

**Implementation**:
```go
// internal/tools/builtin/git_commit.go (enhanced)
func (t *GitCommit) Execute(ctx context.Context, call ToolCall) (string, error) {
    // Run pre-commit hooks
    if err := runHooks("pre-commit"); err != nil {
        return "", fmt.Errorf("pre-commit hooks failed: %w", err)
    }

    // Proceed with commit...
}

// internal/hooks/runner.go
func runHooks(stage string) error {
    config := loadHooksConfig(".alex/hooks.yaml")
    for _, hook := range config.Hooks[stage] {
        if err := runCommand(hook.Command...); err != nil {
            return err
        }
    }
    return nil
}
```

**Files to modify**:
- `internal/hooks/runner.go` (new file)
- `internal/tools/builtin/git_commit.go` (integrate hooks)
- `cmd/alex/cli.go` (add `install-hooks` command)

---

### Task 2.6: Session Replay & Debugging

**Priority**: LOW
**Effort**: 3 days
**Dependencies**: Task 2.4

**Description**: Reproduce and debug past sessions

**Acceptance Criteria**:
- [ ] CLI command: `alex replay SESSION_ID [--step]`
- [ ] Step-by-step replay of tool calls and LLM responses
- [ ] Pause/resume: `--step` flag for interactive stepping
- [ ] Show token usage, costs, latency per step
- [ ] Test: Run session, replay, verify exact reproduction

**Implementation**:
```go
// cmd/alex/replay.go
func replaySession(sessionID string, stepMode bool) error {
    session := loadSession(sessionID)

    for i, turn := range session.Turns {
        fmt.Printf("Turn %d: %s\n", i, turn.UserMessage)
        fmt.Printf("Tool calls: %v\n", turn.ToolCalls)

        if stepMode {
            fmt.Print("Press Enter to continue...")
            bufio.NewReader(os.Stdin).ReadBytes('\n')
        }
    }
}
```

**Files to modify**:
- `cmd/alex/replay.go` (new file)
- `cmd/alex/cli.go` (add `replay` command)

---

## Phase 3: Performance Optimization (Weeks 5-6)

**Objective**: Reduce latency, costs, and token usage

### Task 3.1: Context Compression with Auto-Compaction

**Priority**: HIGH
**Effort**: 4 days
**Dependencies**: None

**Description**: Automatically compress conversation history when approaching context limit

**Acceptance Criteria**:
- [ ] Monitor context window usage (% of max tokens)
- [ ] Trigger compaction at 70% threshold
- [ ] Use LLM to summarize conversation into key facts
- [ ] Preserve recent turns (last 5) and summaries
- [ ] CLI command: `/compact` for manual compaction
- [ ] Test: Long conversation, verify auto-compaction triggers, context reduced

**Implementation**:
```go
// internal/agent/app/context_manager.go
type ContextManager struct {
    maxTokens     int
    compactThreshold float64 // 0.7 = 70%
}

func (cm *ContextManager) ShouldCompact(messages []Message) bool {
    currentTokens := countTokens(messages)
    return float64(currentTokens) / float64(cm.maxTokens) > cm.compactThreshold
}

func (cm *ContextManager) Compact(messages []Message) []Message {
    // Keep recent messages
    recent := messages[len(messages)-5:]

    // Summarize older messages
    older := messages[:len(messages)-5]
    summary := llm.Generate("Summarize conversation:\n" + formatMessages(older))

    return append([]Message{{Role: "system", Content: summary}}, recent...)
}
```

**Files to modify**:
- `internal/agent/app/context_manager.go` (new file)
- `internal/agent/app/coordinator.go` (integrate compaction)
- `cmd/alex/cli.go` (add `/compact` command)

---

### Task 3.2: Prompt Compression with Token Pruning

**Priority**: MEDIUM
**Effort**: 5 days
**Dependencies**: None

**Description**: Reduce prompt size by removing low-attention tokens

**Acceptance Criteria**:
- [ ] Integrate LLMLingua or similar compression library
- [ ] Compress tool outputs >2000 tokens
- [ ] Target: 30-50% token reduction with <5% quality loss
- [ ] Configuration: `~/.alex/config.yaml` compression settings
- [ ] Test: Long tool output, verify compression, validate LLM still understands

**Implementation**:
```go
// internal/llm/compression.go
import "github.com/microsoft/LLMLingua-go" // hypothetical Go binding

type Compressor interface {
    Compress(text string, targetRatio float64) string
}

func (c *LLMLinguaCompressor) Compress(text string, targetRatio float64) string {
    if len(text) < 2000 {
        return text // Don't compress short text
    }

    compressed := llmlingua.Compress(text, ratio=targetRatio)
    return compressed
}
```

**Files to modify**:
- `internal/llm/compression.go` (new file)
- `internal/tools/formatter.go` (compress tool outputs)

**Note**: Requires Go bindings for LLMLingua or implementing basic compression heuristics

---

### Task 3.3: Intelligent Token Budget Management

**Priority**: MEDIUM
**Effort**: 4 days
**Dependencies**: Task 2.1

**Description**: Route queries to appropriate models based on complexity and budget

**Acceptance Criteria**:
- [ ] Classify queries: simple (classification) vs complex (reasoning)
- [ ] Route simple → cheap model (gpt-4o-mini), complex → expensive (gpt-4)
- [ ] Budget limits: `~/.alex/config.yaml` max daily/monthly spend
- [ ] Block requests if budget exceeded (return error to user)
- [ ] CLI command: `alex budget status` (show remaining budget)
- [ ] Test: Set low budget, exceed it, verify blocking

**Implementation**:
```go
// internal/llm/router.go
type QueryClassifier interface {
    Classify(query string) Complexity // Simple, Medium, Complex
}

type ModelRouter struct {
    classifier QueryClassifier
    budgetTracker *BudgetTracker
}

func (r *ModelRouter) SelectModel(query string) (string, error) {
    complexity := r.classifier.Classify(query)

    if !r.budgetTracker.CanAfford(complexity) {
        return "", errors.New("budget exceeded")
    }

    switch complexity {
    case Simple:
        return "gpt-4o-mini", nil
    case Complex:
        return "gpt-4", nil
    }
}
```

**Files to modify**:
- `internal/llm/router.go` (new file)
- `internal/agent/app/coordinator.go` (use router)
- `cmd/alex/cli.go` (add `budget` command)

---

### Task 3.4: Expand Semantic Caching

**Priority**: MEDIUM
**Effort**: 3 days
**Dependencies**: None

**Description**: Cache LLM responses for similar queries (not just exact matches)

**Acceptance Criteria**:
- [ ] Embed queries and cache embeddings
- [ ] Check similarity (cosine distance < 0.1) before LLM call
- [ ] Return cached response if similar query found
- [ ] TTL: 1 hour for cache entries
- [ ] Hit rate monitoring: log cache hits/misses
- [ ] Test: Ask similar questions, verify cache hit

**Implementation**:
```go
// internal/llm/semantic_cache.go
type SemanticCache struct {
    embedder EmbeddingClient
    store    map[string]CachedResponse // embedding_id -> response
}

func (sc *SemanticCache) Get(query string) (string, bool) {
    queryEmbed := sc.embedder.Embed(query)

    // Find similar cached query
    for cachedEmbed, response := range sc.store {
        similarity := cosineSimilarity(queryEmbed, cachedEmbed)
        if similarity > 0.9 { // 90% similar
            return response.Text, true
        }
    }
    return "", false
}
```

**Files to modify**:
- `internal/llm/semantic_cache.go` (new file)
- `internal/llm/openai_client.go` (check cache before API call)

---

### Task 3.5: Batch Tool Execution Optimization

**Priority**: LOW
**Effort**: 3 days
**Dependencies**: None

**Description**: Execute independent tool calls in parallel (already partially done, enhance)

**Acceptance Criteria**:
- [ ] Detect independent tool calls (no data dependencies)
- [ ] Execute in parallel using goroutines
- [ ] Aggregate results before returning to LLM
- [ ] Limit concurrency: max 5 parallel tools
- [ ] Test: LLM calls 3 independent tools, verify parallel execution, measure speedup

**Implementation**:
```go
// internal/tools/executor.go (enhanced)
func (e *Executor) ExecuteBatch(ctx context.Context, calls []ToolCall) ([]string, error) {
    results := make([]string, len(calls))
    var wg sync.WaitGroup
    semaphore := make(chan struct{}, 5) // Max 5 concurrent

    for i, call := range calls {
        wg.Add(1)
        go func(idx int, c ToolCall) {
            defer wg.Done()
            semaphore <- struct{}{}
            defer func() { <-semaphore }()

            results[idx], _ = e.Execute(ctx, c)
        }(i, call)
    }

    wg.Wait()
    return results, nil
}
```

**Files to modify**:
- `internal/tools/executor.go` (enhance batching)
- `internal/agent/domain/react_engine.go` (detect independent calls)

---

## Phase 4: Advanced Context & MCP (Weeks 7-8)

**Objective**: Advanced codebase understanding and extensibility

### Task 4.1: RAG Phase 1 - Basic Code Embeddings

**Priority**: HIGH
**Effort**: 5 days
**Dependencies**: None

**Description**: Implement basic RAG with embeddings for code search

**Acceptance Criteria**:
- [ ] Integrate `chromem-go` (embedded vector DB)
- [ ] Use OpenAI `text-embedding-3-small` API
- [ ] Chunk code files (recursive text splitter, 512 tokens)
- [ ] Index repository: `alex index [--repo PATH]`
- [ ] Search: `alex search "authentication logic"` returns top 5 files
- [ ] Test: Index test repo, search, verify relevant results

**Implementation**:
```go
// internal/rag/indexer.go
import "github.com/philippgille/chromem-go"

type CodeIndexer struct {
    db       *chromem.DB
    embedder EmbeddingClient
}

func (idx *CodeIndexer) IndexRepository(repoPath string) error {
    collection := idx.db.GetOrCreateCollection("codebase", nil, nil)

    files := walkFiles(repoPath, filterCodeFiles)
    for _, file := range files {
        chunks := chunkFile(file.Content, 512)
        for i, chunk := range chunks {
            collection.Add(ctx, chromem.Document{
                ID:      fmt.Sprintf("%s:%d", file.Path, i),
                Content: chunk,
                Metadata: map[string]string{
                    "file_path": file.Path,
                    "language":  detectLanguage(file.Path),
                },
            })
        }
    }
}
```

**Files to modify**:
- `internal/rag/indexer.go` (new file)
- `internal/rag/retriever.go` (new file)
- `cmd/alex/cli.go` (add `index` and `search` commands)

---

### Task 4.2: RAG Phase 2 - AST-Based Chunking with Tree-sitter

**Priority**: MEDIUM
**Effort**: 5 days
**Dependencies**: Task 4.1

**Description**: Improve chunking using AST for better semantic boundaries

**Acceptance Criteria**:
- [ ] Integrate Tree-sitter (`github.com/tree-sitter/go-tree-sitter`)
- [ ] Parse Go/Python/TypeScript files into AST
- [ ] Chunk by semantic boundaries (functions, classes, methods)
- [ ] Add contextual headers (file path, parent class)
- [ ] Measure retrieval improvement: >20% better precision@5
- [ ] Test: Index with AST chunking, compare search quality vs recursive splitter

**Implementation**:
```go
// internal/rag/ast_chunker.go
import tree_sitter "github.com/tree-sitter/go-tree-sitter"

func chunkByAST(content string, language string) []Chunk {
    parser := tree_sitter.NewParser()
    parser.SetLanguage(getLanguage(language)) // Go, Python, etc.

    tree := parser.Parse([]byte(content), nil)
    root := tree.RootNode()

    chunks := []Chunk{}
    traverseAST(root, func(node *tree_sitter.Node) {
        if isChunkBoundary(node.Type()) { // function, class
            chunks = append(chunks, Chunk{
                Code: node.Content(),
                Type: node.Type(),
            })
        }
    })
    return chunks
}
```

**Files to modify**:
- `internal/rag/ast_chunker.go` (new file)
- `internal/rag/indexer.go` (use AST chunker)

---

### Task 4.3: RAG Phase 3 - Hybrid Search with Qdrant

**Priority**: MEDIUM
**Effort**: 4 days
**Dependencies**: Task 4.2

**Description**: Upgrade to Qdrant for hybrid vector + keyword search

**Acceptance Criteria**:
- [ ] Deploy Qdrant (Docker: `docker run -p 6333:6333 qdrant/qdrant`)
- [ ] Migrate from chromem-go to Qdrant Go SDK
- [ ] Implement hybrid search (vector + BM25)
- [ ] Reciprocal Rank Fusion (RRF) to merge results
- [ ] Measure improvement: >15% better recall@10
- [ ] Test: Compare search quality vs chromem-go

**Implementation**:
```go
// internal/rag/retriever.go (enhanced)
import "github.com/qdrant/go-client/qdrant"

func (r *Retriever) HybridSearch(query string, topK int) ([]Chunk, error) {
    // Parallel vector + keyword search
    var vectorResults, keywordResults []qdrant.ScoredPoint
    var wg sync.WaitGroup

    wg.Add(2)
    go func() {
        defer wg.Done()
        embedding := r.embedder.Embed(query)
        vectorResults, _ = r.qdrant.Query(ctx, &qdrant.QueryPoints{
            CollectionName: "codebase",
            Query:          qdrant.NewQuery(embedding...),
            Limit:          topK * 5,
        })
    }()
    go func() {
        defer wg.Done()
        keywordResults, _ = r.qdrant.QueryBM25(ctx, query, topK*5)
    }()
    wg.Wait()

    // Fusion
    fused := reciprocalRankFusion(vectorResults, keywordResults)
    return fused[:topK], nil
}
```

**Files to modify**:
- `internal/rag/retriever.go` (add hybrid search)
- `internal/rag/indexer.go` (use Qdrant)
- `docker-compose.yml` (add Qdrant service)

---

### Task 4.4: Incremental RAG Indexing with File Watching

**Priority**: MEDIUM
**Effort**: 4 days
**Dependencies**: Task 4.3

**Description**: Auto-update index when files change

**Acceptance Criteria**:
- [ ] Use `fsnotify` to watch repository files
- [ ] Re-index only changed files (not entire repo)
- [ ] Debounce changes (wait 2s after last change)
- [ ] CLI: `alex watch [--repo PATH]` (background process)
- [ ] Test: Modify file, verify index updated within 5s

**Implementation**:
```go
// internal/rag/watcher.go
import "github.com/fsnotify/fsnotify"

func (w *Watcher) Watch(repoPath string) error {
    watcher, _ := fsnotify.NewWatcher()
    watcher.Add(repoPath)

    changedFiles := make(map[string]bool)
    timer := time.NewTimer(2 * time.Second)

    for {
        select {
        case event := <-watcher.Events:
            if event.Op&fsnotify.Write == fsnotify.Write {
                changedFiles[event.Name] = true
                timer.Reset(2 * time.Second)
            }
        case <-timer.C:
            for file := range changedFiles {
                w.indexer.ReindexFile(file)
            }
            changedFiles = make(map[string]bool)
        }
    }
}
```

**Files to modify**:
- `internal/rag/watcher.go` (new file)
- `cmd/alex/cli.go` (add `watch` command)

---

### Task 4.5: MCP Server Support - Stdio Servers

**Priority**: MEDIUM
**Effort**: 6 days
**Dependencies**: None

**Description**: Implement MCP protocol client for stdio servers

**Acceptance Criteria**:
- [ ] JSON-RPC 2.0 client for MCP stdio transport
- [ ] Discovery: Read `.mcp.json` configuration
- [ ] Initialize server, list tools/resources/prompts
- [ ] Expose MCP tools as ALEX tools (transparent integration)
- [ ] CLI: `alex mcp add <name> <command> [args...]`
- [ ] Test: Add sample MCP server (e.g., filesystem), verify tools work

**Implementation**:
```go
// internal/mcp/client.go
type MCPClient struct {
    process *exec.Cmd
    stdin   io.Writer
    stdout  io.Reader
}

func (c *MCPClient) Initialize() error {
    // Send initialize request
    request := jsonrpc.Request{
        Method: "initialize",
        Params: map[string]interface{}{
            "protocolVersion": "2024-11-05",
            "clientInfo": map[string]string{
                "name":    "alex",
                "version": "0.1.0",
            },
        },
    }
    return c.call(request)
}

func (c *MCPClient) ListTools() ([]Tool, error) {
    response := c.call(jsonrpc.Request{Method: "tools/list"})
    return parseTools(response), nil
}
```

**Files to modify**:
- `internal/mcp/client.go` (new file)
- `internal/mcp/config.go` (parse `.mcp.json`)
- `internal/tools/registry.go` (register MCP tools dynamically)
- `cmd/alex/cli.go` (add `mcp` commands)

**Configuration format** (`.mcp.json`):
```json
{
  "mcpServers": {
    "filesystem": {
      "command": "mcp-server-filesystem",
      "args": ["/workspace"]
    }
  }
}
```

---

### Task 4.6: Code Graph Analysis for Dependency Tracking

**Priority**: LOW
**Effort**: 7 days
**Dependencies**: Task 4.2

**Description**: Build call graph and dependency graph for advanced code understanding

**Acceptance Criteria**:
- [ ] Parse AST and extract function calls, imports
- [ ] Build directed graph: nodes=functions, edges=calls
- [ ] Query: "What calls function X?" → list callers
- [ ] Query: "What does function Y depend on?" → list callees
- [ ] Integrate with RAG: expand search to related functions
- [ ] Test: Index test repo, query call graph, verify accuracy

**Implementation**:
```go
// internal/rag/graph.go
type CodeGraph struct {
    nodes map[string]*Node // function name -> Node
    edges map[string][]string // caller -> []callees
}

func (g *CodeGraph) BuildFromAST(tree *tree_sitter.Tree) {
    // Traverse AST, find function definitions and calls
    traverseAST(tree.RootNode(), func(node *tree_sitter.Node) {
        if node.Type() == "function_definition" {
            funcName := extractFunctionName(node)
            g.nodes[funcName] = &Node{Name: funcName, AST: node}
        }
        if node.Type() == "call_expression" {
            callee := extractCallee(node)
            g.edges[currentFunction] = append(g.edges[currentFunction], callee)
        }
    })
}

func (g *CodeGraph) FindCallers(functionName string) []string {
    callers := []string{}
    for caller, callees := range g.edges {
        for _, callee := range callees {
            if callee == functionName {
                callers = append(callers, caller)
            }
        }
    }
    return callers
}
```

**Files to modify**:
- `internal/rag/graph.go` (new file)
- `internal/rag/indexer.go` (build graph during indexing)
- `internal/rag/retriever.go` (expand search with graph)

---

### Task 4.7: Workspace Intelligence - Multi-File Coordination

**Priority**: LOW
**Effort**: 5 days
**Dependencies**: Task 4.6

**Description**: Understand workspace structure, coordinate changes across files

**Acceptance Criteria**:
- [ ] Detect project type (Go module, Python package, NPM package)
- [ ] Map workspace structure (directories, entry points, test files)
- [ ] When modifying function, suggest related files to update (callers, tests)
- [ ] CLI: `alex workspace analyze` (show structure)
- [ ] Test: Modify function, verify suggestions include test file and callers

**Implementation**:
```go
// internal/workspace/analyzer.go
type WorkspaceAnalyzer struct {
    graph *rag.CodeGraph
}

func (wa *WorkspaceAnalyzer) Analyze(repoPath string) *Workspace {
    projectType := detectProjectType(repoPath) // Go, Python, etc.

    return &Workspace{
        Type:       projectType,
        EntryPoint: findEntryPoint(repoPath, projectType),
        TestDirs:   findTestDirectories(repoPath, projectType),
        Structure:  buildStructure(repoPath),
    }
}

func (wa *WorkspaceAnalyzer) SuggestRelatedFiles(filePath string) []string {
    // Find functions in file
    functions := extractFunctions(filePath)

    // Find callers and tests
    related := []string{}
    for _, fn := range functions {
        callers := wa.graph.FindCallers(fn.Name)
        related = append(related, callers...)

        testFile := findTestFile(filePath)
        if testFile != "" {
            related = append(related, testFile)
        }
    }
    return related
}
```

**Files to modify**:
- `internal/workspace/analyzer.go` (new file)
- `internal/agent/app/coordinator.go` (use workspace analyzer)
- `cmd/alex/cli.go` (add `workspace` command)

---

## Additional High-Value Tasks (Bonus)

### Task 5.1: Extended Thinking Mode

**Priority**: MEDIUM
**Effort**: 4 days
**Dependencies**: None

**Description**: Implement "think", "think hard", "ultrathink" modes like Claude Code

**Acceptance Criteria**:
- [ ] Detect keywords: "think", "think hard", "ultrathink" in user prompt
- [ ] Allocate thinking budget: basic (1K tokens), hard (5K), ultra (30K)
- [ ] Show thinking process in TUI (expandable section)
- [ ] Improve complex reasoning accuracy by 15-30%
- [ ] Test: Ask complex question with "think hard", verify extended reasoning

---

### Task 5.2: Subagent System with Isolated Contexts

**Priority**: LOW
**Effort**: 7 days
**Dependencies**: None

**Description**: Specialized subagents for specific tasks (code review, testing, refactoring)

**Acceptance Criteria**:
- [ ] YAML frontmatter format for subagent definitions (`.alex/agents/`)
- [ ] Isolated context windows per subagent
- [ ] Delegation: `@code-reviewer` mentions in prompts
- [ ] CLI: `alex agent create <name>` (interactive wizard)
- [ ] Test: Create code-reviewer subagent, delegate review task, verify isolation

---

### Task 5.3: Slash Commands System

**Priority**: LOW
**Effort**: 3 days
**Dependencies**: None

**Description**: Custom commands via `.alex/commands/*.md` files

**Acceptance Criteria**:
- [ ] Parse markdown files with YAML frontmatter
- [ ] Support `$ARGUMENTS`, `$1`, `$2` parameters
- [ ] CLI: `/optimize` executes `.alex/commands/optimize.md`
- [ ] List commands: `alex commands list`
- [ ] Test: Create custom command, execute, verify parameter substitution

---

### Task 5.4: GitHub Actions Integration

**Priority**: LOW
**Effort**: 5 days
**Dependencies**: Task 2.2, 2.3

**Description**: Run ALEX in CI/CD for automated code review, PR descriptions

**Acceptance Criteria**:
- [ ] GitHub Action: `cklxx/alex-action@v1`
- [ ] Inputs: `task` (review, test, fix), `files` (changed files)
- [ ] Outputs: Comments on PR with findings
- [ ] Example workflow: `.github/workflows/alex-review.yml`
- [ ] Test: Create PR, trigger action, verify comment posted

---

## Implementation Timeline

| Week | Phase | Tasks | Focus |
|------|-------|-------|-------|
| 1 | Phase 1 | 1.1-1.3 | Approval gates, backoff, error recovery |
| 2 | Phase 1 | 1.4-1.5 | Circuit breakers, diff preview |
| 3 | Phase 2 | 2.1-2.2 | Cost tracking, Git commit |
| 4 | Phase 2 | 2.3-2.5 | Git PR, observability, hooks |
| 5 | Phase 3 | 3.1-3.3 | Context compression, token management |
| 6 | Phase 3 | 3.4-3.5 | Semantic caching, batch optimization |
| 7 | Phase 4 | 4.1-4.3 | RAG basic → AST → hybrid |
| 8 | Phase 4 | 4.4-4.5 | Incremental indexing, MCP |
| 9+ | Bonus | 4.6-5.4 | Graph analysis, extended thinking, subagents |

---

## Success Metrics

### Safety & Reliability
- [ ] Zero destructive operations without approval
- [ ] <1% error rate from retries
- [ ] 95th percentile error recovery time <5s

### Developer Experience
- [ ] Cost tracking accuracy: ±5%
- [ ] Git commit message quality: >80% accepted without edits
- [ ] Observability dashboard provides actionable insights

### Performance
- [ ] 30-50% token reduction from compression
- [ ] 3-5x speedup from semantic caching
- [ ] Context compaction maintains >90% information quality

### Advanced Context
- [ ] RAG retrieval precision@5 >75%
- [ ] AST chunking improves precision by >20% vs baseline
- [ ] MCP integration: 5+ external servers supported

---

## Next Steps

1. **Review and Prioritize**: Stakeholder review of roadmap, adjust priorities
2. **Prototype Phase 1**: Implement Tasks 1.1-1.3 (approval gates, backoff, error recovery)
3. **Validate**: User testing with early adopters
4. **Iterate**: Adjust based on feedback, proceed to Phase 2

**Total Tasks**: 24 core + 4 bonus = **28 optimization tasks**

This roadmap transforms ALEX from a capable research agent into a production-grade code assistant ready for real-world deployment.
