package di

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	agentcost "alex/internal/app/agent/cost"
	"alex/internal/app/decision"
	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/memory"
	"alex/internal/infra/session/filestore"
	sessionstate "alex/internal/infra/session/state_store"
	"alex/internal/infra/storage"
)

func (b *containerBuilder) buildSessionResources() (sessionResources, error) {
	return sessionResources{
		sessionStore: filestore.New(b.sessionDir),
		stateStore:   sessionstate.NewFileStore(filepath.Join(b.sessionDir, "snapshots")),
		historyStore: sessionstate.NewFileStore(filepath.Join(b.sessionDir, "turns")),
	}, nil
}

func (b *containerBuilder) buildMemoryEngine(ctx context.Context) (memory.Engine, error) {
	root := resolveStorageDir(b.config.MemoryDir, "~/.alex/memory")
	engine := memory.NewMarkdownEngine(root)
	indexCfg := b.config.Proactive.Memory.Index
	if indexCfg.ChunkTokens > 0 || indexCfg.ChunkOverlap >= 0 {
		engine.SetChunkConfig(indexCfg.ChunkTokens, indexCfg.ChunkOverlap)
	}
	if indexCfg.Enabled {
		b.logger.Warn("Memory indexer requires an embedding provider; skipping (no provider configured)")
	}
	if err := engine.EnsureSchema(context.Background()); err != nil {
		b.logger.Warn("Failed to initialize memory root: %v", err)
	}

	memoryCfg := b.config.Proactive.Memory
	if memoryCfg.ArchiveAfterDays > 0 {
		interval, err := time.ParseDuration(memoryCfg.CleanupInterval)
		if err != nil || interval <= 0 {
			interval = 24 * time.Hour
		}
		engine.StartCleanupLoop(ctx, memory.CleanupConfig{
			ArchiveAfterDays: memoryCfg.ArchiveAfterDays,
			CleanupInterval:  interval,
		})
	}

	return engine, nil
}

func (b *containerBuilder) buildDecisionStore() (*decision.Store, error) {
	path := filepath.Join(b.sessionDir, "_decisions", "decisions.json")
	return decision.NewStore(path)
}

func (b *containerBuilder) buildCostTracker() (agentstorage.CostTracker, error) {
	costStore, err := storage.NewFileCostStore(b.costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	return agentcost.NewCostTracker(costStore), nil
}
