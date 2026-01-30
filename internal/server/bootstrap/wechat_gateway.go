package bootstrap

import (
	"context"
	"fmt"
	"strings"

	"alex/internal/agent/presets"
	"alex/internal/async"
	"alex/internal/channels"
	"alex/internal/channels/wechat"
	"alex/internal/di"
	"alex/internal/logging"
)

func startWeChatGateway(ctx context.Context, cfg Config, container *di.Container, logger logging.Logger) (func(), error) {
	logger = logging.OrNop(logger)
	wechatCfg := cfg.Channels.WeChat
	if !wechatCfg.Enabled {
		return nil, nil
	}
	if container == nil {
		return nil, fmt.Errorf("wechat gateway requires server container")
	}
	if ctx == nil {
		ctx = context.Background()
	}

	toolMode := strings.ToLower(strings.TrimSpace(wechatCfg.ToolMode))
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
			return nil, fmt.Errorf("build wechat gateway container: %w", err)
		}
		if err := extraContainer.Start(); err != nil {
			logger.Warn("WeChat container start failed: %v (continuing)", err)
		}
		if summary := cfg.EnvironmentSummary; summary != "" {
			extraContainer.AgentCoordinator.SetEnvironmentSummary(summary)
		}
		agentContainer = extraContainer
	case string(presets.ToolModeWeb):
		agentContainer = container
	default:
		return nil, fmt.Errorf("wechat tool_mode must be cli or web, got %q", wechatCfg.ToolMode)
	}

	gatewayCfg := wechat.Config{
		BaseConfig: channels.BaseConfig{
			SessionPrefix: wechatCfg.SessionPrefix,
			ReplyPrefix:   wechatCfg.ReplyPrefix,
			AllowGroups:   wechatCfg.AllowGroups,
			AllowDirect:   wechatCfg.AllowDirect,
			AgentPreset:   wechatCfg.AgentPreset,
			ToolPreset:    wechatCfg.ToolPreset,
			ReplyTimeout:  wechatCfg.ReplyTimeout,
			MemoryEnabled: wechatCfg.MemoryEnabled,
		},
		Enabled:                wechatCfg.Enabled,
		LoginMode:              wechatCfg.LoginMode,
		HotLogin:               wechatCfg.HotLogin,
		HotLoginStoragePath:    wechatCfg.HotLoginStoragePath,
		MentionOnly:            wechatCfg.MentionOnly,
		ReplyWithMention:       wechatCfg.ReplyWithMention,
		AllowedConversationIDs: wechatCfg.AllowedConversationIDs,
	}

	gateway, err := wechat.NewGateway(gatewayCfg, agentContainer.AgentCoordinator, logger)
	if err != nil {
		if extraContainer != nil {
			_ = extraContainer.Shutdown()
		}
		return nil, err
	}

	async.Go(logger, "wechat.gateway", func() {
		if err := gateway.Start(ctx); err != nil {
			logger.Warn("WeChat gateway stopped: %v", err)
		}
	})

	cleanup := func() {
		gateway.Stop()
		if extraContainer != nil {
			if err := extraContainer.Shutdown(); err != nil {
				logger.Warn("WeChat container shutdown failed: %v", err)
			}
		}
	}

	return cleanup, nil
}
