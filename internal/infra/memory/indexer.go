package memory

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"alex/internal/shared/logging"
	"github.com/fsnotify/fsnotify"
)

const defaultIndexDebounce = 1500 * time.Millisecond

// IndexerConfig controls memory indexing behavior.
type IndexerConfig struct {
	DBPath             string
	ChunkTokens        int
	ChunkOverlap       int
	MinScore           float64
	FusionWeightVector float64
	FusionWeightBM25   float64
}

// Indexer maintains a local vector index for Markdown memory.
type Indexer struct {
	rootDir  string
	cfg      IndexerConfig
	embedder EmbeddingProvider
	logger   logging.Logger

	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	timers   map[string]*time.Timer
	stores   map[string]*IndexStore
	started  bool
	stopOnce sync.Once
	stopCh   chan struct{}
}

// NewIndexer constructs a memory indexer.
func NewIndexer(rootDir string, cfg IndexerConfig, embedder EmbeddingProvider, logger logging.Logger) (*Indexer, error) {
	rootDir = strings.TrimSpace(rootDir)
	if rootDir == "" {
		return nil, fmt.Errorf("memory root required for indexer")
	}
	if embedder == nil {
		return nil, fmt.Errorf("embedding provider required for indexer")
	}
	if cfg.ChunkTokens <= 0 {
		cfg.ChunkTokens = chunkTokenSize
	}
	if cfg.ChunkOverlap < 0 {
		cfg.ChunkOverlap = chunkTokenOverlap
	}
	if cfg.MinScore <= 0 {
		cfg.MinScore = defaultSearchMinScore
	}
	if cfg.FusionWeightVector == 0 && cfg.FusionWeightBM25 == 0 {
		cfg.FusionWeightVector = 0.7
		cfg.FusionWeightBM25 = 0.3
	}
	logger = logging.OrNop(logger)

	return &Indexer{
		rootDir:  rootDir,
		cfg:      cfg,
		embedder: embedder,
		logger:   logger,
		timers:   make(map[string]*time.Timer),
		stores:   make(map[string]*IndexStore),
		stopCh:   make(chan struct{}),
	}, nil
}

// Start performs an initial index pass and begins file watching.
func (i *Indexer) Start(ctx context.Context) error {
	if i == nil {
		return fmt.Errorf("indexer is nil")
	}
	i.mu.Lock()
	if i.started {
		i.mu.Unlock()
		return nil
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		i.mu.Unlock()
		return err
	}
	i.watcher = watcher
	i.started = true
	i.mu.Unlock()

	if err := i.indexAll(ctx); err != nil {
		i.logger.Warn("Memory index initial scan failed: %v", err)
	}
	if err := i.addWatchRoots(); err != nil {
		i.logger.Warn("Memory index watcher setup failed: %v", err)
	}

	go i.watchLoop()
	go func() {
		<-ctx.Done()
		_ = i.Drain(context.Background())
	}()

	return nil
}

// Drain stops the watcher and closes stores.
func (i *Indexer) Drain(ctx context.Context) error {
	_ = ctx
	if i == nil {
		return nil
	}
	i.stopOnce.Do(func() {
		close(i.stopCh)
		i.mu.Lock()
		for _, timer := range i.timers {
			timer.Stop()
		}
		i.timers = make(map[string]*time.Timer)
		if i.watcher != nil {
			_ = i.watcher.Close()
			i.watcher = nil
		}
		for key, store := range i.stores {
			_ = store.Close()
			delete(i.stores, key)
		}
		i.started = false
		i.mu.Unlock()
	})
	return nil
}

// Name returns the drainable name.
func (i *Indexer) Name() string {
	return "memory-indexer"
}

// Search performs hybrid (vector + BM25) retrieval.
func (i *Indexer) Search(ctx context.Context, _ string, query string, maxResults int, minScore float64) ([]SearchHit, error) {
	if i == nil {
		return nil, fmt.Errorf("indexer not initialized")
	}
	query = strings.TrimSpace(query)
	if query == "" {
		return nil, fmt.Errorf("query is required")
	}
	if maxResults <= 0 {
		maxResults = defaultSearchMax
	}
	if minScore <= 0 {
		minScore = i.cfg.MinScore
	}

	store, err := i.storeForUser()
	if err != nil {
		return nil, err
	}
	embeddings, err := i.embedder.Embed(ctx, []string{query})
	if err != nil {
		return nil, err
	}
	if len(embeddings) != 1 {
		return nil, fmt.Errorf("unexpected embedding response size")
	}
	if len(embeddings[0]) == 0 {
		return nil, fmt.Errorf("empty embedding returned")
	}
	if err := store.EnsureSchema(ctx, len(embeddings[0])); err != nil {
		return nil, err
	}

	vecMatches, err := store.SearchVector(ctx, embeddings[0], maxResults)
	if err != nil {
		return nil, err
	}
	textMatches, err := store.SearchBM25(ctx, query, maxResults)
	if err != nil {
		return nil, err
	}

	return mergeMatches(vecMatches, textMatches, maxResults, minScore, i.cfg.FusionWeightVector, i.cfg.FusionWeightBM25), nil
}

func (i *Indexer) watchLoop() {
	for {
		select {
		case <-i.stopCh:
			return
		case event, ok := <-i.watcher.Events:
			if !ok {
				return
			}
			i.handleEvent(event)
		case err, ok := <-i.watcher.Errors:
			if !ok {
				return
			}
			i.logger.Warn("Memory index watcher error: %v", err)
		}
	}
}

func (i *Indexer) handleEvent(event fsnotify.Event) {
	if event.Name == "" {
		return
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}
	if event.Op&fsnotify.Create != 0 {
		info, err := os.Stat(event.Name)
		if err == nil && info.IsDir() {
			i.handleDirCreate(event.Name)
			return
		}
	}
	if !isMemoryFile(event.Name) {
		return
	}
	i.scheduleIndex(event.Name)
}

func (i *Indexer) handleDirCreate(path string) {
	if err := i.addWatchDir(path); err != nil {
		return
	}
	if isUserDir(i.rootDir, path) {
		_ = i.addWatchDir(filepath.Join(path, dailyDirName))
	}
}

func (i *Indexer) scheduleIndex(path string) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if timer, ok := i.timers[path]; ok {
		timer.Stop()
	}
	i.timers[path] = time.AfterFunc(defaultIndexDebounce, func() {
		_ = i.indexPath(context.Background(), path)
	})
}

func (i *Indexer) indexAll(ctx context.Context) error {
	paths, err := collectAllMemoryFiles(i.rootDir)
	if err != nil {
		return err
	}
	for _, path := range paths {
		if err := i.indexPath(ctx, path); err != nil {
			i.logger.Warn("Memory index failed for %s: %v", path, err)
		}
	}
	return nil
}

func (i *Indexer) indexPath(ctx context.Context, path string) error {
	if !isMemoryFile(path) {
		return nil
	}
	relPath, ok := resolveUserPath(i.rootDir, path)
	if !ok {
		return nil
	}
	store, err := i.storeForUser()
	if err != nil {
		return err
	}
	if _, err := os.Stat(path); err != nil {
		if os.IsNotExist(err) {
			return store.DeleteByPath(ctx, relPath)
		}
		return err
	}

	lines, err := readLines(path)
	if err != nil {
		return err
	}
	chunks := buildChunks(lines, i.cfg.ChunkTokens, i.cfg.ChunkOverlap)
	if len(chunks) == 0 {
		return store.DeleteByPath(ctx, relPath)
	}

	hashes := make([]string, 0, len(chunks))
	for _, chunk := range chunks {
		hashes = append(hashes, hashText(chunk.Text))
	}
	cache, err := store.LookupEmbeddings(ctx, hashes)
	if err != nil {
		return err
	}

	var missingTexts []string
	var missingHashes []string
	for idx, chunk := range chunks {
		hash := hashes[idx]
		if _, ok := cache[hash]; ok {
			continue
		}
		missingTexts = append(missingTexts, chunk.Text)
		missingHashes = append(missingHashes, hash)
	}

	if len(missingTexts) > 0 {
		embeddings, err := i.embedder.Embed(ctx, missingTexts)
		if err != nil {
			return err
		}
		if len(embeddings) != len(missingTexts) {
			return fmt.Errorf("embedding batch mismatch")
		}
		for idx, emb := range embeddings {
			cache[missingHashes[idx]] = emb
		}
	}

	dim := len(cache[hashes[0]])
	if err := store.EnsureSchema(ctx, dim); err != nil {
		return err
	}

	indexed := make([]IndexedChunk, 0, len(chunks))
	for idx, chunk := range chunks {
		hash := hashes[idx]
		embedding := cache[hash]
		if len(embedding) == 0 {
			return fmt.Errorf("empty embedding for chunk %s", hash)
		}
		indexed = append(indexed, IndexedChunk{
			Path:      relPath,
			StartLine: chunk.StartLine,
			EndLine:   chunk.EndLine,
			Text:      chunk.Text,
			Hash:      hash,
			Embedding: embedding,
		})
	}
	return store.ReplaceChunks(ctx, relPath, indexed)
}

func (i *Indexer) addWatchRoots() error {
	if err := i.addWatchDir(i.rootDir); err != nil {
		return err
	}
	_ = i.addWatchDir(filepath.Join(i.rootDir, dailyDirName))
	legacyUsersDir := filepath.Join(i.rootDir, legacyUserDirName)
	_ = i.addWatchDir(legacyUsersDir)

	entries, err := os.ReadDir(i.rootDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == legacyUserDirName {
			legacyEntries, err := os.ReadDir(filepath.Join(i.rootDir, legacyUserDirName))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return err
			}
			for _, legacyEntry := range legacyEntries {
				if !legacyEntry.IsDir() {
					continue
				}
				userPath := filepath.Join(legacyUsersDir, legacyEntry.Name())
				_ = i.addWatchDir(userPath)
				_ = i.addWatchDir(filepath.Join(userPath, dailyDirName))
			}
			continue
		}
		if isReservedUserDirName(name) {
			continue
		}
		userPath := filepath.Join(i.rootDir, name)
		_ = i.addWatchDir(userPath)
		_ = i.addWatchDir(filepath.Join(userPath, dailyDirName))
	}
	return nil
}

func (i *Indexer) addWatchDir(path string) error {
	if i == nil || i.watcher == nil {
		return nil
	}
	info, err := os.Stat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() {
		return nil
	}
	return i.watcher.Add(path)
}

func (i *Indexer) storeForUser() (*IndexStore, error) {
	const key = "root"
	i.mu.Lock()
	defer i.mu.Unlock()
	if store, ok := i.stores[key]; ok {
		return store, nil
	}
	path := i.cfg.DBPath
	store, err := OpenIndexStore(path)
	if err != nil {
		return nil, err
	}
	i.stores[key] = store
	return store, nil
}

type chunkWindow struct {
	StartLine int
	EndLine   int
	Text      string
}

func buildChunks(lines []string, chunkTokens, chunkOverlap int) []chunkWindow {
	if len(lines) == 0 {
		return nil
	}
	if chunkTokens <= 0 {
		chunkTokens = chunkTokenSize
	}
	if chunkOverlap < 0 {
		chunkOverlap = 0
	}

	lineCounts := make([]int, len(lines))
	for i, line := range lines {
		lineCounts[i] = len(tokenize(line))
	}

	var chunks []chunkWindow
	start := 0
	for start < len(lines) {
		end, _ := nextChunkEnd(start, lineCounts, chunkTokens)
		if end <= start {
			break
		}
		chunks = append(chunks, chunkWindow{
			StartLine: start + 1,
			EndLine:   end,
			Text:      strings.Join(lines[start:end], "\n"),
		})
		start = nextChunkStart(start, end, lineCounts, chunkOverlap)
		if start <= 0 {
			break
		}
	}
	return chunks
}

func hashText(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func isMemoryFile(path string) bool {
	if !strings.HasSuffix(strings.ToLower(path), ".md") {
		return false
	}
	base := filepath.Base(path)
	if base == memoryFileName {
		return true
	}
	needle := string(filepath.Separator) + dailyDirName + string(filepath.Separator)
	return strings.Contains(path, needle)
}

func isUserDir(root, path string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	rel = filepath.Clean(rel)
	if rel == "." || rel == "" {
		return false
	}
	parts := strings.Split(rel, string(filepath.Separator))
	if len(parts) == 1 {
		if parts[0] == legacyUserDirName || isReservedUserDirName(parts[0]) {
			return false
		}
		return true
	}
	if len(parts) == 2 && parts[0] == legacyUserDirName {
		return true
	}
	return false
}

func resolveUserPath(root, path string) (relPath string, ok bool) {
	root = filepath.Clean(root)
	path = filepath.Clean(path)
	if !strings.HasPrefix(path, root+string(os.PathSeparator)) && path != root {
		return "", false
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return "", false
	}
	parts := strings.Split(rel, string(os.PathSeparator))
	if len(parts) >= 2 && parts[0] == legacyUserDirName {
		return flattenLegacyRelativePath(parts[2:])
	}
	if len(parts) >= 2 && !isReservedUserDirName(parts[0]) {
		return flattenLegacyRelativePath(parts[1:])
	}
	return filepath.Clean(rel), true
}

func flattenLegacyRelativePath(parts []string) (string, bool) {
	if len(parts) == 1 && parts[0] == memoryFileName {
		return memoryFileName, true
	}
	if len(parts) >= 2 && parts[0] == dailyDirName {
		return filepath.Clean(filepath.Join(parts...)), true
	}
	return "", false
}

func collectAllMemoryFiles(root string) ([]string, error) {
	var paths []string
	base, err := collectMemoryFilesForRoot(root)
	if err != nil {
		return nil, err
	}
	paths = append(paths, base...)

	entries, err := os.ReadDir(root)
	if err != nil {
		if os.IsNotExist(err) {
			return paths, nil
		}
		return nil, err
	}
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name == legacyUserDirName {
			legacyEntries, err := os.ReadDir(filepath.Join(root, legacyUserDirName))
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, err
			}
			for _, legacyEntry := range legacyEntries {
				if !legacyEntry.IsDir() {
					continue
				}
				userRoot := filepath.Join(root, legacyUserDirName, legacyEntry.Name())
				userPaths, err := collectMemoryFilesForRoot(userRoot)
				if err != nil {
					continue
				}
				paths = append(paths, userPaths...)
			}
			continue
		}
		if isReservedUserDirName(name) {
			continue
		}
		userRoot := filepath.Join(root, name)
		userPaths, err := collectMemoryFilesForRoot(userRoot)
		if err != nil {
			continue
		}
		paths = append(paths, userPaths...)
	}
	return paths, nil
}

func mergeMatches(vec []VectorMatch, text []TextMatch, limit int, minScore float64, weightVector, weightBM25 float64) []SearchHit {
	if limit <= 0 {
		limit = defaultSearchMax
	}
	if minScore <= 0 {
		minScore = defaultSearchMinScore
	}
	if weightVector == 0 && weightBM25 == 0 {
		weightVector = 0.7
		weightBM25 = 0.3
	}

	type scoreEntry struct {
		chunk      StoredChunk
		vector     float64
		text       float64
		finalScore float64
	}
	entries := map[int64]*scoreEntry{}

	for _, match := range vec {
		entry := entries[match.Chunk.ID]
		if entry == nil {
			entry = &scoreEntry{chunk: match.Chunk}
			entries[match.Chunk.ID] = entry
		}
		entry.vector = 1.0 / (1.0 + match.Distance)
	}
	for _, match := range text {
		entry := entries[match.Chunk.ID]
		if entry == nil {
			entry = &scoreEntry{chunk: match.Chunk}
			entries[match.Chunk.ID] = entry
		}
		score := match.BM25
		if score < 0 {
			score = 0
		}
		entry.text = 1.0 / (1.0 + score)
	}

	results := make([]SearchHit, 0, len(entries))
	for _, entry := range entries {
		entry.finalScore = (weightVector * entry.vector) + (weightBM25 * entry.text)
		if entry.finalScore < minScore {
			continue
		}
		source := "memory"
		if filepath.Base(entry.chunk.Path) == memoryFileName {
			source = "long_term"
		}
		results = append(results, SearchHit{
			Path:      entry.chunk.Path,
			StartLine: entry.chunk.StartLine,
			EndLine:   entry.chunk.EndLine,
			Score:     entry.finalScore,
			Snippet:   buildSnippet(entry.chunk.Text),
			Source:    source,
		})
	}
	if len(results) > 1 {
		sort.Slice(results, func(i, j int) bool {
			if results[i].Score == results[j].Score {
				return results[i].Path < results[j].Path
			}
			return results[i].Score > results[j].Score
		})
	}
	if limit > 0 && len(results) > limit {
		return results[:limit]
	}
	return results
}
