package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/app/di"
	"alex/internal/app/toolregistry"
	"alex/internal/delivery/channels/lark"
	serverApp "alex/internal/delivery/server/app"
	larkoauth "alex/internal/infra/lark/oauth"
	"alex/internal/shared/agent/presets"
	"alex/internal/shared/async"
	"alex/internal/shared/logging"
)

func startLarkGateway(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger, broadcaster *serverApp.EventBroadcaster) (func(), error) {
	logger = logging.OrNop(logger)
	larkCfg := cfg.Channels.Lark
	if !larkCfg.Enabled {
		return nil, nil
	}
	if container == nil {
		return nil, fmt.Errorf("lark gateway requires server container")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	toolMode := strings.ToLower(strings.TrimSpace(larkCfg.ToolMode))
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
		BaseConfig:         larkCfg.BaseConfig,
		Enabled:            larkCfg.Enabled,
		AppID:              larkCfg.AppID,
		AppSecret:          larkCfg.AppSecret,
		TenantCalendarID:   larkCfg.TenantCalendarID,
		BaseDomain:         larkCfg.BaseDomain,
		WorkspaceDir:       larkCfg.WorkspaceDir,
		AutoUploadFiles:    larkCfg.AutoUploadFiles,
		AutoUploadMaxBytes: larkCfg.AutoUploadMaxBytes,
		AutoUploadAllowExt: append([]string(nil), larkCfg.AutoUploadAllowExt...),
		Browser: lark.BrowserConfig{
			CDPURL:      larkCfg.Browser.CDPURL,
			ChromePath:  larkCfg.Browser.ChromePath,
			Headless:    larkCfg.Browser.Headless,
			UserDataDir: larkCfg.Browser.UserDataDir,
			Timeout:     larkCfg.Browser.Timeout,
		},
		ReactEmoji:                    larkCfg.ReactEmoji,
		InjectionAckReactEmoji:        larkCfg.InjectionAckReactEmoji,
		FinalAnswerReviewReactEmoji:   larkCfg.FinalAnswerReviewReactEmoji,
		ShowToolProgress:              larkCfg.ShowToolProgress,
		AutoChatContextSize:           larkCfg.AutoChatContextSize,
		PlanReviewEnabled:             larkCfg.PlanReviewEnabled,
		PlanReviewRequireConfirmation: larkCfg.PlanReviewRequireConfirmation,
		PlanReviewPendingTTL:          larkCfg.PlanReviewPendingTTL,
		PersistenceMode:               larkCfg.PersistenceMode,
		PersistenceDir:                larkCfg.PersistenceDir,
		PersistenceRetention:          larkCfg.PersistenceRetention,
		PersistenceMaxTasksPerChat:    larkCfg.PersistenceMaxTasksPerChat,
		MaxConcurrentTasks:            larkCfg.MaxConcurrentTasks,
		DefaultPlanMode:               lark.PlanMode(larkCfg.DefaultPlanMode),
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

	if strings.TrimSpace(gatewayCfg.PersistenceMode) == "" {
		gatewayCfg.PersistenceMode = larkPersistenceModeFile
	}
	if strings.TrimSpace(gatewayCfg.PersistenceDir) == "" {
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

	gateway, err := lark.NewGateway(gatewayCfg, altCoord.AgentCoordinator, logger)
	if err != nil {
		_ = altCoord.Shutdown()
		return nil, err
	}
	container.LarkGateway = gateway

	// Lark calendar operations require user-scoped OAuth tokens to access a user's
	// primary calendar. Provide an OAuth service to tools via the gateway context.
	if oauthSvc := buildLarkOAuthService(ctx, cfg, container, logger); oauthSvc != nil {
		container.LarkOAuth = oauthSvc
		gateway.SetOAuthService(oauthSvc)
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

	taskStore, err := buildLarkTaskStore(ctx, gatewayCfg)
	if err != nil {
		logger.Warn("Lark task store init failed: %v", err)
	} else {
		gateway.SetTaskStore(taskStore)
		if err := taskStore.MarkStaleRunning(ctx, "gateway restart"); err != nil {
			logger.Warn("Lark task store stale cleanup failed: %v", err)
		}
		logger.Info("Lark task store enabled (mode=%s)", strings.ToLower(strings.TrimSpace(gatewayCfg.PersistenceMode)))
	}

	async.Go(logger, "lark.gateway", func() {
		if err := gateway.Start(ctx); err != nil {
			logger.Warn("Lark gateway stopped: %v", err)
		}
	})

	cleanup := func() {
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
	mode := strings.ToLower(strings.TrimSpace(cfg.PersistenceMode))
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
	mode := strings.ToLower(strings.TrimSpace(cfg.PersistenceMode))
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

func buildLarkTaskStore(ctx context.Context, cfg lark.Config) (lark.TaskStore, error) {
	mode := strings.ToLower(strings.TrimSpace(cfg.PersistenceMode))
	switch mode {
	case larkPersistenceModeMemory:
		store := lark.NewTaskMemoryStore(cfg.PersistenceRetention, cfg.PersistenceMaxTasksPerChat)
		if err := store.EnsureSchema(ctx); err != nil {
			return nil, err
		}
		return store, nil
	case larkPersistenceModeFile:
		store, err := lark.NewTaskFileStore(cfg.PersistenceDir, cfg.PersistenceRetention, cfg.PersistenceMaxTasksPerChat)
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
	larkCfg := cfg.Channels.Lark
	if !larkCfg.Enabled {
		return nil
	}
	if strings.TrimSpace(larkCfg.AppID) == "" || strings.TrimSpace(larkCfg.AppSecret) == "" {
		return nil
	}

	redirectBase := strings.TrimSpace(cfg.Auth.RedirectBaseURL)
	if redirectBase == "" {
		port := strings.TrimPrefix(cfg.Port, ":")
		if port == "" {
			port = "8080"
		}
		redirectBase = fmt.Sprintf("http://localhost:%s", port)
	}
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
