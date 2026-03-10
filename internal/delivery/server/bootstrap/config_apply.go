package bootstrap

import (
	"strings"
	"time"

	runtimeconfig "alex/internal/shared/config"
	"alex/internal/shared/utils"
)

func applyServerHTTPConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Server == nil {
		return
	}
	if port := strings.TrimSpace(file.Server.Port); port != "" {
		cfg.Port = port
	}
	if debugPort := strings.TrimSpace(file.Server.DebugPort); debugPort != "" {
		cfg.DebugPort = debugPort
	}
	if debugBindHost := strings.TrimSpace(file.Server.DebugBindHost); debugBindHost != "" {
		cfg.DebugBindHost = debugBindHost
	}
	if file.Server.MaxTaskBodyBytes != nil && *file.Server.MaxTaskBodyBytes > 0 {
		cfg.MaxTaskBodyBytes = *file.Server.MaxTaskBodyBytes
	}
	if file.Server.StreamMaxDurationSeconds != nil && *file.Server.StreamMaxDurationSeconds > 0 {
		cfg.StreamGuard.MaxDuration = time.Duration(*file.Server.StreamMaxDurationSeconds) * time.Second
	}
	if file.Server.StreamMaxBytes != nil && *file.Server.StreamMaxBytes > 0 {
		cfg.StreamGuard.MaxBytes = *file.Server.StreamMaxBytes
	}
	if file.Server.StreamMaxConcurrent != nil && *file.Server.StreamMaxConcurrent > 0 {
		cfg.StreamGuard.MaxConcurrent = *file.Server.StreamMaxConcurrent
	}
	if file.Server.RateLimitRequestsPerMinute != nil && *file.Server.RateLimitRequestsPerMinute > 0 {
		cfg.RateLimit.RequestsPerMinute = *file.Server.RateLimitRequestsPerMinute
	}
	if file.Server.RateLimitBurst != nil && *file.Server.RateLimitBurst > 0 {
		cfg.RateLimit.Burst = *file.Server.RateLimitBurst
	}
	if file.Server.NonStreamTimeoutSeconds != nil && *file.Server.NonStreamTimeoutSeconds > 0 {
		cfg.NonStreamTimeout = time.Duration(*file.Server.NonStreamTimeoutSeconds) * time.Second
	}
	if token := strings.TrimSpace(file.Server.LeaderAPIToken); token != "" {
		cfg.LeaderAPIToken = token
	}
	if ownerID := strings.TrimSpace(file.Server.TaskExecutionOwnerID); ownerID != "" {
		cfg.TaskExecution.OwnerID = ownerID
	}
	if file.Server.TaskExecutionLeaseTTLSeconds != nil && *file.Server.TaskExecutionLeaseTTLSeconds > 0 {
		cfg.TaskExecution.LeaseTTL = time.Duration(*file.Server.TaskExecutionLeaseTTLSeconds) * time.Second
	}
	if file.Server.TaskExecutionLeaseRenewIntervalSeconds != nil && *file.Server.TaskExecutionLeaseRenewIntervalSeconds > 0 {
		cfg.TaskExecution.LeaseRenewInterval = time.Duration(*file.Server.TaskExecutionLeaseRenewIntervalSeconds) * time.Second
	}
	if file.Server.TaskExecutionMaxInFlight != nil {
		if *file.Server.TaskExecutionMaxInFlight <= 0 {
			cfg.TaskExecution.MaxInFlight = 0
		} else {
			cfg.TaskExecution.MaxInFlight = *file.Server.TaskExecutionMaxInFlight
		}
	}
	if file.Server.TaskExecutionResumeClaimBatchSize != nil && *file.Server.TaskExecutionResumeClaimBatchSize > 0 {
		cfg.TaskExecution.ResumeClaimBatchSize = *file.Server.TaskExecutionResumeClaimBatchSize
	}
	if file.Server.EventHistoryRetentionDays != nil {
		days := *file.Server.EventHistoryRetentionDays
		if days <= 0 {
			cfg.EventHistory.Retention = 0
		} else {
			cfg.EventHistory.Retention = time.Duration(days) * 24 * time.Hour
		}
	}
	if file.Server.EventHistoryMaxSessions != nil {
		if *file.Server.EventHistoryMaxSessions <= 0 {
			cfg.EventHistory.MaxSessions = 0
		} else {
			cfg.EventHistory.MaxSessions = *file.Server.EventHistoryMaxSessions
		}
	}
	if file.Server.EventHistorySessionTTL != nil {
		if *file.Server.EventHistorySessionTTL <= 0 {
			cfg.EventHistory.SessionTTL = 0
		} else {
			cfg.EventHistory.SessionTTL = time.Duration(*file.Server.EventHistorySessionTTL) * time.Second
		}
	}
	if file.Server.EventHistoryMaxEvents != nil {
		if *file.Server.EventHistoryMaxEvents <= 0 {
			cfg.EventHistory.MaxEvents = 0
		} else {
			cfg.EventHistory.MaxEvents = *file.Server.EventHistoryMaxEvents
		}
	}
	if file.Server.AllowedOrigins != nil {
		cfg.AllowedOrigins = normalizeAllowedOrigins(file.Server.AllowedOrigins)
	}
	if len(file.Server.TrustedProxies) > 0 {
		cfg.RateLimit.TrustedProxies = append([]string(nil), file.Server.TrustedProxies...)
	}
}

func applySessionConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Session == nil {
		return
	}
	if dir := strings.TrimSpace(file.Session.Dir); dir != "" {
		cfg.Session.Dir = dir
	}
}

func applyAnalyticsConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Analytics == nil {
		return
	}
	cfg.Analytics = runtimeconfig.AnalyticsConfig{
		PostHogAPIKey: strings.TrimSpace(file.Analytics.PostHogAPIKey),
		PostHogHost:   strings.TrimSpace(file.Analytics.PostHogHost),
	}
}

func applyAttachmentConfig(cfg *Config, file runtimeconfig.FileConfig) {
	if file.Attachments == nil {
		return
	}
	if provider := strings.TrimSpace(file.Attachments.Provider); provider != "" {
		cfg.Attachment.Provider = provider
	}
	if dir := strings.TrimSpace(file.Attachments.Dir); dir != "" {
		cfg.Attachment.Dir = dir
	}
	if accountID := strings.TrimSpace(file.Attachments.CloudflareAccountID); accountID != "" {
		cfg.Attachment.CloudflareAccountID = accountID
	}
	if accessKey := strings.TrimSpace(file.Attachments.CloudflareAccessKeyID); accessKey != "" {
		cfg.Attachment.CloudflareAccessKeyID = accessKey
	}
	if secret := strings.TrimSpace(file.Attachments.CloudflareSecretAccessKey); secret != "" {
		cfg.Attachment.CloudflareSecretAccessKey = secret
	}
	if bucket := strings.TrimSpace(file.Attachments.CloudflareBucket); bucket != "" {
		cfg.Attachment.CloudflareBucket = bucket
	}
	if base := strings.TrimSpace(file.Attachments.CloudflarePublicBaseURL); base != "" {
		cfg.Attachment.CloudflarePublicBaseURL = base
	}
	if prefix := strings.TrimSpace(file.Attachments.CloudflareKeyPrefix); prefix != "" {
		cfg.Attachment.CloudflareKeyPrefix = prefix
	}
	if ttlRaw := strings.TrimSpace(file.Attachments.PresignTTL); ttlRaw != "" {
		if parsed, err := time.ParseDuration(ttlRaw); err == nil && parsed > 0 {
			cfg.Attachment.PresignTTL = parsed
		}
	}
}

func normalizeAllowedOrigins(values []string) []string {
	return utils.TrimDedupeStrings(values)
}

func lookupFirstNonEmptyEnv(lookup runtimeconfig.EnvLookup, keys ...string) string {
	if lookup == nil {
		return ""
	}
	for _, key := range keys {
		if value, ok := lookup(key); ok {
			trimmed := strings.TrimSpace(value)
			if trimmed != "" {
				return trimmed
			}
		}
	}
	return ""
}

func applyTrimmedString(dst *string, value string) {
	trimmed := strings.TrimSpace(value)
	if trimmed != "" {
		*dst = trimmed
	}
}

func applyOptionalTrimmedString(dst *string, value *string) {
	if value == nil {
		return
	}
	*dst = strings.TrimSpace(*value)
}

func applyTrimmedLowerString(dst *string, value string) {
	trimmed := utils.TrimLower(value)
	if trimmed != "" {
		*dst = trimmed
	}
}

func applyOptionalBool(dst *bool, value *bool) {
	if value != nil {
		*dst = *value
	}
}

func applyPositiveInt(dst *int, value *int) {
	if value != nil && *value > 0 {
		*dst = *value
	}
}

func applyPositiveDurationSeconds(dst *time.Duration, seconds *int) {
	if seconds != nil && *seconds > 0 {
		*dst = time.Duration(*seconds) * time.Second
	}
}

func applyPositiveDurationMinutes(dst *time.Duration, minutes *int) {
	if minutes != nil && *minutes > 0 {
		*dst = time.Duration(*minutes) * time.Minute
	}
}

func applyPositiveDurationHours(dst *time.Duration, hours *int) {
	if hours != nil && *hours > 0 {
		*dst = time.Duration(*hours) * time.Hour
	}
}

func applyPositiveDurationMilliseconds(dst *time.Duration, ms *int) {
	if ms != nil && *ms > 0 {
		*dst = time.Duration(*ms) * time.Millisecond
	}
}
