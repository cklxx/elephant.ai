package memory

import (
	"context"
	"fmt"
	"path/filepath"
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
	rootDir        string
	cfg            IndexerConfig
	embedder       EmbeddingProvider
	logger         logging.Logger
	searchVectorFn func(ctx context.Context, store *IndexStore, embedding []float32, maxResults int) ([]VectorMatch, error)
	searchBM25Fn   func(ctx context.Context, store *IndexStore, query string, maxResults int) ([]TextMatch, error)
	ensureSchemaFn func(ctx context.Context, store *IndexStore, dim int) error
	countRelatedFn func(ctx context.Context, store *IndexStore, path string, fromLine, toLine int) (int, error)

	mu       sync.Mutex
	watcher  *fsnotify.Watcher
	timers   map[string]*time.Timer
	store    *IndexStore
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
		if i.store != nil {
			_ = i.store.Close()
			i.store = nil
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

func (i *Indexer) storeForUser() (*IndexStore, error) {
	i.mu.Lock()
	defer i.mu.Unlock()
	if i.store != nil {
		return i.store, nil
	}
	path := i.cfg.DBPath
	store, err := OpenIndexStore(path)
	if err != nil {
		return nil, err
	}
	i.store = store
	return store, nil
}

func (i *Indexer) normalizePath(path string) (string, error) {
	path = strings.TrimSpace(path)
	if path == "" {
		return "", fmt.Errorf("path is required")
	}
	if filepath.IsAbs(path) {
		relPath, ok := resolveUserPath(i.rootDir, path)
		if !ok {
			return "", fmt.Errorf("path outside memory root")
		}
		return filepath.ToSlash(filepath.Clean(relPath)), nil
	}
	return filepath.ToSlash(filepath.Clean(path)), nil
}
