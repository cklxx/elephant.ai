package di

import (
	"context"
	"fmt"
	"path/filepath"

	agentcost "alex/internal/app/agent/cost"
	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/analytics/journal"
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

func (b *containerBuilder) buildMemoryEngine() (memory.Engine, error) {
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
	return engine, nil
}

func (b *containerBuilder) buildJournalWriter() journal.Writer {
	journalDir := filepath.Join(b.sessionDir, "journals")
	fileWriter, err := journal.NewFileWriter(journalDir)
	if err != nil {
		b.logger.Warn("Failed to initialize journal writer: %v", err)
		return journal.NopWriter()
	}
	return fileWriter
}

func (b *containerBuilder) buildCostTracker() (agentstorage.CostTracker, error) {
	costStore, err := storage.NewFileCostStore(b.costDir)
	if err != nil {
		return nil, fmt.Errorf("failed to create cost store: %w", err)
	}
	return agentcost.NewCostTracker(costStore), nil
}
