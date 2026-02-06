package config

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"alex/internal/async"
	"alex/internal/logging"
	"github.com/fsnotify/fsnotify"
)

const defaultConfigWatchDebounce = 750 * time.Millisecond

// RuntimeConfigWatcher monitors the config file and refreshes the cache asynchronously.
type RuntimeConfigWatcher struct {
	path         string
	cache        *RuntimeConfigCache
	logger       logging.Logger
	debounce     time.Duration
	beforeReload func(context.Context) error

	mu       sync.Mutex
	timer    *time.Timer
	watcher  *fsnotify.Watcher
	stopCh   chan struct{}
	stopOnce sync.Once
}

// RuntimeConfigWatcherOption customizes watcher behavior.
type RuntimeConfigWatcherOption func(*RuntimeConfigWatcher)

// WithConfigWatchDebounce sets the debounce window for reloads.
func WithConfigWatchDebounce(debounce time.Duration) RuntimeConfigWatcherOption {
	return func(w *RuntimeConfigWatcher) {
		if debounce > 0 {
			w.debounce = debounce
		}
	}
}

// WithConfigWatchLogger sets the logger for watcher diagnostics.
func WithConfigWatchLogger(logger logging.Logger) RuntimeConfigWatcherOption {
	return func(w *RuntimeConfigWatcher) {
		w.logger = logging.OrNop(logger)
	}
}

// WithConfigWatchBeforeReload registers a hook to run before reloads.
func WithConfigWatchBeforeReload(fn func(context.Context) error) RuntimeConfigWatcherOption {
	return func(w *RuntimeConfigWatcher) {
		w.beforeReload = fn
	}
}

// NewRuntimeConfigWatcher constructs a watcher for the config path.
func NewRuntimeConfigWatcher(path string, cache *RuntimeConfigCache, opts ...RuntimeConfigWatcherOption) (*RuntimeConfigWatcher, error) {
	if cache == nil {
		return nil, fmt.Errorf("runtime config cache required")
	}
	path = strings.TrimSpace(path)
	if path == "" {
		return nil, fmt.Errorf("config path required")
	}
	if abs, err := filepath.Abs(path); err == nil {
		path = abs
	}
	path = filepath.Clean(path)

	watcher := &RuntimeConfigWatcher{
		path:     path,
		cache:    cache,
		logger:   logging.OrNop(nil),
		debounce: defaultConfigWatchDebounce,
		stopCh:   make(chan struct{}),
	}
	for _, opt := range opts {
		opt(watcher)
	}
	return watcher, nil
}

// Start begins watching the config file for changes.
func (w *RuntimeConfigWatcher) Start(ctx context.Context) error {
	if w == nil {
		return fmt.Errorf("runtime config watcher is nil")
	}
	w.mu.Lock()
	if w.watcher != nil {
		w.mu.Unlock()
		return nil
	}
	fsWatcher, err := fsnotify.NewWatcher()
	if err != nil {
		w.mu.Unlock()
		return err
	}
	w.watcher = fsWatcher
	w.mu.Unlock()

	dir := filepath.Dir(w.path)
	if err := fsWatcher.Add(dir); err != nil {
		_ = fsWatcher.Close()
		w.mu.Lock()
		w.watcher = nil
		w.mu.Unlock()
		return err
	}

	async.Go(w.logger, "config.watch", w.watchLoop)
	if ctx != nil {
		async.Go(w.logger, "config.watch.ctx", func() {
			<-ctx.Done()
			w.Stop()
		})
	}
	return nil
}

// Stop terminates the watcher.
func (w *RuntimeConfigWatcher) Stop() {
	if w == nil {
		return
	}
	w.stopOnce.Do(func() {
		close(w.stopCh)
		w.mu.Lock()
		if w.timer != nil {
			w.timer.Stop()
			w.timer = nil
		}
		if w.watcher != nil {
			_ = w.watcher.Close()
			w.watcher = nil
		}
		w.mu.Unlock()
	})
}

// Updates exposes the cache update signals.
func (w *RuntimeConfigWatcher) Updates() <-chan struct{} {
	if w == nil || w.cache == nil {
		return nil
	}
	return w.cache.Updates()
}

// Resolve proxies to the underlying cache.
func (w *RuntimeConfigWatcher) Resolve(ctx context.Context) (RuntimeConfig, Metadata, error) {
	if w == nil || w.cache == nil {
		return RuntimeConfig{}, Metadata{}, fmt.Errorf("runtime config watcher not initialized")
	}
	return w.cache.Resolve(ctx)
}

func (w *RuntimeConfigWatcher) watchLoop() {
	for {
		select {
		case <-w.stopCh:
			return
		case event, ok := <-w.watcher.Events:
			if !ok {
				return
			}
			w.handleEvent(event)
		case err, ok := <-w.watcher.Errors:
			if !ok {
				return
			}
			w.logger.Warn("Config watcher error: %v", err)
		}
	}
}

func (w *RuntimeConfigWatcher) handleEvent(event fsnotify.Event) {
	if event.Name == "" {
		return
	}
	if event.Op&(fsnotify.Create|fsnotify.Write|fsnotify.Remove|fsnotify.Rename) == 0 {
		return
	}
	if filepath.Clean(event.Name) != w.path {
		return
	}
	w.scheduleReload()
}

func (w *RuntimeConfigWatcher) scheduleReload() {
	w.mu.Lock()
	defer w.mu.Unlock()
	if w.timer != nil {
		w.timer.Stop()
	}
	w.timer = time.AfterFunc(w.debounce, func() {
		select {
		case <-w.stopCh:
			return
		default:
		}
		if w.beforeReload != nil {
			if err := w.beforeReload(context.Background()); err != nil {
				w.logger.Warn("Config pre-reload failed: %v", err)
				return
			}
		}
		if err := w.cache.Reload(context.Background()); err != nil {
			w.logger.Warn("Config reload failed: %v", err)
		}
	})
}
