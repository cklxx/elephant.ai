package bootstrap

import (
	"fmt"
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"

	"alex/internal/delivery/channels/lark"
)

func applyLarkEnvFallback(cfg *Config, lookup runtimeconfig.EnvLookup) {
	if debugPort := lookupFirstNonEmptyEnv(lookup, "ALEX_DEBUG_PORT"); debugPort != "" {
		cfg.DebugPort = debugPort
	}
	if logDir := lookupFirstNonEmptyEnv(lookup, "ALEX_LOG_DIR"); logDir != "" {
		cfg.LogDir = logDir
	}
}

func applyLarkConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Channels == nil || file.Channels.Lark == nil {
		return
	}
	larkCfg := file.Channels.Lark
	target := cfg.Channels.LarkConfig()
	applyOptionalBool(&target.Enabled, larkCfg.Enabled)
	applyTrimmedString(&target.AppID, larkCfg.AppID)
	applyTrimmedString(&target.AppSecret, larkCfg.AppSecret)
	applyTrimmedString(&target.TenantCalendarID, larkCfg.TenantCalendarID)
	applyTrimmedString(&target.BaseDomain, larkCfg.BaseDomain)
	applyTrimmedString(&target.WorkspaceDir, larkCfg.WorkspaceDir)
	applyOptionalBool(&target.AutoUploadFiles, larkCfg.AutoUploadFiles)
	applyPositiveInt(&target.AutoUploadMaxBytes, larkCfg.AutoUploadMaxBytes)
	if len(larkCfg.AutoUploadAllowExt) > 0 {
		target.AutoUploadAllowExt = append([]string(nil), larkCfg.AutoUploadAllowExt...)
	}
	applyBrowserConfig(&target.Browser, larkCfg.Browser)
	applyTrimmedString(&target.SessionPrefix, larkCfg.SessionPrefix)
	applyTrimmedString(&target.ReplyPrefix, larkCfg.ReplyPrefix)
	applyOptionalBool(&target.AllowGroups, larkCfg.AllowGroups)
	applyOptionalBool(&target.AllowDirect, larkCfg.AllowDirect)
	applyTrimmedString(&target.AgentPreset, larkCfg.AgentPreset)
	applyTrimmedString(&target.ToolPreset, larkCfg.ToolPreset)
	applyTrimmedString(&target.ToolMode, larkCfg.ToolMode)
	applyPositiveDuration(&target.ReplyTimeout, larkCfg.ReplyTimeoutSeconds, time.Second)
	applyTrimmedString(&target.InjectionAckReactEmoji, larkCfg.InjectionAckReactEmoji)
	applyOptionalBool(&target.MemoryEnabled, larkCfg.MemoryEnabled)
	applyOptionalBool(&target.ShowToolProgress, larkCfg.ShowToolProgress)
	applyOptionalBool(&target.SlowProgressSummaryEnabled, larkCfg.SlowProgressSummaryEnabled)
	applyPositiveDuration(&target.SlowProgressSummaryDelay, larkCfg.SlowProgressSummaryDelaySecs, time.Second)
	applyOptionalBool(&target.ShowPlanClarifyMessages, larkCfg.ShowPlanClarifyMessages)
	applyPositiveInt(&target.ToolFailureAbortThreshold, larkCfg.ToolFailureAbortThreshold)
	applyPositiveInt(&target.AutoChatContextSize, larkCfg.AutoChatContextSize)
	applyOptionalBool(&target.PlanReviewEnabled, larkCfg.PlanReviewEnabled)
	applyOptionalBool(&target.PlanReviewRequireConfirmation, larkCfg.PlanReviewRequireConfirmation)
	applyPositiveDuration(&target.PlanReviewPendingTTL, larkCfg.PlanReviewPendingTTLMinutes, time.Minute)
	applyPositiveDuration(&target.ActiveSlotTTL, larkCfg.ActiveSlotTTLMinutes, time.Minute)
	applyPositiveInt(&target.ActiveSlotMaxEntries, larkCfg.ActiveSlotMaxEntries)
	applyPositiveDuration(&target.PendingInputRelayTTL, larkCfg.PendingInputRelayTTLMinutes, time.Minute)
	applyPositiveInt(&target.PendingInputRelayMaxChats, larkCfg.PendingInputRelayMaxChats)
	applyPositiveInt(&target.PendingInputRelayMaxPerChat, larkCfg.PendingInputRelayMaxPerChat)
	applyPositiveDuration(&target.AIChatSessionTTL, larkCfg.AIChatSessionTTLMinutes, time.Minute)
	applyPositiveDuration(&target.StateCleanupInterval, larkCfg.StateCleanupIntervalSeconds, time.Second)
	applyLarkPersistenceConfig(&target, larkCfg.Persistence)
	applyLarkDeliveryConfig(&target, larkCfg.Delivery)
	applyLarkRateLimiterConfig(&target, larkCfg.RateLimiter)
	applyPositiveInt(&target.MaxConcurrentTasks, larkCfg.MaxConcurrentTasks)
	applyOptionalTrimmedString(&target.DefaultPlanMode, larkCfg.DefaultPlanMode)
	cfg.Channels.SetLarkConfig(target)
}

func applyBrowserConfig(dst *lark.BrowserConfig, browser *runtimeconfig.LarkBrowserConfig) {
	if dst == nil || browser == nil {
		return
	}
	applyTrimmedString(&dst.CDPURL, browser.CDPURL)
	applyTrimmedString(&dst.ChromePath, browser.ChromePath)
	applyOptionalBool(&dst.Headless, browser.Headless)
	applyTrimmedString(&dst.UserDataDir, browser.UserDataDir)
	applyPositiveDuration(&dst.Timeout, browser.TimeoutSeconds, time.Second)
}

func applyLarkPersistenceConfig(dst *LarkGatewayConfig, persistence *runtimeconfig.LarkPersistenceConfig) {
	if dst == nil || persistence == nil {
		return
	}
	applyTrimmedLowerString(&dst.PersistenceMode, persistence.Mode)
	applyTrimmedString(&dst.PersistenceDir, persistence.Dir)
	applyPositiveDuration(&dst.PersistenceRetention, persistence.RetentionHours, time.Hour)
	applyPositiveInt(&dst.PersistenceMaxTasksPerChat, persistence.MaxTasksPerChat)
}

func applyLarkDeliveryConfig(dst *LarkGatewayConfig, delivery *runtimeconfig.LarkDeliveryConfig) {
	if dst == nil || delivery == nil {
		return
	}
	applyTrimmedLowerString(&dst.DeliveryMode, delivery.Mode)
	if delivery.Worker == nil {
		return
	}
	worker := delivery.Worker
	applyOptionalBool(&dst.DeliveryWorker.Enabled, worker.Enabled)
	applyPositiveDuration(&dst.DeliveryWorker.PollInterval, worker.PollIntervalMs, time.Millisecond)
	applyPositiveInt(&dst.DeliveryWorker.BatchSize, worker.BatchSize)
	applyPositiveInt(&dst.DeliveryWorker.MaxAttempts, worker.MaxAttempts)
	applyPositiveDuration(&dst.DeliveryWorker.BaseBackoff, worker.BaseBackoffMs, time.Millisecond)
	applyPositiveDuration(&dst.DeliveryWorker.MaxBackoff, worker.MaxBackoffMs, time.Millisecond)
	if worker.JitterRatio != nil && *worker.JitterRatio > 0 {
		dst.DeliveryWorker.JitterRatio = *worker.JitterRatio
	}
}

func applyLarkRateLimiterConfig(dst *LarkGatewayConfig, rl *runtimeconfig.LarkRateLimiterConfig) {
	if dst == nil || rl == nil {
		return
	}
	applyOptionalBool(&dst.RateLimiterEnabled, rl.Enabled)
	applyPositiveInt(&dst.RateLimiterChatHourlyLimit, rl.ChatHourlyLimit)
	applyPositiveInt(&dst.RateLimiterUserDailyLimit, rl.UserDailyLimit)
}

func validateLarkPersistenceConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	larkCfg := cfg.Channels.LarkConfig()
	if err := validatePersistenceMode(&larkCfg.PersistenceMode, &larkCfg.PersistenceDir, "channels.lark"); err != nil {
		return err
	}
	if larkCfg.PersistenceRetention <= 0 {
		larkCfg.PersistenceRetention = 7 * 24 * time.Hour
	}
	if larkCfg.PersistenceMaxTasksPerChat <= 0 {
		larkCfg.PersistenceMaxTasksPerChat = 200
	}
	cfg.Channels.SetLarkConfig(larkCfg)
	return nil
}

func validateLarkDeliveryConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	larkCfg := cfg.Channels.LarkConfig()
	mode := utils.TrimLower(larkCfg.DeliveryMode)
	if mode == "" {
		mode = "shadow"
	}
	switch mode {
	case "direct", "shadow", "outbox":
	default:
		return fmt.Errorf("channels.lark.delivery.mode must be one of [direct,shadow,outbox], got %q", mode)
	}
	larkCfg.DeliveryMode = mode

	if err := applyDeliveryWorkerDefaults(&larkCfg.DeliveryWorker); err != nil {
		return err
	}
	cfg.Channels.SetLarkConfig(larkCfg)
	return nil
}

func applyDeliveryWorkerDefaults(worker *lark.DeliveryWorkerConfig) error {
	if worker.PollInterval <= 0 {
		worker.PollInterval = 500 * time.Millisecond
	}
	if worker.BatchSize <= 0 {
		worker.BatchSize = 50
	}
	if worker.MaxAttempts <= 0 {
		worker.MaxAttempts = 8
	}
	if worker.BaseBackoff <= 0 {
		worker.BaseBackoff = 500 * time.Millisecond
	}
	if worker.MaxBackoff <= 0 {
		worker.MaxBackoff = 60 * time.Second
	}
	if worker.MaxBackoff < worker.BaseBackoff {
		worker.MaxBackoff = worker.BaseBackoff
	}
	if worker.JitterRatio <= 0 {
		worker.JitterRatio = 0.2
	}
	if worker.JitterRatio > 1 {
		return fmt.Errorf("channels.lark.delivery.worker.jitter_ratio must be <= 1, got %v", worker.JitterRatio)
	}
	return nil
}

func applyTelegramConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Channels == nil || file.Channels.Telegram == nil {
		return
	}
	tgCfg := file.Channels.Telegram
	target := cfg.Channels.TelegramConfig()
	applyOptionalBool(&target.Enabled, tgCfg.Enabled)
	applyTrimmedString(&target.BotToken, tgCfg.BotToken)
	applyTrimmedString(&target.SessionPrefix, tgCfg.SessionPrefix)
	applyTrimmedString(&target.ReplyPrefix, tgCfg.ReplyPrefix)
	applyOptionalBool(&target.AllowGroups, tgCfg.AllowGroups)
	applyOptionalBool(&target.AllowDirect, tgCfg.AllowDirect)
	applyTrimmedString(&target.AgentPreset, tgCfg.AgentPreset)
	applyTrimmedString(&target.ToolPreset, tgCfg.ToolPreset)
	applyPositiveDuration(&target.ReplyTimeout, tgCfg.ReplyTimeoutSeconds, time.Second)
	applyOptionalBool(&target.MemoryEnabled, tgCfg.MemoryEnabled)
	if len(tgCfg.AllowedGroups) > 0 {
		target.AllowedGroups = append([]int64(nil), tgCfg.AllowedGroups...)
	}
	applyOptionalBool(&target.ShowToolProgress, tgCfg.ShowToolProgress)
	applyOptionalBool(&target.SlowProgressSummaryEnabled, tgCfg.SlowProgressSummaryEnabled)
	applyPositiveDuration(&target.SlowProgressSummaryDelay, tgCfg.SlowProgressSummaryDelaySecs, time.Second)
	applyOptionalBool(&target.PlanReviewEnabled, tgCfg.PlanReviewEnabled)
	applyOptionalBool(&target.PlanReviewRequireConfirmation, tgCfg.PlanReviewRequireConfirmation)
	applyPositiveDuration(&target.PlanReviewPendingTTL, tgCfg.PlanReviewPendingTTLMinutes, time.Minute)
	applyPositiveDuration(&target.ActiveSlotTTL, tgCfg.ActiveSlotTTLMinutes, time.Minute)
	applyPositiveInt(&target.ActiveSlotMaxEntries, tgCfg.ActiveSlotMaxEntries)
	applyPositiveDuration(&target.StateCleanupInterval, tgCfg.StateCleanupIntervalSeconds, time.Second)
	applyTelegramPersistenceConfig(&target, tgCfg.Persistence)
	applyPositiveInt(&target.MaxConcurrentTasks, tgCfg.MaxConcurrentTasks)
	cfg.Channels.SetTelegramConfig(target)
}

func applyTelegramPersistenceConfig(dst *TelegramGatewayConfig, persistence *runtimeconfig.TelegramPersistenceConfig) {
	if dst == nil || persistence == nil {
		return
	}
	applyTrimmedLowerString(&dst.PersistenceMode, persistence.Mode)
	applyTrimmedString(&dst.PersistenceDir, persistence.Dir)
	applyPositiveDuration(&dst.PersistenceRetention, persistence.RetentionHours, time.Hour)
	applyPositiveInt(&dst.PersistenceMaxTasksPerChat, persistence.MaxTasksPerChat)
}

func applyTelegramEnvFallback(cfg *Config, lookup runtimeconfig.EnvLookup) {
	if token := lookupFirstNonEmptyEnv(lookup, "TELEGRAM_BOT_TOKEN"); token != "" {
		tgCfg := cfg.Channels.TelegramConfig()
		if tgCfg.BotToken == "" {
			tgCfg.BotToken = token
			cfg.Channels.SetTelegramConfig(tgCfg)
		}
	}
}

func validateTelegramPersistenceConfig(cfg *Config) error {
	if cfg == nil {
		return nil
	}
	tgCfg := cfg.Channels.TelegramConfig()
	if !tgCfg.Enabled {
		return nil
	}
	if err := validatePersistenceMode(&tgCfg.PersistenceMode, &tgCfg.PersistenceDir, "channels.telegram"); err != nil {
		return err
	}
	if tgCfg.PersistenceRetention <= 0 {
		tgCfg.PersistenceRetention = 7 * 24 * time.Hour
	}
	if tgCfg.PersistenceMaxTasksPerChat <= 0 {
		tgCfg.PersistenceMaxTasksPerChat = 200
	}
	cfg.Channels.SetTelegramConfig(tgCfg)
	return nil
}

// validatePersistenceMode validates and normalizes the persistence mode and
// directory for a channel configuration. prefix is used for error messages
// (e.g. "channels.lark").
func validatePersistenceMode(mode *string, dir *string, prefix string) error {
	m := utils.TrimLower(*mode)
	if m == "" {
		m = "file"
	}
	switch m {
	case "file", "memory":
	default:
		return fmt.Errorf("%s.persistence.mode must be one of [file,memory], got %q", prefix, m)
	}
	*mode = m
	if m == "file" {
		d := strings.TrimSpace(*dir)
		if d == "" {
			return fmt.Errorf("%s.persistence.dir is required when persistence.mode=file", prefix)
		}
		*dir = expandHome(d)
	}
	return nil
}
