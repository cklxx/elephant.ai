package config

import (
	"context"
	"fmt"
	"sync"
)

// RuntimeConfigLoader loads a fresh runtime configuration snapshot.
type RuntimeConfigLoader func(context.Context) (RuntimeConfig, Metadata, error)

// RuntimeConfigCache stores the latest runtime configuration snapshot.
type RuntimeConfigCache struct {
	loader RuntimeConfigLoader

	mu      sync.RWMutex
	cfg     RuntimeConfig
	meta    Metadata
	loaded  bool
	updates chan struct{}
}

// NewRuntimeConfigCache loads the initial snapshot and returns a cache for future reloads.
func NewRuntimeConfigCache(loader RuntimeConfigLoader) (*RuntimeConfigCache, error) {
	if loader == nil {
		return nil, fmt.Errorf("runtime config loader required")
	}
	cfg, meta, err := loader(context.Background())
	if err != nil {
		return nil, err
	}
	return &RuntimeConfigCache{
		loader:  loader,
		cfg:     cfg,
		meta:    meta,
		loaded:  true,
		updates: make(chan struct{}, 1),
	}, nil
}

// Resolve returns the latest cached runtime config without triggering a reload.
func (c *RuntimeConfigCache) Resolve(ctx context.Context) (RuntimeConfig, Metadata, error) {
	_ = ctx
	if c == nil {
		return RuntimeConfig{}, Metadata{}, fmt.Errorf("runtime config cache is nil")
	}
	c.mu.RLock()
	defer c.mu.RUnlock()
	if !c.loaded {
		return RuntimeConfig{}, Metadata{}, fmt.Errorf("runtime config cache not initialized")
	}
	return c.cfg, c.meta, nil
}

// Reload refreshes the cached runtime config using the loader.
func (c *RuntimeConfigCache) Reload(ctx context.Context) error {
	if c == nil {
		return fmt.Errorf("runtime config cache is nil")
	}
	if ctx == nil {
		ctx = context.Background()
	}
	cfg, meta, err := c.loader(ctx)
	if err != nil {
		return err
	}
	c.mu.Lock()
	c.cfg = cfg
	c.meta = meta
	c.loaded = true
	c.mu.Unlock()

	select {
	case c.updates <- struct{}{}:
	default:
	}
	return nil
}

// Updates returns a channel that receives signals after successful reloads.
func (c *RuntimeConfigCache) Updates() <-chan struct{} {
	if c == nil {
		return nil
	}
	return c.updates
}
