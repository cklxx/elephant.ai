package bootstrap

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	"alex/internal/agent/presets"
	"alex/internal/async"
	"alex/internal/channels/lark"
	"alex/internal/di"
	larkoauth "alex/internal/lark/oauth"
	"alex/internal/logging"
	serverApp "alex/internal/server/app"
	"alex/internal/toolregistry"
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
	extraContainer, err := buildContainerWithToolModeAndToolset(
		cfg,
		presets.ToolMode(toolMode),
		toolregistry.ToolsetLarkLocal,
		browserCfg,
	)
	if err != nil {
		return nil, fmt.Errorf("build lark gateway container: %w", err)
	}
	if err := extraContainer.Start(); err != nil {
		logger.Warn("Lark container start failed: %v (continuing)", err)
	}
	if summary := cfg.EnvironmentSummary; summary != "" {
		extraContainer.AgentCoordinator.SetEnvironmentSummary(summary)
	}
	agentContainer := extraContainer

	gatewayCfg := lark.Config{
		BaseConfig:                    larkCfg.BaseConfig,
		Enabled:                       larkCfg.Enabled,
		AppID:                         larkCfg.AppID,
		AppSecret:                     larkCfg.AppSecret,
		TenantCalendarID:              larkCfg.TenantCalendarID,
		BaseDomain:                    larkCfg.BaseDomain,
		WorkspaceDir:                  larkCfg.WorkspaceDir,
		CardsEnabled:                  larkCfg.CardsEnabled,
		CardsPlanReview:               larkCfg.CardsPlanReview,
		CardsResults:                  larkCfg.CardsResults,
		CardsErrors:                   larkCfg.CardsErrors,
		CardCallbackVerificationToken: larkCfg.CardCallbackVerificationToken,
		CardCallbackEncryptKey:        larkCfg.CardCallbackEncryptKey,
		AutoUploadFiles:               larkCfg.AutoUploadFiles,
		AutoUploadMaxBytes:            larkCfg.AutoUploadMaxBytes,
		AutoUploadAllowExt:            append([]string(nil), larkCfg.AutoUploadAllowExt...),
		Browser: lark.BrowserConfig{
			CDPURL:      larkCfg.Browser.CDPURL,
			ChromePath:  larkCfg.Browser.ChromePath,
			Headless:    larkCfg.Browser.Headless,
			UserDataDir: larkCfg.Browser.UserDataDir,
			Timeout:     larkCfg.Browser.Timeout,
		},
		ReactEmoji:                    larkCfg.ReactEmoji,
		InjectionAckReactEmoji:        larkCfg.InjectionAckReactEmoji,
		ShowToolProgress:              larkCfg.ShowToolProgress,
		AutoChatContextSize:           larkCfg.AutoChatContextSize,
		PlanReviewEnabled:             larkCfg.PlanReviewEnabled,
		PlanReviewRequireConfirmation: larkCfg.PlanReviewRequireConfirmation,
		PlanReviewPendingTTL:          larkCfg.PlanReviewPendingTTL,
	}
	if gatewayCfg.PlanReviewEnabled {
		if gatewayCfg.PlanReviewPendingTTL <= 0 {
			gatewayCfg.PlanReviewPendingTTL = 60 * time.Minute
		}
		if !gatewayCfg.PlanReviewRequireConfirmation {
			gatewayCfg.PlanReviewRequireConfirmation = true
		}
	}

	var planReviewStore lark.PlanReviewStore
	if gatewayCfg.PlanReviewEnabled {
		if container.SessionDB == nil {
			logger.Warn("Lark plan review disabled: session DB not configured")
			gatewayCfg.PlanReviewEnabled = false
		} else {
			store := lark.NewPlanReviewPostgresStore(container.SessionDB, gatewayCfg.PlanReviewPendingTTL)
			if err := store.EnsureSchema(ctx); err != nil {
				logger.Warn("Lark plan review store init failed: %v", err)
				gatewayCfg.PlanReviewEnabled = false
			} else {
				planReviewStore = store
			}
		}
	}

	gateway, err := lark.NewGateway(gatewayCfg, agentContainer.AgentCoordinator, logger)
	if err != nil {
		if extraContainer != nil {
			_ = extraContainer.Shutdown()
		}
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
		if extraContainer != nil {
			if err := extraContainer.Shutdown(); err != nil {
				logger.Warn("Lark container shutdown failed: %v", err)
			}
		}
	}

	return cleanup, nil
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

	var tokenStore larkoauth.TokenStore
	if container.SessionDB != nil {
		store := larkoauth.NewPostgresTokenStore(container.SessionDB)
		if err := store.EnsureSchema(ctx); err != nil {
			logger.Warn("Lark OAuth token store init failed (Postgres): %v", err)
		} else {
			tokenStore = store
			logger.Info("Lark OAuth token store backed by Postgres")
		}
	}
	if tokenStore == nil {
		dir := filepath.Join(container.SessionDir(), "_lark_oauth")
		store, err := larkoauth.NewFileTokenStore(dir)
		if err != nil {
			logger.Warn("Lark OAuth token store init failed (file): %v", err)
			return nil
		}
		tokenStore = store
		logger.Info("Lark OAuth token store backed by file: %s", dir)
	}

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
