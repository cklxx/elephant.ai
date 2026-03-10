package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/toolregistry"
	"alex/internal/delivery/channels"
	"alex/internal/delivery/channels/telegram"
	serverApp "alex/internal/delivery/server/app"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// registerTelegramChannel registers the Telegram channel plugin into the
// registry if Telegram is enabled. The plugin factory captures the full
// Config and dependencies needed to start the gateway.
func registerTelegramChannel(cfg Config, registry *ChannelRegistry, container *di.Container, logger logging.Logger, broadcaster *serverApp.EventBroadcaster) {
	tgCfg := cfg.Channels.TelegramConfig()
	if !tgCfg.Enabled {
		return
	}
	registry.Register(channels.PluginFactory{
		Name:     "telegram",
		Required: false,
		Build: func(ctx context.Context) (func(), error) {
			return startTelegramGateway(ctx, cfg, container, logger, broadcaster)
		},
	})
}

func startTelegramGateway(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger, broadcaster *serverApp.EventBroadcaster) (func(), error) {
	logger = logging.OrNop(logger)
	tgCfg := cfg.Channels.TelegramConfig()
	if !tgCfg.Enabled {
		return nil, nil
	}
	if container == nil {
		return nil, fmt.Errorf("telegram gateway requires server container")
	}
	if strings.TrimSpace(tgCfg.BotToken) == "" {
		return nil, fmt.Errorf("telegram gateway requires channels.telegram.bot_token")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	browserCfg := toolregistry.BrowserConfig{
		Headless: true,
		Timeout:  60 * time.Second,
	}
	altCoord, err := container.BuildAlternateCoordinator(
		"cli",
		toolregistry.ToolsetDefault,
		browserCfg,
	)
	if err != nil {
		return nil, fmt.Errorf("build telegram alternate coordinator: %w", err)
	}
	if summary := cfg.EnvironmentSummary; summary != "" {
		altCoord.AgentCoordinator.SetEnvironmentSummary(summary)
	}

	gatewayCfg := buildTelegramGatewayConfig(tgCfg, container)

	gateway, err := telegram.NewGateway(gatewayCfg, altCoord.AgentCoordinator, logger)
	if err != nil {
		_ = altCoord.Shutdown()
		return nil, err
	}

	wireTelegramGateway(ctx, gateway, gatewayCfg, container, broadcaster, logger)

	async.Go(logger, "telegram.gateway", func() {
		if err := gateway.Start(ctx); err != nil {
			logger.Warn("Telegram gateway stopped: %v", err)
		}
	})

	cleanup := func() {
		gateway.Stop()
		if err := altCoord.Shutdown(); err != nil {
			logger.Warn("Telegram alternate coordinator shutdown failed: %v", err)
		}
	}

	return cleanup, nil
}

func buildTelegramGatewayConfig(tgCfg TelegramGatewayConfig, container *di.Container) telegram.Config {
	gatewayCfg := telegram.Config{
		BaseConfig:                    tgCfg.BaseConfig,
		Enabled:                       tgCfg.Enabled,
		BotToken:                      tgCfg.BotToken,
		AllowedGroups:                 append([]int64(nil), tgCfg.AllowedGroups...),
		ShowToolProgress:              tgCfg.ShowToolProgress,
		SlowProgressSummaryEnabled:    &tgCfg.SlowProgressSummaryEnabled,
		SlowProgressSummaryDelay:      tgCfg.SlowProgressSummaryDelay,
		PlanReviewEnabled:             tgCfg.PlanReviewEnabled,
		PlanReviewRequireConfirmation: tgCfg.PlanReviewRequireConfirmation,
		PlanReviewPendingTTL:          tgCfg.PlanReviewPendingTTL,
		ActiveSlotTTL:                 tgCfg.ActiveSlotTTL,
		ActiveSlotMaxEntries:          tgCfg.ActiveSlotMaxEntries,
		StateCleanupInterval:          tgCfg.StateCleanupInterval,
		PersistenceMode:               tgCfg.PersistenceMode,
		PersistenceDir:                tgCfg.PersistenceDir,
		PersistenceRetention:          tgCfg.PersistenceRetention,
		PersistenceMaxTasksPerChat:    tgCfg.PersistenceMaxTasksPerChat,
		MaxConcurrentTasks:            tgCfg.MaxConcurrentTasks,
	}

	if gatewayCfg.PlanReviewEnabled {
		if gatewayCfg.PlanReviewPendingTTL <= 0 {
			gatewayCfg.PlanReviewPendingTTL = 60 * time.Minute
		}
		if !gatewayCfg.PlanReviewRequireConfirmation {
			gatewayCfg.PlanReviewRequireConfirmation = true
		}
	}

	if strings.TrimSpace(gatewayCfg.PersistenceMode) == "" {
		gatewayCfg.PersistenceMode = persistenceModeFile
	}
	if strings.TrimSpace(gatewayCfg.PersistenceDir) == "" {
		gatewayCfg.PersistenceDir = filepath.Join(container.SessionDir(), "telegram")
	}
	return gatewayCfg
}

func wireTelegramGateway(ctx context.Context, gateway *telegram.Gateway, gatewayCfg telegram.Config, container *di.Container, broadcaster *serverApp.EventBroadcaster, logger logging.Logger) {
	if broadcaster != nil {
		gateway.SetEventListener(broadcaster)
	}

	if gatewayCfg.PlanReviewEnabled {
		store, err := buildTelegramPlanReviewStore(ctx, gatewayCfg)
		if err != nil {
			logger.Warn("Telegram plan review store init failed: %v", err)
		} else {
			gateway.SetPlanReviewStore(store)
		}
	}

	if container.HasLLMFactory() {
		gateway.SetLLMFactory(container.LLMFactory(), container.DefaultLLMProfile())
	}

	taskStore, err := buildTelegramTaskStore(ctx, gatewayCfg)
	if err != nil {
		logger.Warn("Telegram task store init failed: %v", err)
	} else {
		gateway.SetTaskStore(taskStore)
		if err := taskStore.MarkStaleRunning(ctx, "gateway restart"); err != nil {
			logger.Warn("Telegram task store stale cleanup failed: %v", err)
		}
		logger.Info("Telegram task store enabled (mode=%s)", utils.TrimLower(gatewayCfg.PersistenceMode))
	}
}

func buildTelegramPlanReviewStore(ctx context.Context, cfg telegram.Config) (telegram.PlanReviewStore, error) {
	mode := utils.TrimLower(cfg.PersistenceMode)
	switch mode {
	case persistenceModeMemory:
		store := telegram.NewPlanReviewMemoryStore(cfg.PlanReviewPendingTTL)
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	case persistenceModeFile:
		store, err := telegram.NewPlanReviewFileStore(cfg.PersistenceDir, cfg.PlanReviewPendingTTL)
		if err != nil {
			return nil, err
		}
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported telegram persistence mode %q", cfg.PersistenceMode)
	}
}

func buildTelegramTaskStore(ctx context.Context, cfg telegram.Config) (telegram.TaskStore, error) {
	mode := utils.TrimLower(cfg.PersistenceMode)
	switch mode {
	case persistenceModeMemory:
		store := telegram.NewTaskMemoryStore(cfg.PersistenceRetention, cfg.PersistenceMaxTasksPerChat)
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	case persistenceModeFile:
		store, err := telegram.NewTaskFileStore(cfg.PersistenceDir, cfg.PersistenceRetention, cfg.PersistenceMaxTasksPerChat)
		if err != nil {
			return nil, err
		}
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported telegram persistence mode %q", cfg.PersistenceMode)
	}
}
