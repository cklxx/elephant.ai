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
	"alex/internal/delivery/channels/lark"
	serverApp "alex/internal/delivery/server/app"
	"alex/internal/domain/agent/presets"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
	"alex/internal/shared/utils"
)

// registerLarkChannel registers the Lark channel plugin into the registry
// if Lark is enabled. The plugin factory captures the full Config and
// dependencies needed to start the gateway.
func registerLarkChannel(cfg Config, registry *ChannelRegistry, container *di.Container, logger logging.Logger, broadcaster *serverApp.EventBroadcaster) {
	larkCfg := cfg.Channels.LarkConfig()
	if !larkCfg.Enabled {
		return
	}
	registry.Register(channels.PluginFactory{
		Name:     "lark",
		Required: true,
		Build: func(ctx context.Context) (func(), error) {
			return startLarkGateway(ctx, cfg, container, logger, broadcaster)
		},
	})
}

func startLarkGateway(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger, broadcaster *serverApp.EventBroadcaster) (func(), error) {
	logger = logging.OrNop(logger)
	larkCfg := cfg.Channels.LarkConfig()
	if !larkCfg.Enabled {
		return nil, nil
	}
	if container == nil {
		return nil, fmt.Errorf("lark gateway requires server container")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	toolMode := utils.TrimLower(larkCfg.ToolMode)
	if toolMode == "" {
		toolMode = string(presets.ToolModeCLI)
	}
	if toolMode != string(presets.ToolModeCLI) && toolMode != string(presets.ToolModeWeb) {
		return nil, fmt.Errorf("lark tool_mode must be cli or web, got %q", larkCfg.ToolMode)
	}

	browserCfg := toolregistry.BrowserConfig{
		CDPURL:      larkCfg.Browser.CDPURL,
		ChromePath:  larkCfg.Browser.ChromePath,
		Headless:    larkCfg.Browser.Headless,
		UserDataDir: larkCfg.Browser.UserDataDir,
		Timeout:     larkCfg.Browser.Timeout,
	}
	altCoord, err := container.BuildAlternateCoordinator(
		toolMode,
		toolregistry.ToolsetLarkLocal,
		browserCfg,
	)
	if err != nil {
		return nil, fmt.Errorf("build lark alternate coordinator: %w", err)
	}
	if summary := cfg.EnvironmentSummary; summary != "" {
		altCoord.AgentCoordinator.SetEnvironmentSummary(summary)
	}

	gatewayCfg := lark.Config{
		BaseConfig:                    larkCfg.BaseConfig,
		Enabled:                       larkCfg.Enabled,
		AppID:                         larkCfg.AppID,
		AppSecret:                     larkCfg.AppSecret,
		TenantCalendarID:              larkCfg.TenantCalendarID,
		BaseDomain:                    larkCfg.BaseDomain,
		WorkspaceDir:                  larkCfg.WorkspaceDir,
		AutoUploadFiles:               larkCfg.AutoUploadFiles,
		AutoUploadMaxBytes:            larkCfg.AutoUploadMaxBytes,
		AutoUploadAllowExt:            append([]string(nil), larkCfg.AutoUploadAllowExt...),
		Browser:                       larkCfg.Browser,
		InjectionAckReactEmoji:        larkCfg.InjectionAckReactEmoji,
		ShowToolProgress:              larkCfg.ShowToolProgress,
		SlowProgressSummaryEnabled:    &larkCfg.SlowProgressSummaryEnabled,
		SlowProgressSummaryDelay:      larkCfg.SlowProgressSummaryDelay,
		AutoChatContextSize:           larkCfg.AutoChatContextSize,
		ToolFailureAbortThreshold:     larkCfg.ToolFailureAbortThreshold,
		PlanReviewEnabled:             larkCfg.PlanReviewEnabled,
		PlanReviewRequireConfirmation: larkCfg.PlanReviewRequireConfirmation,
		PlanReviewPendingTTL:          larkCfg.PlanReviewPendingTTL,
		ActiveSlotTTL:                 larkCfg.ActiveSlotTTL,
		ActiveSlotMaxEntries:          larkCfg.ActiveSlotMaxEntries,
		PendingInputRelayTTL:          larkCfg.PendingInputRelayTTL,
		PendingInputRelayMaxChats:     larkCfg.PendingInputRelayMaxChats,
		PendingInputRelayMaxPerChat:   larkCfg.PendingInputRelayMaxPerChat,
		AIChatSessionTTL:              larkCfg.AIChatSessionTTL,
		StateCleanupInterval:          larkCfg.StateCleanupInterval,
		PersistenceMode:               larkCfg.PersistenceMode,
		PersistenceDir:                larkCfg.PersistenceDir,
		PersistenceRetention:          larkCfg.PersistenceRetention,
		PersistenceMaxTasksPerChat:    larkCfg.PersistenceMaxTasksPerChat,
		MaxConcurrentTasks:            larkCfg.MaxConcurrentTasks,
		DefaultPlanMode:               lark.PlanMode(larkCfg.DefaultPlanMode),
		DeliveryMode:                  larkCfg.DeliveryMode,
		DeliveryWorker:                larkCfg.DeliveryWorker,
		AttentionGate:                 larkCfg.AttentionGate,
	}

	// Hooks bridge endpoint lives on the debug HTTP server (DebugPort),
	// not on the web API server (Port).
	hooksPort := strings.TrimPrefix(cfg.DebugPort, ":")
	if hooksPort == "" {
		hooksPort = "9090"
	}
	gatewayCfg.CCHooksAutoConfig = &lark.CCHooksAutoConfig{
		ServerURL: "http://localhost:" + hooksPort,
		Token:     cfg.HooksBridge.Token,
	}

	if gatewayCfg.PlanReviewEnabled {
		if gatewayCfg.PlanReviewPendingTTL <= 0 {
			gatewayCfg.PlanReviewPendingTTL = 60 * time.Minute
		}
		if !gatewayCfg.PlanReviewRequireConfirmation {
			gatewayCfg.PlanReviewRequireConfirmation = true
		}
	}

	if utils.IsBlank(gatewayCfg.PersistenceMode) {
		gatewayCfg.PersistenceMode = larkPersistenceModeFile
	}
	if utils.IsBlank(gatewayCfg.PersistenceDir) {
		gatewayCfg.PersistenceDir = filepath.Join(container.SessionDir(), "lark")
	}

	var planReviewStore lark.PlanReviewStore
	if gatewayCfg.PlanReviewEnabled {
		store, err := buildLarkPlanReviewStore(ctx, gatewayCfg)
		if err != nil {
			logger.Warn("Lark plan review store init failed: %v", err)
			gatewayCfg.PlanReviewEnabled = false
		} else {
			planReviewStore = store
		}
	}

	var chatSessionStore lark.ChatSessionBindingStore
	store, err := buildLarkChatSessionStore(ctx, gatewayCfg)
	if err != nil {
		logger.Warn("Lark chat session binding store init failed: %v", err)
	} else {
		chatSessionStore = store
	}

	var deliveryOutboxStore lark.DeliveryOutboxStore
	if mode := utils.TrimLower(gatewayCfg.DeliveryMode); mode != "direct" {
		outboxStore, outboxErr := buildLarkDeliveryOutboxStore(ctx, gatewayCfg)
		if outboxErr != nil {
			logger.Warn("Lark delivery outbox store init failed: %v", outboxErr)
			gatewayCfg.DeliveryMode = "direct"
		} else {
			deliveryOutboxStore = outboxStore
		}
	}

	taskStore, err := larkTaskStoreForContainer(container)
	if err != nil {
		_ = altCoord.Shutdown()
		return nil, fmt.Errorf("lark task store init failed: %w", err)
	}

	gateway, err := lark.NewGateway(gatewayCfg, altCoord.AgentCoordinator, logger)
	if err != nil {
		_ = altCoord.Shutdown()
		return nil, err
	}
	container.LarkGateway = gateway

	// Lark calendar operations require user-scoped OAuth tokens to access a user's
	// primary calendar. Provide an OAuth service to tools via the gateway context.
	// Also set up AutoAuth for in-message device flow authorization.
	if oauthSvc := buildLarkOAuthService(ctx, cfg, container, logger); oauthSvc != nil {
		container.LarkOAuth = oauthSvc
		gateway.SetOAuthService(oauthSvc)
		gateway.EnableAutoAuth(oauthSvc, logger)
	}

	if broadcaster != nil {
		gateway.SetEventListener(broadcaster)
	}
	if planReviewStore != nil {
		gateway.SetPlanReviewStore(planReviewStore)
	}
	if chatSessionStore != nil {
		gateway.SetChatSessionBindingStore(chatSessionStore)
	}
	if deliveryOutboxStore != nil {
		gateway.SetDeliveryOutboxStore(deliveryOutboxStore)
	}

	if container.HasLLMFactory() {
		gateway.SetLLMFactory(container.LLMFactory(), container.DefaultLLMProfile())
	}
	if container.CostTracker != nil {
		gateway.SetCostTracker(container.CostTracker)
	}

	gateway.SetTaskStore(taskStore)
	if err := taskStore.MarkStaleRunning(ctx, "gateway restart"); err != nil {
		logger.Warn("Lark task store stale cleanup failed: %v", err)
	}
	logger.Info("Lark task store enabled (mode=unified)")

	async.Go(logger, "lark.gateway", func() {
		if err := gateway.Start(ctx); err != nil {
			logger.Warn("Lark gateway stopped: %v", err)
		}
	})

	cleanup := func() {
		interrupted := gateway.NotifyRunningTaskInterruptions("系统正在维护中，您的任务将在服务恢复后自动重新执行。")
		if interrupted > 0 {
			logger.Info("Lark gateway sent interruption notice to %d running chats", interrupted)
		}
		gateway.Stop()
		if container.LarkGateway == gateway {
			container.LarkGateway = nil
		}
		if err := altCoord.Shutdown(); err != nil {
			logger.Warn("Lark alternate coordinator shutdown failed: %v", err)
		}
	}

	return cleanup, nil
}

func buildLarkPlanReviewStore(ctx context.Context, cfg lark.Config) (lark.PlanReviewStore, error) {
	mode := utils.TrimLower(cfg.PersistenceMode)
	switch mode {
	case larkPersistenceModeMemory:
		store := lark.NewPlanReviewMemoryStore(cfg.PlanReviewPendingTTL)
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	case larkPersistenceModeFile:
		store, err := lark.NewPlanReviewFileStore(cfg.PersistenceDir, cfg.PlanReviewPendingTTL)
		if err != nil {
			return nil, err
		}
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported lark persistence mode %q", cfg.PersistenceMode)
	}
}

func buildLarkChatSessionStore(ctx context.Context, cfg lark.Config) (lark.ChatSessionBindingStore, error) {
	mode := utils.TrimLower(cfg.PersistenceMode)
	switch mode {
	case larkPersistenceModeMemory:
		store := lark.NewChatSessionBindingMemoryStore()
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	case larkPersistenceModeFile:
		store, err := lark.NewChatSessionBindingFileStore(cfg.PersistenceDir)
		if err != nil {
			return nil, err
		}
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported lark persistence mode %q", cfg.PersistenceMode)
	}
}

func buildLarkDeliveryOutboxStore(ctx context.Context, cfg lark.Config) (lark.DeliveryOutboxStore, error) {
	mode := utils.TrimLower(cfg.PersistenceMode)
	switch mode {
	case larkPersistenceModeMemory:
		store := lark.NewDeliveryOutboxMemoryStore()
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	case larkPersistenceModeFile:
		store, err := lark.NewDeliveryOutboxFileStore(cfg.PersistenceDir)
		if err != nil {
			return nil, err
		}
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	default:
		return nil, fmt.Errorf("unsupported lark persistence mode %q", cfg.PersistenceMode)
	}
}

func buildLarkOAuthService(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger) *larkoauth.Service {
	logger = logging.OrNop(logger)
	if container == nil {
		return nil
	}
	larkCfg := cfg.Channels.LarkConfig()
	if !larkCfg.Enabled {
		return nil
	}
	if utils.IsBlank(larkCfg.AppID) || utils.IsBlank(larkCfg.AppSecret) {
		return nil
	}

	// In standalone mode, OAuth handlers are served by the embedded debug HTTP
	// server, so redirect_uri must target DebugPort first.
	port := strings.TrimPrefix(strings.TrimSpace(cfg.DebugPort), ":")
	if port == "" {
		port = strings.TrimPrefix(strings.TrimSpace(cfg.Port), ":")
	}
	if port == "" {
		port = "9090"
	}
	redirectBase := fmt.Sprintf("http://localhost:%s", port)
	if !strings.HasPrefix(redirectBase, "http://") && !strings.HasPrefix(redirectBase, "https://") {
		redirectBase = "https://" + redirectBase
	}
	redirectBase = strings.TrimRight(redirectBase, "/")

	dir := filepath.Join(container.SessionDir(), "_lark_oauth")
	tokenStore, err := larkoauth.NewFileTokenStore(dir)
	if err != nil {
		logger.Warn("Lark OAuth token store init failed (file): %v", err)
		return nil
	}
	logger.Info("Lark OAuth token store backed by file: %s", dir)

	svc, err := larkoauth.NewService(larkoauth.ServiceConfig{
		AppID:        larkCfg.AppID,
		AppSecret:    larkCfg.AppSecret,
		BaseDomain:   larkCfg.BaseDomain,
		RedirectBase: redirectBase,
	}, tokenStore, larkoauth.NewMemoryStateStore())
	if err != nil {
		logger.Warn("Lark OAuth service init failed: %v", err)
		return nil
	}
	return svc
}
