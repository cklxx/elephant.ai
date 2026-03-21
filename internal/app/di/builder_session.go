package di

import (
	"context"
	"fmt"
	"path/filepath"
	"time"

	agentcost "alex/internal/app/agent/cost"
	"alex/internal/app/decision"
	coretape "alex/internal/core/tape"
	agentstorage "alex/internal/domain/agent/ports/storage"
	"alex/internal/infra/memory"
	sessionstate "alex/internal/infra/session/state_store"
	"alex/internal/infra/storage"
	"alex/internal/infra/tape"
)

// tapeStore returns the shared FileStore for all tape-backed components.
// Falls back to an in-memory store on failure.
func (b *containerBuilder) tapeStore() coretape.TapeStore {
	store, err := tape.NewFileStore(filepath.Join(b.sessionDir, "tapes"))
	if err != nil {
		b.logger.Error("Failed to create tape store: %v; falling back to in-memory", err)
		return tape.NewMemoryStore()
	}
	return store
}

func (b *containerBuilder) buildSessionResources() sessionResources {
	return sessionResources{
		sessionStore: tape.NewSessionAdapter(b.tapeStore()),
		stateStore:   sessionstate.NewFileStore(filepath.Join(b.sessionDir, "snapshots")),
		historyStore: sessionstate.NewFileStore(filepath.Join(b.sessionDir, "turns")),
	}
}

func (b *containerBuilder) buildTapeMessageReader() *tape.MessageReader {
	return tape.NewMessageReader(b.tapeStore())
}

func (b *containerBuilder) buildTurnRecorder() *tape.TurnRecorder {
	mgr := coretape.NewTapeManager(b.tapeStore(), coretape.TapeContext{})
	return tape.NewTurnRecorder(mgr)
}

func (b *containerBuilder) buildMemoryEngine(ctx context.Context) memory.Engine {
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

	return engine
}

func (b *containerBuilder) buildCheckpointStore() *tape.CheckpointStore {
	return tape.NewCheckpointStore(b.tapeStore(), filepath.Join(b.sessionDir, "checkpoints"))
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
