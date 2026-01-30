package bootstrap

import (
	"context"
	"fmt"
	"strings"
	"time"

	"alex/internal/agent/presets"
	"alex/internal/async"
	"alex/internal/channels"
	"alex/internal/channels/lark"
	"alex/internal/di"
	"alex/internal/logging"
	serverApp "alex/internal/server/app"
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
	agentContainer := container
	var extraContainer *di.Container
	switch toolMode {
	case string(presets.ToolModeCLI):
		var err error
		extraContainer, err = buildContainerWithToolMode(cfg, presets.ToolModeCLI)
		if err != nil {
			return nil, fmt.Errorf("build lark gateway container: %w", err)
		}
		if err := extraContainer.Start(); err != nil {
			logger.Warn("Lark container start failed: %v (continuing)", err)
		}
		if summary := cfg.EnvironmentSummary; summary != "" {
			extraContainer.AgentCoordinator.SetEnvironmentSummary(summary)
		}
		agentContainer = extraContainer
	case string(presets.ToolModeWeb):
		agentContainer = container
	default:
		return nil, fmt.Errorf("lark tool_mode must be cli or web, got %q", larkCfg.ToolMode)
	}

	gatewayCfg := lark.Config{
		BaseConfig: channels.BaseConfig{
			SessionPrefix: larkCfg.SessionPrefix,
			ReplyPrefix:   larkCfg.ReplyPrefix,
			AllowGroups:   larkCfg.AllowGroups,
			AllowDirect:   larkCfg.AllowDirect,
			AgentPreset:   larkCfg.AgentPreset,
			ToolPreset:    larkCfg.ToolPreset,
			ReplyTimeout:  larkCfg.ReplyTimeout,
			MemoryEnabled: larkCfg.MemoryEnabled,
		},
		Enabled:                       larkCfg.Enabled,
		AppID:                         larkCfg.AppID,
		AppSecret:                     larkCfg.AppSecret,
		BaseDomain:                    larkCfg.BaseDomain,
		SessionMode:                   larkCfg.SessionMode,
		ReactEmoji:                    larkCfg.ReactEmoji,
		ShowToolProgress:              larkCfg.ShowToolProgress,
		AutoChatContext:               larkCfg.AutoChatContext,
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
		if extraContainer != nil {
			if err := extraContainer.Shutdown(); err != nil {
				logger.Warn("Lark container shutdown failed: %v", err)
			}
		}
	}

	return cleanup, nil
}
